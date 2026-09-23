package bot

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

var botContext *discordgo.Session

func InitContext() error {
	var err error
	botContext, err = discordgo.New("Bot " + config.GetConfig().Token)
	botContext.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentsGuildMembers | discordgo.IntentMessageContent | discordgo.IntentGuilds

	return err
}

func GetContext() *discordgo.Session {
	return botContext
}

func Start() {
	if err := botContext.Open(); err != nil {
		log.Fatal("Error opening connection: ", err)
		return
	}
}
