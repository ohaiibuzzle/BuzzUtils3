package saucefinder

import (
	"regexp"
	"strings"

	"github.com/GenDoNL/saucenao-go"
	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

var imageURLPattern = regexp.MustCompile(`https?://\S+?\.(?:png|jpg|jpeg|gif|webp)(?:\?\S*)?`)

func hasImageAttachment(message *discord.Message) []string {
	attachmentURLs := []string{}

	for _, attachment := range message.Attachments {
		if attachment.ContentType != nil && strings.HasPrefix(*attachment.ContentType, "image/") {
			attachmentURLs = append(attachmentURLs, attachment.URL)
		}
	}
	return attachmentURLs
}

func hasImageEmbed(message *discord.Message) []string {
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

func messageTextHasImageURL(message *discord.Message) []string {
	if message.Content == "" {
		return nil
	}
	return imageURLPattern.FindAllString(message.Content, -1)
}

// GetImagesFromMessages returns every image URL in a message's attachments, embeds and text.
func GetImagesFromMessages(message *discord.Message) []string {
	var images []string

	images = append(images, hasImageAttachment(message)...)
	images = append(images, hasImageEmbed(message)...)
	images = append(images, messageTextHasImageURL(message)...)

	return images
}

// MessageHasImages reports whether a message contains any image.
func MessageHasImages(message *discord.Message) bool {
	return len(GetImagesFromMessages(message)) > 0
}

// MessageHasMedia reports whether a message has any attachment or embed.
func MessageHasMedia(message *discord.Message) bool {
	return len(message.Attachments) > 0 || len(message.Embeds) > 0 || MessageHasImages(message)
}

// sauceTarget returns the first image of the message to look up, or replies with
// why there isn't one.
func sauceTarget(c *command.Ctx, target *discord.Message) (string, bool) {
	if target == nil {
		c.Reply("Please mention a message containing pasta!")
		return "", false
	}
	images := GetImagesFromMessages(target)
	if len(images) == 0 {
		c.Reply("That message doesn't have any images!")
		return "", false
	}
	return images[0], true
}

var saucenaoClient *saucenao.SaucenaoClient

func GetSauceNaoClient() *saucenao.SaucenaoClient {
	if saucenaoClient == nil {
		saucenaoClient = saucenao.New(config.GetConfig().SauceNaoAPIKey)
	}
	return saucenaoClient
}
