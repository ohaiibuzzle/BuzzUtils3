package saucefinder

import (
	"github.com/GenDoNL/saucenao-go"
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/config"
)

func GetImagesFromMessages(message *discordgo.Message) ([]string, error) {
	var images []string

	for _, attachment := range message.Attachments {
		if attachment.ContentType == "image/png" || attachment.ContentType == "image/jpeg" {
			images = append(images, attachment.URL)
		}
	}

	for _, embed := range message.Embeds {
		if embed.Image != nil {
			images = append(images, embed.Image.URL)
		}
	}

	return images, nil
}

var saucenaoClient *saucenao.SaucenaoClient

func GetSauceNaoClient() *saucenao.SaucenaoClient {
	if saucenaoClient == nil {
		saucenaoClient = saucenao.New(config.GetConfig().SauceNaoAPIKey)
	}
	return saucenaoClient
}
