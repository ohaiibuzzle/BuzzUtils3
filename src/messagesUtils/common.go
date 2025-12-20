package messagesutils

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

func BuildSaveEmbed(ctx *discordgo.Session, msg *discordgo.Message) *discordgo.MessageEmbed {

	originChannel, err := ctx.Channel(msg.ChannelID)
	if err != nil {
		originChannel = &discordgo.Channel{
			Name: "Unknown Channel",
		}
	}

	originGuild, err := ctx.Guild(originChannel.GuildID)
	if err != nil {
		originGuild = &discordgo.Guild{
			Name: "Unknown Guild",
		}
	}

	embedFields := []*discordgo.MessageEmbedField{
		{
			Name:   "From",
			Value:  fmt.Sprintf("@%s in %s on %s", msg.Author.Username, originChannel.Name, originGuild.Name),
			Inline: false,
		},
		{
			Name: "Location",
			Value: fmt.Sprintf("[Jump to Message](https://discord.com/channels/%s/%s/%s)",
				originGuild.ID, originChannel.ID, msg.ID),
			Inline: false,
		},
	}

	embed := &discordgo.MessageEmbed{
		Title:  "Saved!",
		Fields: embedFields,
	}

	attachedMedia, err := saucefinder.GetImagesFromMessages(msg)
	if err == nil && len(attachedMedia) > 0 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: attachedMedia[0],
		}
	}

	return embed
}
