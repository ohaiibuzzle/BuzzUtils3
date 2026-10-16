package bot

import (
	"context"
	"log"
	"net/http"

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

	// For `buzzutils3 healthcheck`: healthy while the gateway is connected to Discord
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if botContext.Gateway.Status() != gateway.StatusReady {
			http.Error(w, "gateway not ready", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
	go func() {
		log.Println("Health server stopped: ", http.ListenAndServe(HealthAddr, nil))
	}()
}

// HealthAddr is loopback only, so the health endpoint isn't reachable from outside the container
const HealthAddr = "127.0.0.1:8080"

func Close() {
	botContext.Close(context.Background())
}
