package messagesutils

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

const soICanSave = ". so I can save"

func SaveThisCommand(c *command.Ctx) {
	saveMessage(c, c.FindMessage(20, saucefinder.MessageHasMedia))
}

func saveMessage(c *command.Ctx, target *discordgo.Message) {
	if target == nil {
		c.ReplyPrivate("Please reply to a message containing media to save it.")
		return
	}

	if err := sendToDM(c.Session, c.Author.ID, BuildSaveEmbed(c.Session, target)); err != nil {
		c.ReplyPrivate("I couldn't DM you. Do you have DMs from server members turned off?")
		return
	}
	c.ReplyPrivate("I've sent you a DM with the saved message!")
}

// HandleSaveShortcut saves the latest message with media when someone says
// ". so I can save", like the old bot did. It returns true if the message was handled.
func HandleSaveShortcut(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if !strings.HasPrefix(m.Content, soICanSave) {
		return false
	}
	messages, err := s.ChannelMessages(m.ChannelID, 10, m.ID, "", "")
	if err != nil {
		return true
	}
	for _, message := range messages {
		if saucefinder.MessageHasMedia(message) {
			sendToDM(s, m.Author.ID, BuildSaveEmbed(s, message))
			break
		}
	}
	return true
}

func sendToDM(s *discordgo.Session, userID string, embed *discordgo.MessageEmbed) error {
	dmChannel, err := s.UserChannelCreate(userID)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSendEmbed(dmChannel.ID, embed)
	return err
}
