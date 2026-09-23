package messagesutils

import (
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

const soICanSave = ". so I can save"

func SaveThisCommand(c *command.Ctx) {
	saveMessage(c, c.FindMessage(20, saucefinder.MessageHasMedia))
}

func saveMessage(c *command.Ctx, target *discord.Message) {
	if target == nil {
		c.ReplyPrivate("Please reply to a message containing media to save it.")
		return
	}

	if err := sendToDM(c.Client, c.Author.ID, BuildSaveEmbed(c.Client, target)); err != nil {
		c.ReplyPrivate("I couldn't DM you. Do you have DMs from server members turned off?")
		return
	}
	c.ReplyPrivate("I've sent you a DM with the saved message!")
}

// HandleSaveShortcut saves the latest message with media when someone says
// ". so I can save", like the old bot did. It returns true if the message was handled.
func HandleSaveShortcut(e *events.MessageCreate) bool {
	if !strings.HasPrefix(e.Message.Content, soICanSave) {
		return false
	}
	client := e.Client()
	messages, err := client.Rest.GetMessages(e.ChannelID, 0, e.MessageID, 0, 10)
	if err != nil {
		return true
	}
	for i := range messages {
		if saucefinder.MessageHasMedia(&messages[i]) {
			sendToDM(client, e.Message.Author.ID, BuildSaveEmbed(client, &messages[i]))
			break
		}
	}
	return true
}
