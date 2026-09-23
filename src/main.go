package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ohaiibuzzle/BuzzUtils3/src/bot"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

func main() {
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
