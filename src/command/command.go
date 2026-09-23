// Package command provides a small "bridge" command framework: every command is
// declared once and is reachable both as a prefix command (e.g. ".zerochan hu tao")
// and as a slash command (e.g. "/zerochan tags:hu tao").
package command

import (
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"
)

type Command struct {
	Name        string
	Aliases     []string // prefix-only aliases
	Description string
	Options     []Option
	Subcommands []*Command
	GuildOnly   bool
	Permissions discord.Permissions // required member permissions, 0 for none
	OwnerOnly   bool
	PrefixOnly  bool // do not register as a slash command
	Handler     func(c *Ctx)
}

// MessageAction is a message context-menu command (right click → Apps).
type MessageAction struct {
	Name    string
	Handler func(c *Ctx, target *discord.Message)
}

// Option is a command argument. It is shared by the prefix parser and the slash
// command definition, so only the option types the dispatcher understands are used:
// String, Int, Float, Bool, User, Channel and Role.
type Option struct {
	Type         discord.ApplicationCommandOptionType
	Name         string
	Description  string
	Required     bool
	MinValue     *int // Int options only
	MaxValue     *int // Int options only
	ChannelTypes []discord.ChannelType
}

func (o Option) applicationCommandOption() discord.ApplicationCommandOption {
	switch o.Type {
	case discord.ApplicationCommandOptionTypeInt:
		return discord.ApplicationCommandOptionInt{Name: o.Name, Description: o.Description, Required: o.Required, MinValue: o.MinValue, MaxValue: o.MaxValue}
	case discord.ApplicationCommandOptionTypeFloat:
		return discord.ApplicationCommandOptionFloat{Name: o.Name, Description: o.Description, Required: o.Required}
	case discord.ApplicationCommandOptionTypeBool:
		return discord.ApplicationCommandOptionBool{Name: o.Name, Description: o.Description, Required: o.Required}
	case discord.ApplicationCommandOptionTypeUser:
		return discord.ApplicationCommandOptionUser{Name: o.Name, Description: o.Description, Required: o.Required}
	case discord.ApplicationCommandOptionTypeChannel:
		return discord.ApplicationCommandOptionChannel{Name: o.Name, Description: o.Description, Required: o.Required, ChannelTypes: o.ChannelTypes}
	case discord.ApplicationCommandOptionTypeRole:
		return discord.ApplicationCommandOptionRole{Name: o.Name, Description: o.Description, Required: o.Required}
	default:
		return discord.ApplicationCommandOptionString{Name: o.Name, Description: o.Description, Required: o.Required}
	}
}

func applicationCommandOptions(options []Option) []discord.ApplicationCommandOption {
	var out []discord.ApplicationCommandOption
	for _, o := range options {
		out = append(out, o.applicationCommandOption())
	}
	return out
}

var (
	registryMu sync.RWMutex
	commands   []*Command
	byName     = map[string]*Command{}
	actions    = map[string]*MessageAction{}
)

func Register(cmds ...*Command) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, cmd := range cmds {
		commands = append(commands, cmd)
		byName[strings.ToLower(cmd.Name)] = cmd
		for _, alias := range cmd.Aliases {
			byName[strings.ToLower(alias)] = cmd
		}
	}
}

func RegisterMessageActions(acts ...*MessageAction) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, act := range acts {
		actions[act.Name] = act
	}
}

// Lookup finds a command by name or alias.
func Lookup(name string) *Command {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return byName[strings.ToLower(name)]
}

// All returns every registered command, sorted by name.
func All() []*Command {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := append([]*Command(nil), commands...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (cmd *Command) subcommand(name string) *Command {
	for _, sub := range cmd.Subcommands {
		if strings.EqualFold(sub.Name, name) {
			return sub
		}
	}
	return nil
}

func (cmd *Command) applicationCommand() discord.SlashCommandCreate {
	ac := discord.SlashCommandCreate{
		Name:        cmd.Name,
		Description: truncate(cmd.Description, 100),
		Options:     applicationCommandOptions(cmd.Options),
	}
	for _, sub := range cmd.Subcommands {
		ac.Options = append(ac.Options, discord.ApplicationCommandOptionSubCommand{
			Name:        sub.Name,
			Description: truncate(sub.Description, 100),
			Options:     applicationCommandOptions(sub.Options),
		})
	}
	if cmd.Permissions != 0 {
		ac.DefaultMemberPermissions = omit.NewPtr(cmd.Permissions)
	}
	if cmd.GuildOnly {
		ac.Contexts = []discord.InteractionContextType{discord.InteractionContextTypeGuild}
	}
	return ac
}

// ApplicationCommands returns the slash and context-menu command definitions.
func ApplicationCommands() []discord.ApplicationCommandCreate {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var appCommands []discord.ApplicationCommandCreate
	for _, cmd := range commands {
		if !cmd.PrefixOnly && !cmd.OwnerOnly {
			appCommands = append(appCommands, cmd.applicationCommand())
		}
	}
	for name := range actions {
		appCommands = append(appCommands, discord.MessageCommandCreate{Name: name})
	}
	return appCommands
}

// Sync overwrites the bot's global application commands with everything registered.
// Global commands can take up to an hour to propagate.
func Sync(client *bot.Client) error {
	appCommands := ApplicationCommands()
	_, err := client.Rest.SetGlobalCommands(client.ApplicationID, appCommands)
	if err == nil {
		log.Default().Printf("Registered %d application commands", len(appCommands))
	}
	return err
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
