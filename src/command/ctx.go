package command

import (
	"log"
	"sync"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Ctx is the invocation context shared by prefix and slash commands.
type Ctx struct {
	Client      *bot.Client
	Message     *discord.Message                       // the invoking message (prefix commands only)
	Interaction *discord.ApplicationCommandInteraction // the invoking interaction (slash / context menu only)
	Command     *Command
	ChannelID   snowflake.ID
	GuildID     snowflake.ID // 0 in DMs
	Author      discord.User

	args   map[string]any
	target *discord.Message // context-menu target

	mu        sync.Mutex
	deferred  bool
	responded bool
}

func (c *Ctx) IsSlash() bool { return c.Interaction != nil }
func (c *Ctx) IsDM() bool    { return c.GuildID == 0 }

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

func (c *Ctx) User(name string) *discord.User {
	v, _ := c.args[name].(*discord.User)
	return v
}

func (c *Ctx) Channel(name string) discord.GuildChannel {
	v, _ := c.args[name].(discord.GuildChannel)
	return v
}

func (c *Ctx) Role(name string) *discord.Role {
	v, _ := c.args[name].(*discord.Role)
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
		c.Client.Rest.SendTyping(c.ChannelID)
		return
	}
	if c.deferred || c.responded {
		return
	}
	err := c.Client.Rest.CreateInteractionResponse(c.Interaction.ID(), c.Interaction.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
	})
	if err != nil {
		log.Default().Println("Error deferring interaction: " + err.Error())
		return
	}
	c.deferred = true
}

// Send sends a response. Prefix commands reply to the invoking message; slash commands
// respond to (or follow up on) the interaction.
func (c *Ctx) Send(ms discord.MessageCreate) (*discord.Message, error) {
	return c.send(ms, false)
}

func (c *Ctx) Reply(content string) (*discord.Message, error) {
	return c.Send(discord.MessageCreate{Content: content})
}

func (c *Ctx) ReplyEmbed(embed discord.Embed) (*discord.Message, error) {
	return c.Send(discord.MessageCreate{Embeds: []discord.Embed{embed}})
}

// ReplyPrivate responds ephemerally to slash commands, and normally to prefix commands.
func (c *Ctx) ReplyPrivate(content string) (*discord.Message, error) {
	return c.send(discord.MessageCreate{Content: content}, true)
}

func (c *Ctx) send(ms discord.MessageCreate, ephemeral bool) (*discord.Message, error) {
	msg, err := c.sendRaw(ms, ephemeral)
	if err != nil {
		log.Default().Println("Error sending response: " + err.Error())
	}
	return msg, err
}

