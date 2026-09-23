package fun

import (
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func userOption(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionUser,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func init() {
	command.Register(
		&command.Command{
			Name:        "owo",
			Description: "Yowouwuw wowowst nyghtmawe, in a cowommand.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "text",
					Description: "The text to owoify",
					Required:    true,
				},
			},
			Handler: OwoCommand,
		},
		&command.Command{
			Name:        "ship",
			Description: "Ships two people together 🛳️",
			Options: []*discordgo.ApplicationCommandOption{
				userOption("first", "The first person", true),
				userOption("second", "The second person", true),
			},
			GuildOnly: true,
			Handler:   ShipCommand,
		},
		&command.Command{
			Name:        "marry",
			Description: "Take your ship to the next level 💍",
			Options: []*discordgo.ApplicationCommandOption{
				userOption("user", "Who to propose to", true),
			},
			GuildOnly: true,
			Handler:   MarryCommand,
		},
		&command.Command{
			Name:        "divorce",
			Description: "End your marriage 💔",
			GuildOnly:   true,
			Handler:     DivorceCommand,
		},
		&command.Command{
			Name:        "marriagecert",
			Description: "Show a marriage certificate",
			Options: []*discordgo.ApplicationCommandOption{
				userOption("user", "Whose marriage to show (defaults to yours)", false),
			},
			GuildOnly: true,
			Handler:   MarriageCertCommand,
		},
	)
}
