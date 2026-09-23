package messagesutils

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

func BuildSaveEmbed(ctx *discordgo.Session, msg *discordgo.Message) *discordgo.MessageEmbed {
	originChannel, err := ctx.Channel(msg.ChannelID)
	if err != nil {
		originChannel = &discordgo.Channel{
			ID:   msg.ChannelID,
			Name: "Unknown Channel",
		}
	}

	guildID := msg.GuildID
	if guildID == "" {
		guildID = originChannel.GuildID
	}
	location := "Direct Messages"
	jumpGuild := "@me"
	if guildID != "" {
		jumpGuild = guildID
		if originGuild, err := ctx.Guild(guildID); err == nil {
			location = originGuild.Name
		} else {
			location = "Unknown Guild"
		}
	}

	embed := &discordgo.MessageEmbed{
		Title: "Saved!",
		Fields: []*discordgo.MessageEmbedField{
			command.Field("From", fmt.Sprintf("@%s in #%s on %s", msg.Author.Username, originChannel.Name, location), false),
			command.Field("Location", fmt.Sprintf("[Jump to Message](https://discord.com/channels/%s/%s/%s)",
				jumpGuild, msg.ChannelID, msg.ID), false),
		},
	}

	if msg.Content != "" {
		embed.Fields = append(embed.Fields, command.Field("Content", msg.Content, false))
	}

	if images := saucefinder.GetImagesFromMessages(msg); len(images) > 0 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: images[0],
		}
	}

	// List anything that can't be shown as the embed image
	var others []string
	for _, attachment := range msg.Attachments {
		if !strings.HasPrefix(attachment.ContentType, "image/") {
			others = append(others, attachment.URL)
		}
	}
	for _, e := range msg.Embeds {
		if e.Image == nil && e.URL != "" {
			others = append(others, e.URL)
		}
	}
	if len(others) > 0 {
		embed.Fields = append(embed.Fields, command.Field("Attached Content", strings.Join(others, "\n"), false))
	}

	return embed
}
