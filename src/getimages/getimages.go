package getimages

import "github.com/bwmarrin/discordgo"

var Commands = []string{
	"zerochan",
	"safebooru",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "zerochan":
		Zerochan(msg, ctx)
	case "safebooru":
		Safebooru(msg, ctx)
	}
}
