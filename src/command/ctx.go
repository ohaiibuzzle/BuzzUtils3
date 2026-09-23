package command

import (
	"log"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// Ctx is the invocation context shared by prefix and slash commands.
type Ctx struct {
	Session     *discordgo.Session
	Message     *discordgo.Message     // the invoking message (prefix commands only)
	Interaction *discordgo.Interaction // the invoking interaction (slash / context menu only)
	Command     *Command
	ChannelID   string
	GuildID     string
	Author      *discordgo.User

	args   map[string]any
	target *discordgo.Message // context-menu target

	mu        sync.Mutex
	deferred  bool
	responded bool
}

func (c *Ctx) IsSlash() bool { return c.Interaction != nil }
func (c *Ctx) IsDM() bool    { return c.GuildID == "" }

// Args returns a copy of the parsed arguments.
func (c *Ctx) Args() map[string]any {
	out := make(map[string]any, len(c.args))
	for k, v := range c.args {
		out[k] = v
	}
	return out
}

func (c *Ctx) String(name string) string {
	v, _ := c.args[name].(string)
	return v
}

func (c *Ctx) Int(name string) (int64, bool) {
	v, ok := c.args[name].(int64)
	return v, ok
}

func (c *Ctx) Bool(name string) bool {
	v, _ := c.args[name].(bool)
	return v
}

func (c *Ctx) User(name string) *discordgo.User {
	v, _ := c.args[name].(*discordgo.User)
	return v
}

func (c *Ctx) Channel(name string) *discordgo.Channel {
	v, _ := c.args[name].(*discordgo.Channel)
	return v
}

func (c *Ctx) Role(name string) *discordgo.Role {
	v, _ := c.args[name].(*discordgo.Role)
	return v
}

// Redirect runs another command's handler within this context, with the given arguments.
func (c *Ctx) Redirect(cmd *Command, args map[string]any) {
	c.Command = cmd
	c.args = args
	cmd.Handler(c)
}

// Defer acknowledges a slash command so it can take longer than 3 seconds.
// For prefix commands it shows the typing indicator.
func (c *Ctx) Defer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.IsSlash() {
		c.Session.ChannelTyping(c.ChannelID)
		return
	}
	if c.deferred || c.responded {
		return
	}
	err := c.Session.InteractionRespond(c.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		log.Default().Println("Error deferring interaction: " + err.Error())
		return
	}
	c.deferred = true
}

// Send sends a response. Prefix commands reply to the invoking message; slash commands
// respond to (or follow up on) the interaction.
func (c *Ctx) Send(ms *discordgo.MessageSend) (*discordgo.Message, error) {
	return c.send(ms, false)
}

func (c *Ctx) Reply(content string) (*discordgo.Message, error) {
	return c.Send(&discordgo.MessageSend{Content: content})
}

func (c *Ctx) ReplyEmbed(embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	return c.Send(&discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{embed}})
}

// ReplyPrivate responds ephemerally to slash commands, and normally to prefix commands.
func (c *Ctx) ReplyPrivate(content string) (*discordgo.Message, error) {
	return c.send(&discordgo.MessageSend{Content: content}, true)
}

func (c *Ctx) send(ms *discordgo.MessageSend, ephemeral bool) (*discordgo.Message, error) {
	msg, err := c.sendRaw(ms, ephemeral)
	if err != nil {
		log.Default().Println("Error sending response: " + err.Error())
	}
	return msg, err
}

