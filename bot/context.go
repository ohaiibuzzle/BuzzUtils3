package bot

import (
	"log"
	"math/rand"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/config"
	imageclassifier "github.com/ohaiibuzzle/BuzzUtils3/imageClassifier"
)

var botContext *discordgo.Session

func InitContext() error {
	var err error
	botContext, err = discordgo.New("Bot " + config.GetConfig().Token)
	botContext.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentsGuildMembers | discordgo.IntentMessageContent

	// Seed the rng
	rand.Seed(time.Now().UnixNano())
	imageclassifier.InitializeModel()

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
