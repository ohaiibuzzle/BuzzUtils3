package fun

import "github.com/bwmarrin/discordgo"

var Commands = []string{
	"owo",
}

func ProcessCommands(command string, args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	switch command {
	case "owo":
		go OwoCommand(args, msg, ctx)
	}
}
