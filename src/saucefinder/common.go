package saucefinder

import (
	"regexp"

	"github.com/GenDoNL/saucenao-go"
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

func hasImageAttachment(message *discordgo.Message) ([]string, error) {
	attachmentURLs := []string{}

	if message.Attachments != nil {
		for _, attachment := range message.Attachments {
			if attachment.ContentType == "image/png" || attachment.ContentType == "image/jpeg" {
				attachmentURLs = append(attachmentURLs, attachment.URL)
			}
		}
	}
	return attachmentURLs, nil
}

func hasImageEmbed(message *discordgo.Message) ([]string, error) {
	embedURLs := []string{}

	if message.Embeds != nil {
		for _, embed := range message.Embeds {
			if embed.Image != nil {
				embedURLs = append(embedURLs, embed.Image.URL)
			}
		}
	}
	return embedURLs, nil
}

func messageTextHasImageURL(message *discordgo.Message) ([]string, error) {
	// Simple check for image URLs in the message content
	if message.Content != "" {
		pattern := `(https?://[^\s]+(\.png|\.jpg|\.jpeg|\.gif))`
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(message.Content, -1)
		if len(matches) > 0 {
			return matches, nil
		}
	}
	return nil, nil
}

func GetLastMessageWithAttachments(channelID string, ctx *discordgo.Session) (*discordgo.Message, error) {
	messages, err := ctx.ChannelMessages(channelID, 10, "", "", "")
	if err != nil {
		return nil, err
	}

	for _, message := range messages {
		attachments, err := hasImageAttachment(message)
		if err != nil {
			return nil, err
		}
		if len(attachments) > 0 {
			return message, nil
		}

		embeds, err := hasImageEmbed(message)
		if err != nil {
			return nil, err
		}
		if len(embeds) > 0 {
			return message, nil
		}

		urls, err := messageTextHasImageURL(message)
		if err != nil {
			return nil, err
		}
		if len(urls) > 0 {
			return message, nil
		}
	}

	return nil, nil
}

func GetImagesFromMessages(message *discordgo.Message) ([]string, error) {
	var images []string

	attachmentURLs, err := hasImageAttachment(message)
	if err != nil {
		return nil, err
	}
	images = append(images, attachmentURLs...)

	embedURLs, err := hasImageEmbed(message)
	if err != nil {
		return nil, err
	}
	images = append(images, embedURLs...)

	textURLs, err := messageTextHasImageURL(message)
	if err != nil {
		return nil, err
	}
	images = append(images, textURLs...)

	return images, nil
}

var saucenaoClient *saucenao.SaucenaoClient

func GetSauceNaoClient() *saucenao.SaucenaoClient {
	if saucenaoClient == nil {
		saucenaoClient = saucenao.New(config.GetConfig().SauceNaoAPIKey)
	}
	return saucenaoClient
}
