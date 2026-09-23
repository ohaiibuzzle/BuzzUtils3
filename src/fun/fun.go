package fun

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func userOption(name, description string, required bool) command.Option {
	return command.Option{
		Type:        discord.ApplicationCommandOptionTypeUser,
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
			Options: []command.Option{
				{
					Type:        discord.ApplicationCommandOptionTypeString,
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
			Options: []command.Option{
				userOption("first", "The first person", true),
				userOption("second", "The second person", true),
			},
			GuildOnly: true,
			Handler:   ShipCommand,
		},
		&command.Command{
			Name:        "marry",
			Description: "Take your ship to the next level 💍",
			Options: []command.Option{
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
			Options: []command.Option{
				userOption("user", "Whose marriage to show (defaults to yours)", false),
			},
			GuildOnly: true,
			Handler:   MarriageCertCommand,
		},
	)
}
