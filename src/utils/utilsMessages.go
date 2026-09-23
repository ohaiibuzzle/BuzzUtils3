package utils

import (
	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func init() {
	command.Register(
		&command.Command{
			Name:        "ping",
			Description: "Check if the bot is alive, and its latency",
			Handler:     Ping,
		},
		&command.Command{
			Name:        "help",
			Description: "List the available commands",
			Handler:     Help,
		},
		&command.Command{
			Name:        "sudo",
			Description: "Make the bot say something",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "text",
					Description: "What to say",
					Required:    true,
				},
			},
			OwnerOnly: true,
			Handler:   Sudo,
		},
		&command.Command{
			Name:        "leaveserver",
			Description: "Make the bot leave this server",
			GuildOnly:   true,
			OwnerOnly:   true,
			Handler:     LeaveServer,
		},
	)
}
