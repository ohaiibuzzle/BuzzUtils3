package command

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Button struct {
	Label string
	Style discordgo.ButtonStyle
	Value string
}

type buttonWaiter struct {
	allowedUser string
	result      chan string
}

var (
	waitersMu sync.Mutex
	waiters   = map[string]*buttonWaiter{}
)

// Ask sends a message with buttons as the command's response and waits for
// allowedUser to click one. It returns the clicked button's Value, or false on timeout.
func (c *Ctx) Ask(ms *discordgo.MessageSend, allowedUser string, buttons []Button, timeout time.Duration) (string, bool) {
	return ask(c.Session, c.Send, ms, allowedUser, buttons, timeout)
}

// AskInChannel is like Ask, but posts to an arbitrary channel (e.g. a DM).
func AskInChannel(s *discordgo.Session, channelID string, ms *discordgo.MessageSend, allowedUser string, buttons []Button, timeout time.Duration) (string, bool) {
	send := func(ms *discordgo.MessageSend) (*discordgo.Message, error) {
		return s.ChannelMessageSendComplex(channelID, ms)
	}
	return ask(s, send, ms, allowedUser, buttons, timeout)
}

func ask(s *discordgo.Session, send func(*discordgo.MessageSend) (*discordgo.Message, error), ms *discordgo.MessageSend, allowedUser string, buttons []Button, timeout time.Duration) (string, bool) {
	id := newID()
	w := &buttonWaiter{allowedUser: allowedUser, result: make(chan string, 1)}
	waitersMu.Lock()
	waiters[id] = w
	waitersMu.Unlock()
	defer func() {
		waitersMu.Lock()
		delete(waiters, id)
		waitersMu.Unlock()
	}()

	ms.Components = []discordgo.MessageComponent{buttonRow(id, buttons, false)}
	msg, err := send(ms)
	if err != nil {
		return "", false
	}

	select {
	case value := <-w.result:
		return value, true
	case <-time.After(timeout):
		if msg != nil {
			components := []discordgo.MessageComponent{buttonRow(id, buttons, true)}
			s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         msg.ID,
				Channel:    msg.ChannelID,
				Components: &components,
			})
		}
		return "", false
	}
}

func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID
	id, value, found := strings.Cut(customID, ":")
	if !found {
		return
	}

	waitersMu.Lock()
	w := waiters[id]
	waitersMu.Unlock()

	if w == nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This prompt has expired.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var clicker string
	if i.Member != nil && i.Member.User != nil {
		clicker = i.Member.User.ID
	} else if i.User != nil {
		clicker = i.User.ID
	}
	if w.allowedUser != "" && clicker != w.allowedUser {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This isn't for you!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Disable the buttons now that a choice has been made
	var disabled []discordgo.MessageComponent
	for _, row := range i.Message.Components {
		if r, ok := row.(*discordgo.ActionsRow); ok {
			for _, comp := range r.Components {
				if b, ok := comp.(*discordgo.Button); ok {
					b.Disabled = true
				}
			}
			disabled = append(disabled, r)
		}
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    i.Message.Content,
			Embeds:     i.Message.Embeds,
			Components: disabled,
		},
	})

	select {
	case w.result <- value:
	default:
	}
}

func buttonRow(id string, buttons []Button, disabled bool) *discordgo.ActionsRow {
	row := &discordgo.ActionsRow{}
	for _, b := range buttons {
		style := b.Style
		if style == 0 {
			style = discordgo.PrimaryButton
		}
		row.Components = append(row.Components, &discordgo.Button{
			Label:    b.Label,
			Style:    style,
			CustomID: id + ":" + b.Value,
			Disabled: disabled,
		})
	}
	return row
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Field builds an embed field, making sure it's valid for Discord
// (non-empty, and within the length limits).
func Field(name, value string, inline bool) *discordgo.MessageEmbedField {
	if strings.TrimSpace(name) == "" {
		name = "​"
	}
	if strings.TrimSpace(value) == "" {
		value = "N/A"
	}
	return &discordgo.MessageEmbedField{
		Name:   truncate(name, 256),
		Value:  truncate(value, 1024),
		Inline: inline,
	}
}

// CodeField builds an embed field whose value is shown in a code block.
func CodeField(name, value string) *discordgo.MessageEmbedField {
	return Field(name, "```\n"+truncate(value, 1000)+"\n```", false)
}
