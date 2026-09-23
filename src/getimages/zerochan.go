package getimages

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	imageclassifier "github.com/ohaiibuzzle/BuzzUtils3/src/imageClassifier"
)

// Tags that are always filtered outside NSFW channels. The classifier *should*
// handle the rest.
var zerochanBannedTags = []string{"Nipples"}

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

func Zerochan(c *command.Ctx) {
	c.Defer()
	query := c.String("tags")
	filter := !allowNSFW(c)

	results, err := getZerochanResults(query)
	if err != nil {
		log.Default().Println("Error getting Zerochan results: " + err.Error())
		c.Reply(internetBroke)
		return
	}
	if len(results) == 0 {
		c.Reply("Sorry, I can't find you anything :( \nEither check your search, or Buzzle banned a tag in the result")
		return
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result := results[rand.Intn(len(results))]
		if filter && hasBannedTag(result.Tags) {
			continue
		}

		detail, err := getZerochanDetail(result.ID)
		if err != nil {
			log.Default().Println("Error getting Zerochan details: " + err.Error())
			continue
		}
		if filter && !imageclassifier.IsSafe(detail.Large) {
			continue
		}

		c.ReplyEmbed(makeZerochanEmbed(detail))
		remember(c)
		return
	}
	c.Reply("Your search string was wonky, or it included NSFW tags.\nTry again")
}

func hasBannedTag(tags []string) bool {
	for _, banned := range zerochanBannedTags {
		if slices.ContainsFunc(tags, func(tag string) bool { return strings.EqualFold(tag, banned) }) {
			return true
		}
	}
	return false
}

// getZerochanResults returns the 200 latest posts for "Tag One + Tag Two".
func getZerochanResults(query string) ([]ZerochanResult, error) {
	var tags []string
	for _, tag := range strings.Split(query, "+") {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, strings.ReplaceAll(url.PathEscape(tag), "%20", "+"))
		}
	}

	req, err := newRequest("https://www.zerochan.net/" + strings.Join(tags, ",") + "?l=200&s=id&json")
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zerochan returned %s", resp.Status)
	}

	var results ZerochanResults
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return results.Items, nil
}

func getZerochanDetail(id int) (*ZerochanDetailedResult, error) {
	req, err := newRequest("https://www.zerochan.net/" + strconv.Itoa(id) + "?json")
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zerochan returned %s", resp.Status)
	}

	var detail ZerochanDetailedResult
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func makeZerochanEmbed(result *ZerochanDetailedResult) *discordgo.MessageEmbed {
	link := "https://www.zerochan.net/" + strconv.Itoa(result.ID)
	title := result.Primary
	if title == "" {
		title = "Zerochan result"
	}
	return &discordgo.MessageEmbed{
		Title: title,
		URL:   link,
		Fields: []*discordgo.MessageEmbedField{
			command.Field("Source", result.Source, false),
			command.CodeField("Tags", strings.Join(result.Tags, ", ")),
		},
		Image: &discordgo.MessageEmbedImage{
			URL: result.Large,
		},
	}
}
