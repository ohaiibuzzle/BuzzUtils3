package config

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type configration struct {
	Token           string `yaml:"token"`               // Bot token
	BotPrefix       string `yaml:"bot_prefix"`          // Bot prefix
	SauceNaoAPIKey  string `yaml:"saucenao_api_key"`    // SauceNao API key
	UserAgent       string `yaml:"user_agent"`          // User agent for HTTP requests
	InferenceServer string `yaml:"inference_server"`    // Inference server URL
	PixivToken      string `yaml:"pixiv_refresh_token"` // Pixiv OAuth refresh token
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
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		return err
	}

	log.Default().Println("Using prefix: " + config.BotPrefix)
	return nil
}

func GetConfig() *configration {
	return config
}
