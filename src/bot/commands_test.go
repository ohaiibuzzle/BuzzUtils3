package bot

import (
	"regexp"
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

var slashNamePattern = regexp.MustCompile(`^[-_\p{Ll}\p{N}]{1,32}$`)

// Discord rejects the whole bulk overwrite if any command is invalid, so check
// every registered definition against Discord's rules.
func TestApplicationCommandsValid(t *testing.T) {
	appCommands := command.ApplicationCommands()
	if len(appCommands) > 100 {
		t.Fatalf("%d commands exceeds Discord's limit of 100", len(appCommands))
	}

	seen := map[string]bool{}
	messageActions := 0
	for _, ac := range appCommands {
		key := string(rune(ac.Type())) + ac.CommandName()
		if seen[key] {
			t.Errorf("duplicate command %q", ac.CommandName())
		}
		seen[key] = true

		switch ac := ac.(type) {
		case discord.MessageCommandCreate:
			messageActions++
			if l := len([]rune(ac.Name)); l < 1 || l > 32 {
				t.Errorf("message command %q name must be 1-32 characters", ac.Name)
			}
		case discord.SlashCommandCreate:
			if !slashNamePattern.MatchString(ac.Name) {
				t.Errorf("invalid slash command name %q", ac.Name)
			}
			checkDescription(t, ac.Name, ac.Description)
			checkOptions(t, ac.Name, ac.Options)
		default:
			t.Errorf("unexpected command type %T", ac)
		}
	}
	if messageActions > 5 {
		t.Errorf("%d message commands exceeds Discord's limit of 5", messageActions)
	}
}

func checkDescription(t *testing.T, name, description string) {
	if l := len([]rune(description)); l < 1 || l > 100 {
		t.Errorf("%s: description must be 1-100 characters, got %d", name, l)
	}
}

func checkOptions(t *testing.T, parent string, options []discord.ApplicationCommandOption) {
	if len(options) > 25 {
		t.Errorf("%s: more than 25 options", parent)
	}
	optional := false
	for _, opt := range options {
		path := parent + " " + opt.OptionName()
		if !slashNamePattern.MatchString(opt.OptionName()) {
			t.Errorf("invalid option name %q", path)
		}
		checkDescription(t, path, opt.OptionDescription())
		if sub, ok := opt.(discord.ApplicationCommandOptionSubCommand); ok {
			checkOptions(t, path, sub.Options)
			continue
		}
		required := isRequired(opt)
		if required && optional {
			t.Errorf("%s: required options must come before optional ones", path)
		}
		optional = optional || !required
	}
}

func isRequired(opt discord.ApplicationCommandOption) bool {
	switch o := opt.(type) {
	case discord.ApplicationCommandOptionString:
		return o.Required
	case discord.ApplicationCommandOptionInt:
		return o.Required
	case discord.ApplicationCommandOptionFloat:
		return o.Required
	case discord.ApplicationCommandOptionBool:
		return o.Required
	case discord.ApplicationCommandOptionUser:
		return o.Required
	case discord.ApplicationCommandOptionChannel:
		return o.Required
	case discord.ApplicationCommandOptionRole:
		return o.Required
	}
	return false
}

func TestPrefixAliasesResolve(t *testing.T) {
	for alias, want := range map[string]string{
		"sbr": "safebooru", "zcr": "zerochan", "dbr": "danbooru", "pxr": "pixivrandom",
		"pxs": "pixivshow", "police": "predict", "oofie": "oof", "save": "savethis",
		"bday": "birthday", "setNSFWRole": "setnsfwrole",
	} {
		cmd := command.Lookup(alias)
		if cmd == nil || cmd.Name != want {
			t.Errorf("alias %q should resolve to %q, got %v", alias, want, cmd)
		}
	}
}
