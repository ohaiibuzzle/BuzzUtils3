package utils

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func saveCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	// Check if there is any message referenced by the command
	referencedMessage, err := ctx.ChannelMessage(msg.ChannelID, msg.MessageReference.MessageID)

	if err != nil {
		// Fall back to the last message in the channel
		messages, err := ctx.ChannelMessages(msg.ChannelID, 1, "", "", "")
		if err != nil {
			log.Default().Println("Error getting messages: " + err.Error())
			return
		}
		referencedMessage = messages[0]
	}

	// Save the message
	saveMessageToUserDM(referencedMessage, msg.Author, ctx)
}

func saveMessageToUserDM(msg *discordgo.Message, author *discordgo.User, ctx *discordgo.Session) {
	// Get the user's DM channel
	channel, err := ctx.UserChannelCreate(author.ID)

	if err != nil {
		log.Default().Println("Error getting user DM channel: " + err.Error())
		return
	}

	// Create the embed
	embed, err := createSaveEmbed(msg, ctx)
	if err != nil {
		log.Default().Println("Error creating embed: " + err.Error())
		return
	}

	// Send the embed
	ctx.ChannelMessageSendEmbed(channel.ID, embed)
}

func createSaveEmbed(msg *discordgo.Message, ctx *discordgo.Session) (*discordgo.MessageEmbed, error) {
	// Create the embed
	originChannel, err := ctx.Channel(msg.ChannelID)
	if err != nil {
		log.Default().Println("Error getting channel: " + err.Error())
		return nil, err
	}

	originGuild, err := ctx.Guild(originChannel.GuildID)
	if err != nil {
		log.Default().Println("Error getting guild: " + err.Error())
		return nil, err
	}

	originChannelName := originChannel.Name
	originGuildName := originGuild.Name

	attachedOrEmbedURL := ""

	for _, attachment := range msg.Attachments {
		if attachment.ContentType == "image/png" || attachment.ContentType == "image/jpeg" {
			attachedOrEmbedURL = attachment.URL
		}
	}

	for _, embed := range msg.Embeds {
		if embed.Image != nil {
			attachedOrEmbedURL = embed.Image.URL
		}
	}

	embed := &discordgo.MessageEmbed{
		Title: "Saved message",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "From",
				Value: fmt.Sprintf("@ %s in %s on %s", msg.Author.Username, originChannelName, originGuildName),
			},
			{
				Name:  "Jump to message",
				Value: fmt.Sprintf("https://discord.com/channels/%s/%s/%s", originGuild.ID, originChannel.ID, msg.ID),
			},
			{
				Name:  "Message content",
				Value: msg.Content,
			},
		},
	}

	if attachedOrEmbedURL != "" {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: attachedOrEmbedURL,
		}
	}

	return embed, nil
}
