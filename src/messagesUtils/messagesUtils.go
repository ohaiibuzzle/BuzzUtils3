package messagesutils

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

var Commands = []string{
	"saveall",
	"savethis",
	"oof",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "saveall":
		go SaveAllCommand(args, msg, ctx)
	case "savethis":
		go SaveThisCommand(args, msg, ctx)
	case "oof":
		go OofCommand(args, msg, ctx)
	default:
		log.Panic("Unknown command: " + command)
	}
}
