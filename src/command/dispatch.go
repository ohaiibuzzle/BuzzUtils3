package command

import (
	"fmt"
	"log"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// HandleMessage dispatches a prefix command. It returns false if the message
// isn't a known command.
func HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate, prefix string) bool {
	if prefix == "" || !strings.HasPrefix(m.Content, prefix) {
		return false
	}
	name, rest := nextToken(m.Content[len(prefix):])
	cmd := Lookup(name)
	if cmd == nil {
		return false
	}

	c := &Ctx{
		Session:   s,
		Message:   m.Message,
		Command:   cmd,
		ChannelID: m.ChannelID,
		GuildID:   m.GuildID,
		Author:    m.Author,
		args:      map[string]any{},
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

// HandleInteraction dispatches slash commands, message context-menu commands and
// button clicks.
func HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionMessageComponent:
		handleComponent(s, i)
		return
	case discordgo.InteractionApplicationCommand:
	default:
		return
	}

	data := i.ApplicationCommandData()
	c := &Ctx{
		Session:     s,
		Interaction: i.Interaction,
		ChannelID:   i.ChannelID,
		GuildID:     i.GuildID,
		args:        map[string]any{},
	}
	if i.Member != nil {
		c.Author = i.Member.User
	} else {
		c.Author = i.User
	}

	if data.CommandType == discordgo.MessageApplicationCommand {
		registryMu.RLock()
		act := actions[data.Name]
		registryMu.RUnlock()
		if act == nil || data.Resolved == nil {
			return
		}
		c.target = data.Resolved.Messages[data.TargetID]
		if c.target != nil && c.target.GuildID == "" {
			c.target.GuildID = i.GuildID
		}
		log.Default().Println("User " + c.Author.Username + " used message action: " + act.Name)
		go func() {
			defer recoverPanic(c)
			act.Handler(c, c.target)
		}()
		return
	}

	cmd := Lookup(data.Name)
	if cmd == nil {
		return
	}
	c.Command = cmd
	log.Default().Println("User " + c.Author.Username + " issued slash command: " + cmd.Name)

	go func() {
		defer recoverPanic(c)
		target := cmd
		options := data.Options
		if len(cmd.Subcommands) > 0 && len(options) > 0 {
			target = cmd.subcommand(options[0].Name)
			if target == nil {
				return
			}
			options = options[0].Options
		}
		for _, opt := range options {
			switch opt.Type {
			case discordgo.ApplicationCommandOptionString:
				c.args[opt.Name] = opt.StringValue()
			case discordgo.ApplicationCommandOptionInteger:
				c.args[opt.Name] = opt.IntValue()
			case discordgo.ApplicationCommandOptionNumber:
				c.args[opt.Name] = opt.FloatValue()
			case discordgo.ApplicationCommandOptionBoolean:
				c.args[opt.Name] = opt.BoolValue()
			case discordgo.ApplicationCommandOptionUser:
				c.args[opt.Name] = opt.UserValue(s)
			case discordgo.ApplicationCommandOptionChannel:
				c.args[opt.Name] = opt.ChannelValue(s)
			case discordgo.ApplicationCommandOptionRole:
				c.args[opt.Name] = opt.RoleValue(s, i.GuildID)
			}
		}
		run(c, cmd, target)
	}()
}

func run(c *Ctx, cmd, target *Command) {
	if (cmd.GuildOnly || target.GuildOnly) && c.IsDM() {
		c.ReplyPrivate("This command can only be used inside a server.")
		return
	}
	if (cmd.OwnerOnly || target.OwnerOnly) && !IsOwner(c.Session, c.Author.ID) {
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
	ownerIDs  = map[string]bool{}
)

// IsOwner reports whether the user owns the bot application (or is on its team).
func IsOwner(s *discordgo.Session, userID string) bool {
	ownerOnce.Do(func() {
		app, err := s.Application("@me")
		if err != nil {
			log.Default().Println("Error fetching application info: " + err.Error())
			return
		}
		if app.Owner != nil {
			ownerIDs[app.Owner.ID] = true
		}
		if app.Team != nil {
			for _, member := range app.Team.Members {
				if member.User != nil {
					ownerIDs[member.User.ID] = true
				}
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

func parsePrefixArgs(c *Ctx, options []*discordgo.ApplicationCommandOption, rest string) error {
	for idx, opt := range options {
		var token string
		isLast := idx == len(options)-1
		if isLast && opt.Type == discordgo.ApplicationCommandOptionString {
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
		case discordgo.ApplicationCommandOptionString:
			c.args[opt.Name] = token
		case discordgo.ApplicationCommandOptionInteger:
			var v int64
			v, err = strconv.ParseInt(token, 10, 64)
			c.args[opt.Name] = v
		case discordgo.ApplicationCommandOptionNumber:
			var v float64
			v, err = strconv.ParseFloat(token, 64)
			c.args[opt.Name] = v
		case discordgo.ApplicationCommandOptionBoolean:
			var v bool
			v, err = strconv.ParseBool(token)
			c.args[opt.Name] = v
		case discordgo.ApplicationCommandOptionUser:
			var u *discordgo.User
			if id := snowflake(token); id != "" {
				u, err = c.Session.User(id)
			} else {
				err = fmt.Errorf("not a user")
			}
			c.args[opt.Name] = u
		case discordgo.ApplicationCommandOptionChannel:
			var ch *discordgo.Channel
			if id := snowflake(token); id != "" {
				ch, err = c.fetchChannel(id)
			} else {
				err = fmt.Errorf("not a channel")
			}
			c.args[opt.Name] = ch
		case discordgo.ApplicationCommandOptionRole:
			var role *discordgo.Role
			if id := snowflake(token); id != "" {
				role, err = findRole(c.Session, c.GuildID, id)
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

func snowflake(token string) string {
	m := snowflakeRe.FindStringSubmatch(token)
	if m == nil {
		return ""
	}
	return m[1]
}

func findRole(s *discordgo.Session, guildID, roleID string) (*discordgo.Role, error) {
	if role, err := s.State.Role(guildID, roleID); err == nil {
		return role, nil
	}
	roles, err := s.GuildRoles(guildID)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role.ID == roleID {
			return role, nil
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

func optionUsage(options []*discordgo.ApplicationCommandOption) string {
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
