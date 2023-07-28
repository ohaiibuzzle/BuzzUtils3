package config

import (
	"encoding/json"
	"log"
	"os"
)

type configration struct {
	Token          string // Discord bot token
	BotPrefix      string // Bot prefix
	SauceNaoAPIKey string // SauceNao API key
}

var config *configration

func LoadConfig(configFile string) error {
	// Open the json config file
	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Decode the json config file
	config = &configration{}
	err = json.NewDecoder(file).Decode(config)

	log.Default().Println("Using prefix: " + config.BotPrefix)

	if err != nil {
		return err
	}
	return nil
}

func GetConfig() *configration {
	return config
}
