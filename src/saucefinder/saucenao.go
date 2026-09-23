package saucefinder

import (
	"log"

	"github.com/GenDoNL/saucenao-go"
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func SauceplzCommand(c *command.Ctx) {
	c.Defer()
	sauceNaoMessage(c, c.FindMessage(10, MessageHasImages))
}

func sauceNaoMessage(c *command.Ctx, target *discordgo.Message) {
	if target == nil {
		c.Reply("Please mention a message containing pasta!")
		return
	}
	images := GetImagesFromMessages(target)
	if len(images) == 0 {
		c.Reply("That message doesn't have any images!")
		return
	}

	result, err := GetSauceNaoClient().FromURL(images[0])
	if err != nil {
		log.Default().Println("Error querying SauceNao: " + err.Error())
		c.Reply("SauceNAO is having a moment :( Try again later")
		return
	}
	if len(result.Data) == 0 {
		c.Reply("No sauce found")
		return
	}

	c.ReplyEmbed(createSauceNaoEmbed(result.Data[0]))
}

func createSauceNaoEmbed(result saucenao.SaucenaoResults) *discordgo.MessageEmbed {
	resultHeader := result.Header
	resultData := result.Data

	embed := &discordgo.MessageEmbed{
		Title: "Sauce found!",
		Fields: []*discordgo.MessageEmbedField{
			command.Field("Similarity", resultHeader.Similarity+"%", false),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: resultHeader.Thumbnail,
		},
	}

	if resultData.Title != "" {
		embed.Fields = append(embed.Fields, command.Field("Title", resultData.Title, false))
	}
	if resultData.MemberName != "" {
		embed.Fields = append(embed.Fields, command.Field("Author", resultData.MemberName, false))
	} else if resultData.Creator != "" {
		embed.Fields = append(embed.Fields, command.Field("Author", resultData.Creator, false))
	}
	if len(resultData.ExtUrls) > 0 {
		embed.URL = resultData.ExtUrls[0]
		embed.Fields = append(embed.Fields, command.Field("Source", resultData.ExtUrls[0], false))
	}
	embed.Fields = append(embed.Fields, command.Field("Index", resultHeader.IndexName, false))

	return embed
}
