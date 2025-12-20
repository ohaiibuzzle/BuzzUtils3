package getimages

import "github.com/bwmarrin/discordgo"

var Commands = []string{
	"zerochan",
	"safebooru",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "zerochan":
		go Zerochan(msg, ctx)
	case "safebooru":
		go Safebooru(msg, ctx)
	}
}
