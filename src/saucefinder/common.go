package saucefinder

import (
	"regexp"
	"strings"

	"github.com/GenDoNL/saucenao-go"
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

var imageURLPattern = regexp.MustCompile(`https?://\S+?\.(?:png|jpg|jpeg|gif|webp)(?:\?\S*)?`)

func hasImageAttachment(message *discordgo.Message) []string {
	attachmentURLs := []string{}

	for _, attachment := range message.Attachments {
		if strings.HasPrefix(attachment.ContentType, "image/") {
			attachmentURLs = append(attachmentURLs, attachment.URL)
		}
	}
	return attachmentURLs
}

func hasImageEmbed(message *discordgo.Message) []string {
	embedURLs := []string{}

	for _, embed := range message.Embeds {
		if embed.Image != nil && embed.Image.URL != "" {
			embedURLs = append(embedURLs, embed.Image.URL)
		}
		if embed.Thumbnail != nil && embed.Thumbnail.URL != "" {
			embedURLs = append(embedURLs, embed.Thumbnail.URL)
		}
	}
	return embedURLs
}

func messageTextHasImageURL(message *discordgo.Message) []string {
	if message.Content == "" {
		return nil
	}
	return imageURLPattern.FindAllString(message.Content, -1)
}

// GetImagesFromMessages returns every image URL in a message's attachments, embeds and text.
func GetImagesFromMessages(message *discordgo.Message) []string {
	var images []string

	images = append(images, hasImageAttachment(message)...)
	images = append(images, hasImageEmbed(message)...)
	images = append(images, messageTextHasImageURL(message)...)

	return images
}

// MessageHasImages reports whether a message contains any image.
func MessageHasImages(message *discordgo.Message) bool {
	return len(GetImagesFromMessages(message)) > 0
}

// MessageHasMedia reports whether a message has any attachment or embed.
func MessageHasMedia(message *discordgo.Message) bool {
	return len(message.Attachments) > 0 || len(message.Embeds) > 0 || MessageHasImages(message)
}

var saucenaoClient *saucenao.SaucenaoClient

func GetSauceNaoClient() *saucenao.SaucenaoClient {
	if saucenaoClient == nil {
		saucenaoClient = saucenao.New(config.GetConfig().SauceNaoAPIKey)
	}
	return saucenaoClient
}
