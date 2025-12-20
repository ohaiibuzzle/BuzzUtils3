package messagesutils

import (
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

func SaveAllCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	// Get the number of messages to scan from args
	numMessages := 10 // Default value
	if len(args) > 0 {
		parsedNum, err := strconv.Atoi(args[0])
		if err == nil && parsedNum > 0 && parsedNum <= 100 {
			numMessages = parsedNum
		}
	}

	// Fetch messages from the channel
	messages, err := ctx.ChannelMessages(msg.ChannelID, numMessages, "", "", "")
	if err != nil {
		ctx.ChannelMessageSendReply(msg.ChannelID, "Failed to fetch messages.", msg.Reference())
		return
	}

	for _, message := range messages {
		messageHasEmbed, err := saucefinder.GetImagesFromMessages(message)
		if err != nil || len(messageHasEmbed) == 0 {
			continue
		}

		// Build the embed for the message
		embed := BuildSaveEmbed(ctx, message)

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
	}

	// Acknowledge the command in the original channel
	ctx.ChannelMessageSendReply(msg.ChannelID, "I've sent you a DM with the saved media!", msg.Reference())
}
