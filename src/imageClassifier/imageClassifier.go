package imageclassifier

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

func init() {
	command.Register(&command.Command{
		Name:        "predict",
		Aliases:     []string{"police"},
		Description: "Ask Ai-chan to comment about an image (reply to one, or uses the latest)",
		Handler:     PredictCommand,
	})
	command.RegisterMessageActions(&command.MessageAction{
		Name: "Rate image",
		Handler: func(c *command.Ctx, target *discord.Message) {
			c.Defer()
			predictMessage(c, target)
		},
	})
}
