package messagesutils

import (
	"github.com/disgoorg/snowflake/v2"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

var minSaveAll, maxSaveAll = 1, 100

func SaveAllCommand(c *command.Ctx) {
	numMessages := 10
	if n, ok := c.Int("amount"); ok {
		if n < int64(minSaveAll) || n > int64(maxSaveAll) {
			c.ReplyPrivate("You can only look between 1 and 100 messages back!")
			return
		}
		numMessages = int(n)
	}

	c.Defer()
	var before snowflake.ID
	if c.Message != nil {
		before = c.Message.ID
	}
	messages, err := c.Client.Rest.GetMessages(c.ChannelID, 0, before, 0, numMessages)
	if err != nil {
		c.ReplyPrivate("Failed to fetch messages.")
		return
	}

	saved := 0
	for i := range messages {
		message := &messages[i]
		if !saucefinder.MessageHasMedia(message) {
			continue
		}
		if err := sendToDM(c.Client, c.Author.ID, BuildSaveEmbed(c.Client, message)); err != nil {
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
