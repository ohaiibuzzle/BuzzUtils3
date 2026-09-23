package imageclassifier

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
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

// nsfwThreshold is the score above which hentai/porn/sexy content is filtered out.
const nsfwThreshold = 0.5

var httpClient = &http.Client{Timeout: 30 * time.Second}

var ErrNoInferenceServer = errors.New("no inference server configured")

func formatFloat(f float32) string {
	return strconv.FormatFloat(float64(f*100), 'f', 2, 32) + "%"
}

func PredictCommand(c *command.Ctx) {
	c.Defer()
	target := c.FindMessage(10, saucefinder.MessageHasImages)
	predictMessage(c, target)
}

func predictMessage(c *command.Ctx, target *discord.Message) {
	if target == nil {
		c.Reply("Hey, at least give me something to work with!")
		return
	}
	images := saucefinder.GetImagesFromMessages(target)
	if len(images) == 0 {
		c.Reply("Hey, that is not an image")
		return
	}

	predictions, err := FromUrl(images[0])
	if err != nil {
		log.Default().Println("Error predicting image: ", err)
		c.Reply("Ai-chan couldn't look at that image right now :(")
		return
	}

	embed := discord.Embed{
		Title:     "Image Classification Results",
		Color:     topCategoryColor(predictions),
		Thumbnail: &discord.EmbedResource{URL: images[0]},
		Fields: []discord.EmbedField{
			command.Field("Drawings", formatFloat(predictions.Drawing), true),
			command.Field("Hentai", formatFloat(predictions.Hentai), true),
			command.Field("Neutral", formatFloat(predictions.Neutral), true),
			command.Field("Porn", formatFloat(predictions.Porn), true),
			command.Field("Sexy", formatFloat(predictions.Sexy), true),
		},
		Footer: &discord.EmbedFooter{Text: "Powered by advanced Keyboard Cat technologies"},
	}

	c.ReplyEmbed(embed)
}

// topCategoryColor returns the embed colour of the highest-scoring category.
func topCategoryColor(p *Predictions) int {
	scores := []float32{p.Drawing, p.Hentai, p.Neutral, p.Porn, p.Sexy}
	colors := []int{0xD53113, 0x5B17B1, 0x2299B8, 0x6B1616, 0x1EB117}
	best := 0
	for i, score := range scores {
		if score > scores[best] {
			best = i
		}
	}
	return colors[best]
}

// IsSafe reports whether an image passes the NSFW filter. If the inference server is
// unavailable the image is allowed through, so keep the site-level filters (e.g.
// rating:safe) on as well.
func IsSafe(url string) bool {
	p, err := FromUrl(url)
	if err != nil {
		if !errors.Is(err, ErrNoInferenceServer) {
			log.Default().Println("NSFW filter unavailable, allowing image: ", err)
		}
		return true
	}
	if p.Hentai >= nsfwThreshold || p.Porn >= nsfwThreshold || p.Sexy >= nsfwThreshold {
		log.Default().Println("NSFW filter rejected image: " + url)
		return false
	}
	return true
}

// FromUrl downloads an image and sends it to the inference server for classification.
func FromUrl(url string) (*Predictions, error) {
	server := strings.TrimRight(config.GetConfig().InferenceServer, "/")
	if server == "" {
		return nil, ErrNoInferenceServer
	}

	imageData, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer imageData.Body.Close()
	if imageData.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download returned %s", imageData.Status)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image")
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, imageData.Body); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", server+"/classify", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("inference server returned %s: %s", resp.Status, msg)
	}

	predictions := &Predictions{}
	if err = json.NewDecoder(resp.Body).Decode(predictions); err != nil {
		return nil, err
	}
	return predictions, nil
}
