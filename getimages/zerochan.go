package getimages

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
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
		ctx.ChannelMessageSend(msg.ChannelID, "You need to specify a search term!")
		return
	}

	// Get the search term
	searchTerm := strings.Join(args[1:], " ")

	// Get the image
	embed, err := getZerochanImage(searchTerm)
	if err != nil {
		log.Default().Println("Error getting image: " + err.Error())
		return
	}

	// Send the image
	ctx.ChannelMessageSendEmbed(msg.ChannelID, embed)
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
	// https://www.zerochan.net/Keqing?page=1&limit=1&json

	// Get the image count
	imageCount, err := getZerochanImageCount(searchTerm)
	if err != nil {
		log.Default().Println("Error getting image count: " + err.Error())
		return nil, err
	}

	// Select a random image
	imageIndex := rand.Intn(imageCount)

	// Get the result
	result, err := getZerochanResultPage(searchTerm, imageIndex)

	if err != nil {
		log.Default().Println("Error getting result: " + err.Error())
		return nil, err
	}

	return result, nil
}

func getZerochanResultPage(searchTerm string, imageIndex int) (*ZerochanResult, error) {
	// https://www.zerochan.net/Keqing?page=1&limit=1&json

	// Math
	page := imageIndex / 100
	indexInPage := imageIndex % 100

	req, err := http.NewRequest("GET", "https://www.zerochan.net/"+searchTerm+"?page="+strconv.Itoa(page)+"&l=100&json", nil)
	if err != nil {
		log.Default().Println("Error making request: " + err.Error())
		return nil, err
	}
	req.Header.Set("User-Agent", "Firefox/110.0 (Windows NT 10.0; Win64; x64)")

	jsonResp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Default().Println("Error getting json: " + err.Error())
		return nil, err
	}

	// Get the results
	decoder := json.NewDecoder(jsonResp.Body)
	var results ZerochanResults
	err = decoder.Decode(&results)

	if err != nil {
		log.Default().Println("Error unmarshalling json: " + err.Error())
		return nil, err
	}

	return &results.Items[indexInPage], nil
}

func getZerochanImageCount(searchTerm string) (int, error) {
	// Unfortunately we have to read the old xml api to get the image count
	// (Zerochan has <number> <tag> anime images, wallpapers, fanart, and many more in its gallery.)
	// https://www.zerochan.net/Keqing?xml

	req, err := http.NewRequest("GET", "https://www.zerochan.net/"+searchTerm+"?xml", nil)
	if err != nil {
		log.Default().Println("Error creating request: " + err.Error())
		return 0, err
	}
	req.Header.Set("User-Agent", "Firefox/110.0 (Windows NT 10.0; Win64; x64)")

	// Get the xml
	xmlResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Default().Println("Error getting xml: " + err.Error())
		return 0, err
	}

	xmlContent, err := io.ReadAll(xmlResponse.Body)
	if err != nil {
		log.Default().Println("Error reading xml: " + err.Error())
		return 0, err
	}

	// regex to get the image count (number format 2,429,947)
	descriptionRegex := regexp.MustCompile(`<description>([\s\S]*)<\/description>`)
	imageCountRegex := regexp.MustCompile(`[(\d{0,3}),]+`)

	// Get the description
	description := descriptionRegex.FindStringSubmatch(string(xmlContent))
	if len(description) < 1 {
		log.Default().Println("Error finding description")
		return 0, errors.New("error finding description")
	}

	// Get the image count
	imageCount := imageCountRegex.FindStringSubmatch(description[0])

	// Convert the image count to an int
	// First remove the commas
	imageCountString := strings.ReplaceAll(imageCount[0], ",", "")
	// Then convert to an int
	imageCountInt, err := strconv.Atoi(imageCountString)
	if err != nil {
		log.Default().Println("Error converting image count to int: " + err.Error())
		return 0, err
	}

	// Set an upper limit (else the API explodes) of 1000
	if imageCountInt > 1000 {
		imageCountInt = 1000
	}

	return imageCountInt, nil
}

func makeZerochanEmbed(result *ZerochanResult, msg *discordgo.MessageCreate, ctx *discordgo.Session) *discordgo.MessageEmbed {
	res, err := http.NewRequest("GET", "https://www.zerochan.net/"+strconv.Itoa(result.ID)+"?json", nil)
	if err != nil {
		log.Default().Println("Error creating request: " + err.Error())
		return nil
	}

	res.Header.Set("User-Agent", "Firefox/110.0 (Windows NT 10.0; Win64; x64)")

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
