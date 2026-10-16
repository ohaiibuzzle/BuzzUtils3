package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ohaiibuzzle/BuzzUtils3/src/bot"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

func main() {
	// The runtime image has no curl, so the Docker healthcheck runs the binary itself
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		resp, err := http.Get("http://" + bot.HealthAddr + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}

	if err := config.LoadConfig("runtime/config.yaml"); err != nil {
		log.Fatal("Error loading runtime/config.yaml: ", err)
	}

	if err := bot.InitContext(); err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}
	bot.RegisterHandlers(bot.GetContext())
	bot.Start()

	log.Default().Println("Bot is now running. Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	bot.Close()
}
