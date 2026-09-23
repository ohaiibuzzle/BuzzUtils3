package messagesutils

import (
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func init() {
	command.Register(
		&command.Command{
			Name:        "savethis",
			Aliases:     []string{"save"},
			Description: "Save a message to your DMs (reply to one, or uses the latest with media)",
			Handler:     SaveThisCommand,
		},
		&command.Command{
			Name:        "saveall",
			Description: "Save every message with media among the last n messages to your DMs",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "amount",
					Description: "How many messages back to look (default 10, max 100)",
					MinValue:    &minSaveAll,
					MaxValue:    maxSaveAll,
				},
			},
			Handler: SaveAllCommand,
		},
		&command.Command{
			Name:        "oof",
			Aliases:     []string{"oofie"},
			Description: "Delete the bot's last message in this channel, in case things went wrong",
			Handler:     OofCommand,
		},
	)
	command.RegisterMessageActions(&command.MessageAction{
		Name: "Save to DMs",
		Handler: func(c *command.Ctx, target *discordgo.Message) {
			saveMessage(c, target)
		},
	})
}