func (c *Ctx) sendRaw(ms *discordgo.MessageSend, ephemeral bool) (*discordgo.Message, error) {
	if !c.IsSlash() {
		if c.Message != nil {
			// Don't fail if the invoking message has since been deleted
			failIfNotExists := false
			ms.Reference = c.Message.Reference()
			ms.Reference.FailIfNotExists = &failIfNotExists
			if ms.AllowedMentions == nil {
				ms.AllowedMentions = &discordgo.MessageAllowedMentions{
					Parse:       []discordgo.AllowedMentionType{discordgo.AllowedMentionTypeUsers},
					RepliedUser: false,
				}
			}
		}
		return c.Session.ChannelMessageSendComplex(c.ChannelID, ms)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	switch {
	case !c.deferred && !c.responded:
		err := c.Session.InteractionRespond(c.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:         ms.Content,
				Embeds:          ms.Embeds,
				Components:      ms.Components,
				Files:           ms.Files,
				AllowedMentions: ms.AllowedMentions,
				Flags:           flags,
			},
		})
		if err != nil {
			return nil, err
		}
		c.responded = true
		return c.Session.InteractionResponse(c.Interaction)
	case c.deferred && !c.responded:
		c.responded = true
		edit := &discordgo.WebhookEdit{Content: &ms.Content, Files: ms.Files, AllowedMentions: ms.AllowedMentions}
		if ms.Embeds != nil {
			edit.Embeds = &ms.Embeds
		}
		if ms.Components != nil {
			edit.Components = &ms.Components
		}
		return c.Session.InteractionResponseEdit(c.Interaction, edit)
	default:
		return c.Session.FollowupMessageCreate(c.Interaction, true, &discordgo.WebhookParams{
			Content:         ms.Content,
			Embeds:          ms.Embeds,
			Components:      ms.Components,
			Files:           ms.Files,
			AllowedMentions: ms.AllowedMentions,
			Flags:           flags,
		})
	}
}

// Ack makes sure a slash command has been answered, e.g. after a command that only
// reacts or sends DMs. It is a no-op for prefix commands.
func (c *Ctx) Ack(content string) {
	c.mu.Lock()
	done := c.responded
	c.mu.Unlock()
	if c.IsSlash() && !done {
		c.ReplyPrivate(content)
	}
}

// DeleteInvocation deletes the invoking prefix message, if any.
func (c *Ctx) DeleteInvocation() {
	if c.Message != nil {
		c.Session.ChannelMessageDelete(c.ChannelID, c.Message.ID)
	}
}

// IsNSFW reports whether the command was issued in an NSFW guild channel
// (threads inherit from their parent channel).
func (c *Ctx) IsNSFW() bool {
	if c.IsDM() {
		return false
	}
	ch, err := c.fetchChannel(c.ChannelID)
	if err != nil {
		return false
	}
	if ch.IsThread() && ch.ParentID != "" {
		if parent, err := c.fetchChannel(ch.ParentID); err == nil {
			return parent.NSFW
		}
	}
	return ch.NSFW
}

func (c *Ctx) fetchChannel(id string) (*discordgo.Channel, error) {
	if ch, err := c.Session.State.Channel(id); err == nil {
		return ch, nil
	}
	return c.Session.Channel(id)
}

// HasPermission reports whether the invoking member has all the given permissions
// in the current channel.
func (c *Ctx) HasPermission(perm int64) bool {
	if perm == 0 {
		return true
	}
	if c.IsDM() {
		return false
	}
	var perms int64
	if c.IsSlash() && c.Interaction.Member != nil {
		perms = c.Interaction.Member.Permissions
	} else {
		p, err := c.Session.UserChannelPermissions(c.Author.ID, c.ChannelID)
		if err != nil {
			return false
		}
		perms = p
	}
	if perms&discordgo.PermissionAdministrator != 0 {
		return true
	}
	return perms&perm == perm
}

// FindMessage returns the message a command should act on: the context-menu target,
// the message a prefix command replied to, or the most recent of the last `limit`
// channel messages matching `match`.
func (c *Ctx) FindMessage(limit int, match func(*discordgo.Message) bool) *discordgo.Message {
	if c.target != nil {
		return c.target
	}
	if c.Message != nil && c.Message.MessageReference != nil {
		ref := c.Message.ReferencedMessage
		if ref == nil {
			ref, _ = c.Session.ChannelMessage(c.ChannelID, c.Message.MessageReference.MessageID)
		}
		if ref != nil {
			return ref
		}
	}
	messages, err := c.Session.ChannelMessages(c.ChannelID, limit, "", "", "")
	if err != nil {
		log.Default().Println("Error fetching channel history: " + err.Error())
		return nil
	}
	for _, m := range messages {
		if c.Message != nil && m.ID == c.Message.ID {
			continue
		}
		if match(m) {
			return m
		}
	}
	return nil
}
