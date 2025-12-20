package messagesutils

import "github.com/bwmarrin/discordgo"

func SaveThisCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	// Check if this message has a referenced message
	referencedMessage, err := ctx.ChannelMessage(msg.ChannelID, msg.MessageReference.MessageID)

	if err != nil {
		// If there is no referenced message, inform the user
		ctx.ChannelMessageSendReply(msg.ChannelID, "Please reply to a message containing media to save it.", msg.Reference())
		return
	}

	// Build the embed for the referenced message
	embed := BuildSaveEmbed(ctx, referencedMessage)

	// Send the embed to the user's DM
	dmChannel, err := ctx.UserChannelCreate(msg.Author.ID)
	if err != nil {
		ctx.ChannelMessageSendReply(msg.ChannelID, "Failed to create DM channel.", msg.Reference())
		return
	}

	_, err = ctx.ChannelMessageSendEmbed(dmChannel.ID, embed)
	if err != nil {
		ctx.ChannelMessageSendReply(msg.ChannelID, "Failed to send DM.", msg.Reference())
		return
	}

	// Acknowledge the command in the original channel
	ctx.ChannelMessageSendReply(msg.ChannelID, "I've sent you a DM with the saved media!", msg.Reference())
}
