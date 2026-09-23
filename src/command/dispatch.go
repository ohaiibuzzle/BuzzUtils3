package command

import (
	"fmt"
	"log"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// HandleMessage dispatches a prefix command. It returns false if the message
// isn't a known command.
func HandleMessage(e *events.MessageCreate, prefix string) bool {
	m := e.Message
	if prefix == "" || !strings.HasPrefix(m.Content, prefix) {
		return false
	}
	name, rest := nextToken(m.Content[len(prefix):])
	cmd := Lookup(name)
	if cmd == nil {
		return false
	}

	c := &Ctx{
		Client:    e.Client(),
		Message:   &m,
		Command:   cmd,
		ChannelID: m.ChannelID,
		Author:    m.Author,
		args:      map[string]any{},
	}
	if e.GuildID != nil {
		c.GuildID = *e.GuildID
	}
	log.Default().Println("User " + m.Author.Username + " issued command: " + cmd.Name)

	go func() {
		defer recoverPanic(c)
		target := cmd
		if len(cmd.Subcommands) > 0 {
			subName, subRest := nextToken(rest)
			sub := cmd.subcommand(subName)
			if sub == nil {
				c.Reply(usage(prefix, cmd))
				return
			}
			target, rest = sub, subRest
		}
		if err := parsePrefixArgs(c, target.Options, rest); err != nil {
			c.Reply(err.Error() + "\n" + usage(prefix, cmd))
			return
		}
		run(c, cmd, target)
	}()
	return true
}

// HandleComponent dispatches button clicks.
func HandleComponent(e *events.ComponentInteractionCreate) {
	handleComponent(e)
}

// HandleCommand dispatches slash commands and message context-menu commands.
func HandleCommand(e *events.ApplicationCommandInteractionCreate) {
	c := &Ctx{
		Client:      e.Client(),
		Interaction: &e.ApplicationCommandInteraction,
		Author:      e.User(),
		args:        map[string]any{},
	}
	if ch := e.Channel(); ch.MessageChannel != nil {
		c.ChannelID = ch.ID()
	}
	if guildID := e.GuildID(); guildID != nil {
		c.GuildID = *guildID
	}

	if e.Data.Type() == discord.ApplicationCommandTypeMessage {
		data := e.MessageCommandInteractionData()
		registryMu.RLock()
		act := actions[data.CommandName()]
		registryMu.RUnlock()
		if act == nil {
			return
		}
		target := data.TargetMessage()
		if target.GuildID == nil && c.GuildID != 0 {
			target.GuildID = &c.GuildID
		}
		c.target = &target
		log.Default().Println("User " + c.Author.Username + " used message action: " + act.Name)
		go func() {
			defer recoverPanic(c)
			act.Handler(c, c.target)
		}()
		return
	}
	if e.Data.Type() != discord.ApplicationCommandTypeSlash {
		return
	}

	data := e.SlashCommandInteractionData()
	cmd := Lookup(data.CommandName())
	if cmd == nil {
		return
	}
	c.Command = cmd
	log.Default().Println("User " + c.Author.Username + " issued slash command: " + cmd.Name)

	go func() {
		defer recoverPanic(c)
		target := cmd
		if data.SubCommandName != nil {
			target = cmd.subcommand(*data.SubCommandName)
			if target == nil {
				return
			}
		}
		if err := parseSlashArgs(c, target.Options, data); err != nil {
			c.ReplyPrivate(err.Error())
			return
		}
		run(c, cmd, target)
	}()
}

// parseSlashArgs copies the interaction's option values into the context. Channels
// are looked up in full, since interactions only carry a partial channel.
func parseSlashArgs(c *Ctx, options []Option, data discord.SlashCommandInteractionData) error {
	for _, opt := range options {
		switch opt.Type {
		case discord.ApplicationCommandOptionTypeString:
			if v, ok := data.OptString(opt.Name); ok {
				c.args[opt.Name] = v
			}
		case discord.ApplicationCommandOptionTypeInt:
			if v, ok := data.OptInt(opt.Name); ok {
				c.args[opt.Name] = int64(v)
			}
		case discord.ApplicationCommandOptionTypeFloat:
			if v, ok := data.OptFloat(opt.Name); ok {
				c.args[opt.Name] = v
			}
		case discord.ApplicationCommandOptionTypeBool:
			if v, ok := data.OptBool(opt.Name); ok {
				c.args[opt.Name] = v
			}
		case discord.ApplicationCommandOptionTypeUser:
			if v, ok := data.OptUser(opt.Name); ok {
				c.args[opt.Name] = &v
			}
		case discord.ApplicationCommandOptionTypeChannel:
			if v, ok := data.OptChannel(opt.Name); ok {
				ch, err := fetchGuildChannel(c.Client, v.ID)
				if err != nil {
					return fmt.Errorf("I couldn't look up that channel.")
				}
				c.args[opt.Name] = ch
			}
		case discord.ApplicationCommandOptionTypeRole:
			if v, ok := data.OptRole(opt.Name); ok {
				v.GuildID = c.GuildID
				c.args[opt.Name] = &v
			}
		}
	}
	return nil
}

func run(c *Ctx, cmd, target *Command) {
	if (cmd.GuildOnly || target.GuildOnly) && c.IsDM() {
		c.ReplyPrivate("This command can only be used inside a server.")
		return
	}
	if (cmd.OwnerOnly || target.OwnerOnly) && !IsOwner(c.Client, c.Author.ID) {
		c.ReplyPrivate("Only my owner can do that!")
		return
	}
	if !c.HasPermission(cmd.Permissions | target.Permissions) {
		c.ReplyPrivate("You don't have permission to do that.")
		return
	}
	c.Command = target
	target.Handler(c)
}

func recoverPanic(c *Ctx) {
	if r := recover(); r != nil {
		log.Default().Printf("Panic while handling command: %v\n%s", r, debug.Stack())
		c.ReplyPrivate("There was an error processing your request :(")
	}
}

var (
	ownerOnce sync.Once
	ownerIDs  = map[snowflake.ID]bool{}
)

// IsOwner reports whether the user owns the bot application (or is on its team).
func IsOwner(client *bot.Client, userID snowflake.ID) bool {
	ownerOnce.Do(func() {
		app, err := client.Rest.GetBotApplicationInfo()
		if err != nil {
			log.Default().Println("Error fetching application info: " + err.Error())
			return
		}
		if app.Owner != nil {
			ownerIDs[app.Owner.ID] = true
		}
		if app.Team != nil {
			for _, member := range app.Team.Members {
				ownerIDs[member.User.ID] = true
			}
		}
	})
	return ownerIDs[userID]
}

func nextToken(s string) (string, string) {
	s = strings.TrimLeft(s, " \t\n")
	idx := strings.IndexAny(s, " \t\n")
	if idx < 0 {
		return s, ""
	}
	return s[:idx], strings.TrimLeft(s[idx:], " \t\n")
}

var snowflakeRe = regexp.MustCompile(`^<?[@#]?[!&]?(\d{15,25})>?$`)

func parsePrefixArgs(c *Ctx, options []Option, rest string) error {
	for idx, opt := range options {
		var token string
		isLast := idx == len(options)-1
		if isLast && opt.Type == discord.ApplicationCommandOptionTypeString {
			token, rest = strings.TrimSpace(rest), ""
		} else {
			token, rest = nextToken(rest)
		}
		if token == "" {
			if opt.Required {
				return fmt.Errorf("Missing argument: `%s`", opt.Name)
			}
			continue
		}

		var err error
		switch opt.Type {
		case discord.ApplicationCommandOptionTypeString:
			c.args[opt.Name] = token
		case discord.ApplicationCommandOptionTypeInt:
			var v int64
			v, err = strconv.ParseInt(token, 10, 64)
			c.args[opt.Name] = v
		case discord.ApplicationCommandOptionTypeFloat:
			var v float64
			v, err = strconv.ParseFloat(token, 64)
			c.args[opt.Name] = v
		case discord.ApplicationCommandOptionTypeBool:
			var v bool
			v, err = strconv.ParseBool(token)
			c.args[opt.Name] = v
		case discord.ApplicationCommandOptionTypeUser:
			var u *discord.User
			if id := parseSnowflake(token); id != 0 {
				u, err = c.Client.Rest.GetUser(id)
			} else {
				err = fmt.Errorf("not a user")
			}
			c.args[opt.Name] = u
		case discord.ApplicationCommandOptionTypeChannel:
			var ch discord.GuildChannel
			if id := parseSnowflake(token); id != 0 {
				ch, err = fetchGuildChannel(c.Client, id)
			} else {
				err = fmt.Errorf("not a channel")
			}
			c.args[opt.Name] = ch
		case discord.ApplicationCommandOptionTypeRole:
			var role *discord.Role
			if id := parseSnowflake(token); id != 0 {
				role, err = findRole(c.Client, c.GuildID, id)
			} else {
				err = fmt.Errorf("not a role")
			}
			c.args[opt.Name] = role
		}
		if err != nil {
			delete(c.args, opt.Name)
			return fmt.Errorf("Invalid value for `%s`: %s", opt.Name, token)
		}
	}
	return nil
}

// parseSnowflake extracts the ID from a mention or a raw ID, or returns 0.
func parseSnowflake(token string) snowflake.ID {
	m := snowflakeRe.FindStringSubmatch(token)
	if m == nil {
		return 0
	}
	id, err := snowflake.Parse(m[1])
	if err != nil {
		return 0
	}
	return id
}

func fetchGuildChannel(client *bot.Client, id snowflake.ID) (discord.GuildChannel, error) {
	ch, err := FetchChannel(client, id)
	if err != nil {
		return nil, err
	}
	gc, ok := ch.(discord.GuildChannel)
	if !ok {
		return nil, fmt.Errorf("not a guild channel")
	}
	return gc, nil
}

func findRole(client *bot.Client, guildID, roleID snowflake.ID) (*discord.Role, error) {
	if role, ok := client.Caches.Role(guildID, roleID); ok {
		return &role, nil
	}
	roles, err := client.Rest.GetRoles(guildID)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role.ID == roleID {
			role.GuildID = guildID
			return &role, nil
		}
	}
	return nil, fmt.Errorf("role not found")
}

func usage(prefix string, cmd *Command) string {
	if len(cmd.Subcommands) > 0 {
		var subs []string
		for _, sub := range cmd.Subcommands {
			subs = append(subs, prefix+cmd.Name+" "+sub.Name+optionUsage(sub.Options))
		}
		return "Usage:\n`" + strings.Join(subs, "`\n`") + "`"
	}
	return "Usage: `" + prefix + cmd.Name + optionUsage(cmd.Options) + "`"
}

func optionUsage(options []Option) string {
	var sb strings.Builder
	for _, opt := range options {
		if opt.Required {
			sb.WriteString(" <" + opt.Name + ">")
		} else {
			sb.WriteString(" [" + opt.Name + "]")
		}
	}
	return sb.String()
}

// Usage returns the prefix usage string for a command.
func Usage(prefix string, cmd *Command) string { return usage(prefix, cmd) }
