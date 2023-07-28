package saucefinder

import (
	"log"

	"github.com/GenDoNL/saucenao-go"
	"github.com/bwmarrin/discordgo"
)

func SauceplzCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
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

	// Query SauceNao
	embed, err := querySauceNao(referencedMessage, ctx)

	if err != nil {
		log.Default().Println("Error querying SauceNao: " + err.Error())
		return
	}

	// Send the embed
	ctx.ChannelMessageSendEmbed(msg.ChannelID, embed)
}

func querySauceNao(msg *discordgo.Message, ctx *discordgo.Session) (*discordgo.MessageEmbed, error) {
	images, err := GetImagesFromMessages(msg)

	if err != nil {
		log.Default().Println("Error getting images: " + err.Error())
		return nil, err
	}

	result, err := GetSauceNaoClient().FromURL(images[0])
	if err != nil {
		log.Default().Println("Error querying SauceNao: " + err.Error())
		return nil, err
	}

	embed := createSauceNaoEmbed(result.Data[0], msg, ctx)

	return embed, nil
}

func createSauceNaoEmbed(result saucenao.SaucenaoResults, msg *discordgo.Message, ctx *discordgo.Session) *discordgo.MessageEmbed {
	resultHeader := result.Header
	resultData := result.Data

	// Create the embed
	embed := &discordgo.MessageEmbed{
		Title: "SauceNao result",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Title",
				Value: resultData.Title,
			},
			{
				Name:  "Index",
				Value: resultHeader.IndexName,
			},
			{
				Name:  "Similarity",
				Value: resultHeader.Similarity,
			},
			{
				Name:  "URL",
				Value: resultData.ExtUrls[0],
			},
		},
	}

	// Add the thumbnail
	embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: resultHeader.Thumbnail,
	}

	return embed
}