func (c *Ctx) sendRaw(ms discord.MessageCreate, ephemeral bool) (*discord.Message, error) {
	if !c.IsSlash() {
		if c.Message == nil {
			return c.Client.Rest.CreateMessage(c.ChannelID, ms)
		}
		if ms.AllowedMentions == nil {
			ms.AllowedMentions = &discord.AllowedMentions{
				Parse:       []discord.AllowedMentionType{discord.AllowedMentionTypeUsers},
				RepliedUser: false,
			}
		}
		reply := ms
		reply.MessageReference = &discord.MessageReference{MessageID: &c.Message.ID, ChannelID: &c.ChannelID}
		msg, err := c.Client.Rest.CreateMessage(c.ChannelID, reply)
		if err == nil {
			return msg, nil
		}
		// The invoking message may have since been deleted (disgo can't send
		// fail_if_not_exists: false), so try again without replying to it
		return c.Client.Rest.CreateMessage(c.ChannelID, ms)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ephemeral {
		ms.Flags = ms.Flags.Add(discord.MessageFlagEphemeral)
	}

	appID, token := c.Interaction.ApplicationID(), c.Interaction.Token()
	switch {
	case !c.deferred && !c.responded:
		err := c.Client.Rest.CreateInteractionResponse(c.Interaction.ID(), token, discord.InteractionResponse{
			Type: discord.InteractionResponseTypeCreateMessage,
			Data: ms,
		})
		if err != nil {
			return nil, err
		}
		c.responded = true
		return c.Client.Rest.GetInteractionResponse(appID, token)
	case c.deferred && !c.responded:
		c.responded = true
		edit := discord.MessageUpdate{Content: &ms.Content, Files: ms.Files, AllowedMentions: ms.AllowedMentions}
		if ms.Embeds != nil {
			edit.Embeds = &ms.Embeds
		}
		if ms.Components != nil {
			edit.Components = &ms.Components
		}
		return c.Client.Rest.UpdateInteractionResponse(appID, token, edit)
	default:
		return c.Client.Rest.CreateFollowupMessage(appID, token, ms)
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
		c.Client.Rest.DeleteMessage(c.ChannelID, c.Message.ID)
	}
}

// IsNSFW reports whether the command was issued in an NSFW guild channel
// (threads inherit from their parent channel).
func (c *Ctx) IsNSFW() bool {
	if c.IsDM() {
		return false
	}
	ch, err := FetchChannel(c.Client, c.ChannelID)
	if err != nil {
		return false
	}
	if thread, ok := ch.(discord.GuildThread); ok {
		parent, err := FetchChannel(c.Client, *thread.ParentID())
		if err != nil {
			return false
		}
		ch = parent
	}
	if mc, ok := ch.(discord.GuildMessageChannel); ok {
		return mc.NSFW()
	}
	return false
}

// FetchChannel returns a channel from the cache, or from the API if it isn't cached.
func FetchChannel(client *bot.Client, id snowflake.ID) (discord.Channel, error) {
	if ch, ok := client.Caches.Channel(id); ok {
		return ch, nil
	}
	return client.Rest.GetChannel(id)
}

// HasPermission reports whether the invoking member has all the given permissions
// in the current channel.
func (c *Ctx) HasPermission(perm discord.Permissions) bool {
	if perm == 0 {
		return true
	}
	if c.IsDM() {
		return false
	}
	perms, ok := c.memberPermissions()
	if !ok {
		return false
	}
	return perms.Has(discord.PermissionAdministrator) || perms.Has(perm)
}

func (c *Ctx) memberPermissions() (discord.Permissions, bool) {
	if c.IsSlash() {
		if member := c.Interaction.Member(); member != nil {
			return member.Permissions, true
		}
		return 0, false
	}
	if c.Message == nil || c.Message.Member == nil {
		return 0, false
	}
	member := *c.Message.Member
	member.GuildID = c.GuildID
	member.User = c.Author

	// Threads use their parent channel's permission overwrites
	channelID := c.ChannelID
	if thread, ok := c.Client.Caches.GuildThread(channelID); ok {
		channelID = *thread.ParentID()
	}
	if ch, ok := c.Client.Caches.Channel(channelID); ok {
		return c.Client.Caches.MemberPermissionsInChannel(ch, member), true
	}
	return c.Client.Caches.MemberPermissions(member), true
}

// FindMessage returns the message a command should act on: the context-menu target,
// the message a prefix command replied to, or the most recent of the last `limit`
// channel messages matching `match`.
func (c *Ctx) FindMessage(limit int, match func(*discord.Message) bool) *discord.Message {
	if c.target != nil {
		return c.target
	}
	if c.Message != nil && c.Message.MessageReference != nil && c.Message.MessageReference.MessageID != nil {
		ref := c.Message.ReferencedMessage
		if ref == nil {
			ref, _ = c.Client.Rest.GetMessage(c.ChannelID, *c.Message.MessageReference.MessageID)
		}
		if ref != nil {
			return ref
		}
	}
	messages, err := c.Client.Rest.GetMessages(c.ChannelID, 0, 0, 0, limit)
	if err != nil {
		log.Default().Println("Error fetching channel history: " + err.Error())
		return nil
	}
	for i := range messages {
		m := &messages[i]
		if c.Message != nil && m.ID == c.Message.ID {
			continue
		}
		if match(m) {
			return m
		}
	}
	return nil
}
