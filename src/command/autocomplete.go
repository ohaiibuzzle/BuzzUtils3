package command

import (
	"log"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

// Discord sends an autocomplete request for nearly every keystroke. Each request
// waits this long, and is dropped if the same user typed again in the meantime,
// so the upstream sites only see the text the user paused on.
const autocompleteDebounce = 300 * time.Millisecond

const maxAutocompleteChoices = 25

var (
	keystrokesMu sync.Mutex
	keystrokes   = map[snowflake.ID]snowflake.ID{} // user -> latest interaction
)

// HandleAutocomplete answers autocomplete requests for String options.
func HandleAutocomplete(e *events.AutocompleteInteractionCreate) {
	cmd := Lookup(e.Data.CommandName)
	if cmd == nil {
		return
	}
	target := cmd
	if e.Data.SubCommandName != nil {
		if target = cmd.subcommand(*e.Data.SubCommandName); target == nil {
			return
		}
	}
	focused := e.Data.Focused()
	var opt *Option
	for i := range target.Options {
		if target.Options[i].Name == focused.Name {
			opt = &target.Options[i]
		}
	}
	if opt == nil || opt.Autocomplete == nil {
		return
	}

	userID := e.User().ID
	keystrokesMu.Lock()
	keystrokes[userID] = e.ID()
	keystrokesMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Default().Printf("Panic while autocompleting: %v", r)
			}
		}()

		time.Sleep(autocompleteDebounce)
		keystrokesMu.Lock()
		latest := keystrokes[userID] == e.ID()
		if latest {
			delete(keystrokes, userID)
		}
		keystrokesMu.Unlock()
		if !latest {
			return
		}

		c := &Ctx{Client: e.Client(), Command: target, Author: e.User(), args: map[string]any{}}
		if ch := e.Channel(); ch.MessageChannel != nil {
			c.ChannelID = ch.ID()
		}
		if guildID := e.GuildID(); guildID != nil {
			c.GuildID = *guildID
		}

		typed, _ := e.Data.OptString(focused.Name)
		choices := opt.Autocomplete(c, typed)
		if len(choices) > maxAutocompleteChoices {
			choices = choices[:maxAutocompleteChoices]
		}
		if choices == nil {
			choices = []discord.AutocompleteChoice{}
		}
		if err := e.AutocompleteResult(choices); err != nil {
			log.Default().Println("Error responding to autocomplete: " + err.Error())
		}
	}()
}
