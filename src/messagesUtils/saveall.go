package messagesutils

import (
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

var (
	minSaveAll float64 = 1
	maxSaveAll float64 = 100
)

func SaveAllCommand(c *command.Ctx) {
	numMessages := 10
	if n, ok := c.Int("amount"); ok {
		if n < 1 || n > int64(maxSaveAll) {
			c.ReplyPrivate("You can only look between 1 and 100 messages back!")
			return
		}
		numMessages = int(n)
	}

	c.Defer()
	before := ""
	if c.Message != nil {
		before = c.Message.ID
	}
	messages, err := c.Session.ChannelMessages(c.ChannelID, numMessages, before, "", "")
	if err != nil {
		c.ReplyPrivate("Failed to fetch messages.")
		return
	}

	saved := 0
	for _, message := range messages {
		if !saucefinder.MessageHasMedia(message) {
			continue
		}
		if err := sendToDM(c.Session, c.Author.ID, BuildSaveEmbed(c.Session, message)); err != nil {
			c.ReplyPrivate("I couldn't DM you. Do you have DMs from server members turned off?")
			return
		}
		saved++
	}

	if saved == 0 {
		c.ReplyPrivate("I couldn't find anything to save :(")
		return
	}
	c.ReplyPrivate("I've sent you a DM with the saved media!")
}
