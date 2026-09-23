package command

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

type Button struct {
	Label string
	Style discord.ButtonStyle
	Value string
}

type buttonWaiter struct {
	allowedUser snowflake.ID
	buttons     []Button
	result      chan string
}

var (
	waitersMu sync.Mutex
	waiters   = map[string]*buttonWaiter{}
)

// Ask sends a message with buttons as the command's response and waits for
// allowedUser to click one. It returns the clicked button's Value, or false on timeout.
func (c *Ctx) Ask(ms discord.MessageCreate, allowedUser snowflake.ID, buttons []Button, timeout time.Duration) (string, bool) {
	return ask(c.Client, c.Send, ms, allowedUser, buttons, timeout)
}

// AskInChannel is like Ask, but posts to an arbitrary channel (e.g. a DM).
func AskInChannel(client *bot.Client, channelID snowflake.ID, ms discord.MessageCreate, allowedUser snowflake.ID, buttons []Button, timeout time.Duration) (string, bool) {
	send := func(ms discord.MessageCreate) (*discord.Message, error) {
		return client.Rest.CreateMessage(channelID, ms)
	}
	return ask(client, send, ms, allowedUser, buttons, timeout)
}

func ask(client *bot.Client, send func(discord.MessageCreate) (*discord.Message, error), ms discord.MessageCreate, allowedUser snowflake.ID, buttons []Button, timeout time.Duration) (string, bool) {
	id := newID()
	w := &buttonWaiter{allowedUser: allowedUser, buttons: buttons, result: make(chan string, 1)}
	waitersMu.Lock()
	waiters[id] = w
	waitersMu.Unlock()
	defer func() {
		waitersMu.Lock()
		delete(waiters, id)
		waitersMu.Unlock()
	}()

	ms.Components = buttonRow(id, buttons, false)
	msg, err := send(ms)
	if err != nil {
		return "", false
	}

	select {
	case value := <-w.result:
		return value, true
	case <-time.After(timeout):
		if msg != nil {
			components := buttonRow(id, buttons, true)
			client.Rest.UpdateMessage(msg.ChannelID, msg.ID, discord.MessageUpdate{Components: &components})
		}
		return "", false
	}
}

func handleComponent(e *events.ComponentInteractionCreate) {
	id, value, found := strings.Cut(e.Data.CustomID(), ":")
	if !found {
		return
	}

	waitersMu.Lock()
	w := waiters[id]
	waitersMu.Unlock()

	if w == nil {
		e.CreateMessage(discord.MessageCreate{Content: "This prompt has expired.", Flags: discord.MessageFlagEphemeral})
		return
	}

	if w.allowedUser != 0 && e.User().ID != w.allowedUser {
		e.CreateMessage(discord.MessageCreate{Content: "This isn't for you!", Flags: discord.MessageFlagEphemeral})
		return
	}

	// Disable the buttons now that a choice has been made
	disabled := buttonRow(id, w.buttons, true)
	e.UpdateMessage(discord.MessageUpdate{Components: &disabled})

	select {
	case w.result <- value:
	default:
	}
}

func buttonRow(id string, buttons []Button, disabled bool) []discord.LayoutComponent {
	var row []discord.InteractiveComponent
	for _, b := range buttons {
		style := b.Style
		if style == 0 {
			style = discord.ButtonStylePrimary
		}
		row = append(row, discord.ButtonComponent{
			Label:    b.Label,
			Style:    style,
			CustomID: id + ":" + b.Value,
			Disabled: disabled,
		})
	}
	return []discord.LayoutComponent{discord.NewActionRow(row...)}
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Field builds an embed field, making sure it's valid for Discord
// (non-empty, and within the length limits).
func Field(name, value string, inline bool) discord.EmbedField {
	if strings.TrimSpace(name) == "" {
		name = "​"
	}
	if strings.TrimSpace(value) == "" {
		value = "N/A"
	}
	return discord.EmbedField{
		Name:   truncate(name, 256),
		Value:  truncate(value, 1024),
		Inline: &inline,
	}
}

// CodeField builds an embed field whose value is shown in a code block.
func CodeField(name, value string) discord.EmbedField {
	return Field(name, "```\n"+truncate(value, 1000)+"\n```", false)
}
