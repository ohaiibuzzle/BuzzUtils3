package messagesutils

import "github.com/bwmarrin/discordgo"

func OofCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	// Delete the bot's last message in the channel
	messages, err := ctx.ChannelMessages(msg.ChannelID, 10, "", "", "")
	if err != nil {
		ctx.ChannelMessageSendReply(msg.ChannelID, "Failed to fetch messages.", msg.Reference())
		return
	}

	for _, message := range messages {
		if message.Author.ID == ctx.State.User.ID {
			err := ctx.ChannelMessageDelete(msg.ChannelID, message.ID)
			if err != nil {
				ctx.ChannelMessageSendReply(msg.ChannelID, "Failed to delete my last message.", msg.Reference())
			}
			return
		}
	}
}
