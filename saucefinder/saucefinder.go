package saucefinder

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

var Commands = []string{
	"sauceplz",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "sauceplz":
		go SauceplzCommand(args, msg, ctx)
	default:
		log.Panic("Unknown command: " + command)
	}
}
