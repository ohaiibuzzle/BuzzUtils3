package bot

import (
	"regexp"
	"testing"

	"github.com/bwmarrin/discordgo"
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
		key := string(rune(ac.Type)) + ac.Name
		if seen[key] {
			t.Errorf("duplicate command %q", ac.Name)
		}
		seen[key] = true

		if ac.Type == discordgo.MessageApplicationCommand {
			messageActions++
			if l := len([]rune(ac.Name)); l < 1 || l > 32 {
				t.Errorf("message command %q name must be 1-32 characters", ac.Name)
			}
			continue
		}

		if !slashNamePattern.MatchString(ac.Name) {
			t.Errorf("invalid slash command name %q", ac.Name)
		}
		checkDescription(t, ac.Name, ac.Description)
		checkOptions(t, ac.Name, ac.Options)
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

func checkOptions(t *testing.T, parent string, options []*discordgo.ApplicationCommandOption) {
	if len(options) > 25 {
		t.Errorf("%s: more than 25 options", parent)
	}
	optional := false
	for _, opt := range options {
		path := parent + " " + opt.Name
		if !slashNamePattern.MatchString(opt.Name) {
			t.Errorf("invalid option name %q", path)
		}
		checkDescription(t, path, opt.Description)
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand {
			checkOptions(t, path, opt.Options)
			continue
		}
		if opt.Required && optional {
			t.Errorf("%s: required options must come before optional ones", path)
		}
		optional = optional || !opt.Required
	}
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
