// Package command provides a small "bridge" command framework: every command is
// declared once and is reachable both as a prefix command (e.g. ".zerochan hu tao")
// and as a slash command (e.g. "/zerochan tags:hu tao").
package command

import (
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type Command struct {
	Name        string
	Aliases     []string // prefix-only aliases
	Description string
	Options     []*discordgo.ApplicationCommandOption
	Subcommands []*Command
	GuildOnly   bool
	Permissions int64 // required member permissions, 0 for none
	OwnerOnly   bool
	PrefixOnly  bool // do not register as a slash command
	Handler     func(c *Ctx)
}

// MessageAction is a message context-menu command (right click → Apps).
type MessageAction struct {
	Name    string
	Handler func(c *Ctx, target *discordgo.Message)
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

func (cmd *Command) applicationCommand() *discordgo.ApplicationCommand {
	ac := &discordgo.ApplicationCommand{
		Type:        discordgo.ChatApplicationCommand,
		Name:        cmd.Name,
		Description: truncate(cmd.Description, 100),
		Options:     cmd.Options,
	}
	for _, sub := range cmd.Subcommands {
		ac.Options = append(ac.Options, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        sub.Name,
			Description: truncate(sub.Description, 100),
			Options:     sub.Options,
		})
	}
	if cmd.Permissions != 0 {
		perms := cmd.Permissions
		ac.DefaultMemberPermissions = &perms
	}
	if cmd.GuildOnly {
		contexts := []discordgo.InteractionContextType{discordgo.InteractionContextGuild}
		ac.Contexts = &contexts
	}
	return ac
}

// ApplicationCommands returns the slash and context-menu command definitions.
func ApplicationCommands() []*discordgo.ApplicationCommand {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var appCommands []*discordgo.ApplicationCommand
	for _, cmd := range commands {
		if !cmd.PrefixOnly && !cmd.OwnerOnly {
			appCommands = append(appCommands, cmd.applicationCommand())
		}
	}
	for name := range actions {
		appCommands = append(appCommands, &discordgo.ApplicationCommand{
			Type: discordgo.MessageApplicationCommand,
			Name: name,
		})
	}
	return appCommands
}

// Sync overwrites the bot's global application commands with everything registered.
// Global commands can take up to an hour to propagate.
func Sync(s *discordgo.Session) error {
	appCommands := ApplicationCommands()
	_, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, "", appCommands)
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
