package bot

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/config"
	"github.com/ohaiibuzzle/BuzzUtils3/getimages"
	imageclassifier "github.com/ohaiibuzzle/BuzzUtils3/imageClassifier"
	"github.com/ohaiibuzzle/BuzzUtils3/saucefinder"
	"github.com/ohaiibuzzle/BuzzUtils3/utils"
	"golang.org/x/exp/slices"
)

func RegisterHandlers(s *discordgo.Session) {
	s.AddHandler(OnReadyHandler)
	s.AddHandler(OnMessageHandler)
}

func OnReadyHandler(s *discordgo.Session, event *discordgo.Ready) {
	s.UpdateGameStatus(0, "Hello, world!")
}

func OnMessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Prevent a loop of doom
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Prevent a multi-bot mess
	if m.Author.Bot {
		return
	}

	// Check prefix
	prefix := config.GetConfig().BotPrefix

	if m.Content[:len(prefix)] != prefix {
		return
	}

	// Split command
	command, args := splitCommand(m.Content[len(prefix):])
	// log.Default().Println("User " + m.Author.Username + " issued command: " + command)

	if slices.Contains(utils.Commands, command) {
		go utils.ProcessCommands(command, args, m, s)
		return
	}

	if slices.Contains(saucefinder.Commands, command) {
		go saucefinder.ProcessCommands(command, args, m, s)
		return
	}

	if slices.Contains(getimages.Commands, command) {
		go getimages.ProcessCommands(command, args, m, s)
		return
	}

	if slices.Contains(imageclassifier.Commands, command) {
		go imageclassifier.ProcessCommands(command, args, m, s)
		return
	}

	log.Default().Println("Unknown command: " + command)
}

func splitCommand(message string) (string, []string) {
	for i := 0; i < len(message); i++ {
		if message[i] == ' ' {
			return message[:i], splitArgs(message[i+1:])
		}
	}
	return message, []string{}
}

func splitArgs(message string) []string {
	var args []string
	var currentArg string
	for i := 0; i < len(message); i++ {
		if message[i] == ' ' {
			args = append(args, currentArg)
			currentArg = ""
		} else {
			currentArg += string(message[i])
		}
	}
	args = append(args, currentArg)
	return args
}
