package messagesutils

import "github.com/ohaiibuzzle/BuzzUtils3/src/command"

func OofCommand(c *command.Ctx) {
	c.DeleteInvocation()

	// Delete the bot's last message in the channel
	messages, err := c.Session.ChannelMessages(c.ChannelID, 10, "", "", "")
	if err != nil {
		c.ReplyPrivate("Failed to fetch messages.")
		return
	}

	for _, message := range messages {
		if message.Author.ID == c.Session.State.User.ID {
			if err := c.Session.ChannelMessageDelete(c.ChannelID, message.ID); err != nil {
				c.ReplyPrivate("Failed to delete my last message.")
				return
			}
			c.Ack("Deleted my last message.")
			return
		}
	}
	c.ReplyPrivate("I couldn't find a recent message of mine.")
}
