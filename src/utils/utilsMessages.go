package utils

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

var Commands = []string{
	"ping",
	"save",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "ping":
		go Ping(args, msg, ctx)
	case "save":
		go saveCommand(args, msg, ctx)
	default:
		log.Panic("Unknown command: " + command)
	}
}
