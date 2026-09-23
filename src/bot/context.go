package bot

import (
	"context"
	"log"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/godave/golibdave"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

var botContext *bot.Client

func InitContext() error {
	var err error
	botContext, err = disgo.New(config.GetConfig().Token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentsNonPrivileged, gateway.IntentGuildMembers, gateway.IntentMessageContent),
			gateway.WithPresenceOpts(gateway.WithPlayingActivity("in Buzzle's Box. Available on GitHub")),
		),
		// Only what the commands look up: guild names, channel NSFW flags and
		// permission overwrites, roles for permission checks, and who is in which
		// voice channel for music
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds, cache.FlagChannels, cache.FlagRoles, cache.FlagVoiceStates)),
		// Discord requires end-to-end encrypted voice (DAVE), done by libdave
		bot.WithVoiceManagerConfigOpts(voice.WithDaveSessionCreateFunc(golibdave.NewSession)),
		// Run each event in its own goroutine (like discordgo), so slow handlers such
		// as the welcome card don't hold up the gateway
		bot.WithEventManagerConfigOpts(bot.WithAsyncEventsEnabled()),
	)
	return err
}

func GetContext() *bot.Client {
	return botContext
}

func Start() {
	if err := botContext.OpenGateway(context.Background()); err != nil {
		log.Fatal("Error opening connection: ", err)
	}
}

func Close() {
	botContext.Close(context.Background())
}
