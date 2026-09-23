package saucefinder

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func init() {
	command.Register(
		&command.Command{
			Name:        "sauceplz",
			Description: "Search SauceNAO for an image's source (reply to one, or uses the latest)",
			Handler:     SauceplzCommand,
		},
		&command.Command{
			Name:        "iqdb",
			Description: "Search IQDB for an image's source (reply to one, or uses the latest)",
			Handler:     IqdbCommand,
		},
	)
	command.RegisterMessageActions(
		&command.MessageAction{
			Name: "Find sauce (SauceNAO)",
			Handler: func(c *command.Ctx, target *discord.Message) {
				c.Defer()
				sauceNaoMessage(c, target)
			},
		},
		&command.MessageAction{
			Name: "Find sauce (IQDB)",
			Handler: func(c *command.Ctx, target *discord.Message) {
				c.Defer()
				iqdbMessage(c, target)
			},
		},
	)
}
