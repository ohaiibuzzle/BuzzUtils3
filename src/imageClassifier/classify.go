package imageclassifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
	"github.com/ohaiibuzzle/BuzzUtils3/src/saucefinder"
)

type Predictions struct {
	Drawing float32 `json:"drawings"`
	Hentai  float32 `json:"hentai"`
	Neutral float32 `json:"neutral"`
	Porn    float32 `json:"porn"`
	Sexy    float32 `json:"sexy"`
}

func formatFloat(f float32) string {
	return strconv.FormatFloat(float64(f*100), 'f', 2, 32) + "%"
}

func PredictCommand(args []string, msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	var images []string
	var err error

	if msg.ReferencedMessage == nil {
		target, err := saucefinder.GetLastMessageWithAttachments(msg.ChannelID, ctx)
		if err != nil {
			log.Default().Println("Error getting last message with attachments: ", err)
			return
		}
		if target == nil {
			log.Default().Println("No message with attachments found")
			return
		}
		images, err = saucefinder.GetImagesFromMessages(target)
	} else {
		images, err = saucefinder.GetImagesFromMessages(msg.ReferencedMessage)
	}

	if err != nil {
		log.Default().Println("Error getting images: ", err)
		return
	}

	if len(images) == 0 {
		log.Default().Println("No images found")
		return
	}

	firstImage := images[0]

	predictChan := make(chan *Predictions)
	go func() {
		res, err := FromUrl(firstImage)
		if err != nil {
			log.Default().Println("Error predicting image: ", err)
			return
		}
		predictChan <- res
	}()

	predictions := <-predictChan

	embed := &discordgo.MessageEmbed{
		Title: "Image Classification Results",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Drawings",
				Value:  formatFloat(predictions.Drawing),
				Inline: true,
			},
			{
				Name:   "Hentai",
				Value:  formatFloat(predictions.Hentai),
				Inline: true,
			},
			{
				Name:   "Neutral",
				Value:  formatFloat(predictions.Neutral),
				Inline: true,
			},
			{
				Name:   "Porn",
				Value:  formatFloat(predictions.Porn),
				Inline: true,
			},
			{
				Name:   "Sexy",
				Value:  formatFloat(predictions.Sexy),
				Inline: true,
			},
		},
	}

	ctx.ChannelMessageSendEmbedReply(msg.ChannelID, embed, msg.Reference())
}

func FromUrl(url string) (*Predictions, error) {
	var inferenceServerURL = fmt.Sprintf("%s/classify", config.GetConfig().InferenceServer)

	client := &http.Client{}
	// Download the image
	imageData, err := client.Get(url)

	if err != nil {
		return nil, err
	}
	defer imageData.Body.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image.jpg")
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, imageData.Body)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", inferenceServerURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Deserialize the JSON response
	predictions := &Predictions{}
	err = json.NewDecoder(resp.Body).Decode(predictions)
	if err != nil {
		return nil, err
	}

	return predictions, nil
}
