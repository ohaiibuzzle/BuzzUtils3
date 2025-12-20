package getimages

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

type ZerochanDetailedResult struct {
	ID      int      `json:"id"`
	Small   string   `json:"small"`
	Medium  string   `json:"medium"`
	Large   string   `json:"large"`
	Full    string   `json:"full"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Size    int      `json:"size"`
	Hash    string   `json:"hash"`
	Source  string   `json:"source"`
	Primary string   `json:"primary"`
	Tags    []string `json:"tags"`
}

type ZerochanResult struct {
	ID        int      `json:"id"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Thumbnail string   `json:"thumbnail"`
	Source    string   `json:"source"`
	Tag       string   `json:"tag"`
	Tags      []string `json:"tags"`
}

type ZerochanResults struct {
	Items []ZerochanResult `json:"items"`
}

func Zerochan(msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	// Get the arguments
	args := strings.Split(msg.Content, " ")
	if len(args) < 2 {
		ctx.ChannelMessageSendReply(msg.ChannelID, "You need to specify a search term!", msg.Reference())
		return
	}

	// Join the arguments first
	searchTerm := strings.ReplaceAll(strings.Join(args[1:], " "), "+", ",")

	// Get the image
	embed, err := getZerochanImage(searchTerm)
	if err != nil {
		log.Default().Println("Error getting image: " + err.Error())
		return
	}

	// Send the image
	ctx.ChannelMessageSendEmbedReply(msg.ChannelID, embed, msg.Reference())
}

func getZerochanImage(searchTerm string) (*discordgo.MessageEmbed, error) {
	// Get the results
	results, err := getZerochanResult(searchTerm)
	if err != nil {
		log.Default().Println("Error getting results: " + err.Error())
		return nil, err
	}

	// Create the embed
	return makeZerochanEmbed(results, nil, nil), nil
}

func getZerochanResult(searchTerm string) (*ZerochanResult, error) {
	result, err := getZerochanResultPage(searchTerm)

	if err != nil {
		log.Default().Println("Error getting result: " + err.Error())
		return nil, err
	}

	return result, nil
}

func getZerochanResultPage(searchTerm string) (*ZerochanResult, error) {
	// https://www.zerochan.net/Keqing?page=1&limit=1&json

	req, err := http.NewRequest("GET", "https://www.zerochan.net/"+searchTerm+"?l=200&s=id&json", nil)
	if err != nil {
		log.Default().Println("Error making request: " + err.Error())
		return nil, err
	}
	req.Header.Set("User-Agent", config.GetConfig().UserAgent)

	jsonResp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Default().Println("Error getting json: " + err.Error())
		return nil, err
	}

	// Get the results
	decoder := json.NewDecoder(jsonResp.Body)
	var results ZerochanResults
	err = decoder.Decode(&results)

	totalImageCount := len(results.Items)
	if totalImageCount == 0 {
		return nil, nil
	}

	// Get a random index
	postIndex := rand.Intn(totalImageCount)
	indexInPage := postIndex % totalImageCount

	if err != nil {
		log.Default().Println("Error unmarshalling json: " + err.Error())
		return nil, err
	}

	return &results.Items[indexInPage], nil
}

func makeZerochanEmbed(result *ZerochanResult, msg *discordgo.MessageCreate, ctx *discordgo.Session) *discordgo.MessageEmbed {
	res, err := http.NewRequest("GET", "https://www.zerochan.net/"+strconv.Itoa(result.ID)+"?json", nil)
	if err != nil {
		log.Default().Println("Error creating request: " + err.Error())
		return nil
	}

	res.Header.Set("User-Agent", config.GetConfig().UserAgent)

	// Get the json
	jsonResp, err := http.DefaultClient.Do(res)
	if err != nil {
		log.Default().Println("Error getting json: " + err.Error())
		return nil
	}

	// Get the results
	decoder := json.NewDecoder(jsonResp.Body)
	var results ZerochanDetailedResult
	err = decoder.Decode(&results)

	if err != nil {
		log.Default().Println("Error unmarshalling json: " + err.Error())
		return nil
	}

	// Create the embed
	embed := &discordgo.MessageEmbed{
		Title: "Zerochan result",
		URL:   "https://www.zerochan.net/" + strconv.Itoa(result.ID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Source",
				Value: result.Source,
			},
			{
				Name:  "Tags",
				Value: "```\n" + strings.Join(result.Tags, ", ") + "\n```",
			},
		},
	}

	// Add the thumbnail
	embed.Image = &discordgo.MessageEmbedImage{
		URL: results.Large,
	}

	return embed
}
