package messagesutils

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

func BuildSaveEmbed(client *bot.Client, msg *discord.Message) discord.Embed {
	channelName := "Unknown Channel"
	var guildID snowflake.ID
	if msg.GuildID != nil {
		guildID = *msg.GuildID
	}
	if ch, err := command.FetchChannel(client, msg.ChannelID); err == nil {
		channelName = ch.Name()
		if gc, ok := ch.(discord.GuildChannel); ok && guildID == 0 {
			guildID = gc.GuildID()
		}
	}

	location := "Direct Messages"
	jumpGuild := "@me"
	if guildID != 0 {
		jumpGuild = guildID.String()
		location = "Unknown Guild"
		if guild, ok := client.Caches.Guild(guildID); ok {
			location = guild.Name
		} else if guild, err := client.Rest.GetGuild(guildID, false); err == nil {
			location = guild.Name
		}
	}

	embed := discord.Embed{
		Title: "Saved!",
		Fields: []discord.EmbedField{
			command.Field("From", fmt.Sprintf("@%s in #%s on %s", msg.Author.Username, channelName, location), false),
			command.Field("Location", fmt.Sprintf("[Jump to Message](https://discord.com/channels/%s/%s/%s)",
				jumpGuild, msg.ChannelID, msg.ID), false),
		},
	}

	if msg.Content != "" {
		embed.Fields = append(embed.Fields, command.Field("Content", msg.Content, false))
	}

	if images := saucefinder.GetImagesFromMessages(msg); len(images) > 0 {
		embed.Image = &discord.EmbedResource{URL: images[0]}
	}

	// List anything that can't be shown as the embed image
	var others []string
	for _, attachment := range msg.Attachments {
		if attachment.ContentType == nil || !strings.HasPrefix(*attachment.ContentType, "image/") {
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

func sendToDM(client *bot.Client, userID snowflake.ID, embed discord.Embed) error {
	dm, err := client.Rest.CreateDMChannel(userID)
	if err != nil {
		return err
	}
	_, err = client.Rest.CreateMessage(dm.ID(), discord.MessageCreate{Embeds: []discord.Embed{embed}})
	return err
}
