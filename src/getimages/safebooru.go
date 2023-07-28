package getimages

import (
	"encoding/xml"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

type SafebooruPosts struct {
	XMLName xml.Name        `xml:"posts"`
	Text    string          `xml:",chardata"`
	Count   string          `xml:"count,attr"`
	Offset  string          `xml:"offset,attr"`
	Posts   []SafebooruPost `xml:"post"`
}

type SafebooruPost struct {
	Text          string `xml:",chardata"`
	Height        string `xml:"height,attr"`
	Score         string `xml:"score,attr"`
	FileURL       string `xml:"file_url,attr"`
	ParentID      string `xml:"parent_id,attr"`
	SampleURL     string `xml:"sample_url,attr"`
	SampleWidth   string `xml:"sample_width,attr"`
	SampleHeight  string `xml:"sample_height,attr"`
	PreviewURL    string `xml:"preview_url,attr"`
	Rating        string `xml:"rating,attr"`
	Tags          string `xml:"tags,attr"`
	ID            string `xml:"id,attr"`
	Width         string `xml:"width,attr"`
	Change        string `xml:"change,attr"`
	Md5           string `xml:"md5,attr"`
	CreatorID     string `xml:"creator_id,attr"`
	HasChildren   string `xml:"has_children,attr"`
	CreatedAt     string `xml:"created_at,attr"`
	Status        string `xml:"status,attr"`
	Source        string `xml:"source,attr"`
	HasNotes      string `xml:"has_notes,attr"`
	HasComments   string `xml:"has_comments,attr"`
	PreviewWidth  string `xml:"preview_width,attr"`
	PreviewHeight string `xml:"preview_height,attr"`
}

func Safebooru(msg *discordgo.MessageCreate, ctx *discordgo.Session) {
	// Get the result
	result, err := getSafebooruResult(msg.Content[len(config.GetConfig().BotPrefix)+len("safebooru "):])
	if err != nil {
		log.Default().Println("Error getting result: " + err.Error())
		return
	}

	// Create the embed
	embed := makeSafebooruEmbed(result, msg, ctx)

	// Send the embed
	ctx.ChannelMessageSendEmbed(msg.ChannelID, embed)
}

func getSafebooruResult(searchTerm string) (*SafebooruPost, error) {
	countReq, err := getSafebooruPage(searchTerm, 0, 1)
	if err != nil {
		log.Default().Println("Error getting result: " + err.Error())
		return nil, err
	}

	// Get the post count
	postCount, err := strconv.Atoi(countReq.Count)
	if err != nil {
		log.Default().Println("Error converting post count: " + err.Error())
		return nil, err
	}

	// Select a random post
	postIndex := rand.Intn(postCount)

	page := postIndex / 100
	indexInPage := postIndex % 100

	// Get the result
	result, err := getSafebooruPage(searchTerm, page, 100)

	if err != nil {
		log.Default().Println("Error getting result: " + err.Error())
		return nil, errors.New("Error getting result: " + err.Error())
	}

	return &result.Posts[indexInPage], nil
}

func getSafebooruPage(searchTerm string, page int, limit int) (*SafebooruPosts, error) {
	// https://safebooru.org/index.php?page=dapi&s=post&q=index&tags=raiden_shogun

	// Convert the search term
	// sb uses (tag_1 + tag_2) instead of tag 1, tag 2
	searchTerm = convertSearchTerm(searchTerm)

	req, err := http.NewRequest("GET", "https://safebooru.org/index.php?page=dapi&s=post&q=index&limit="+strconv.Itoa(limit)+"&tags="+searchTerm+"&pid="+strconv.Itoa(page), nil)
	if err != nil {
		log.Default().Println("Error making request: " + err.Error())
		return nil, err
	}

	xmlResp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Default().Println("Error getting xml: " + err.Error())
		return nil, err
	}

	//Parse the page
	decoder := xml.NewDecoder(xmlResp.Body)
	var results SafebooruPosts
	err = decoder.Decode(&results)

	if err != nil {
		log.Default().Println("Error unmarshalling xml: " + err.Error())
		return nil, err
	}

	return &results, nil
}

func convertSearchTerm(searchTerm string) string {
	// Inject rating:safe
	if !strings.Contains(searchTerm, "rating:") {
		searchTerm += "+rating:safe"
	}
	// Convert the search term
	// sb uses (tag_1 + tag_2) instead of tag 1, tag 2
	searchTerm = strings.ReplaceAll(searchTerm, " ", "_")
	searchTerm = strings.ReplaceAll(searchTerm, "+", " ")
	return searchTerm
}

func makeSafebooruEmbed(result *SafebooruPost, msg *discordgo.MessageCreate, ctx *discordgo.Session) *discordgo.MessageEmbed {
	// Create the embed
	embed := &discordgo.MessageEmbed{
		Title: "Safebooru result",
		URL:   "https://safebooru.org/index.php?page=post&s=view&id=" + result.ID,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Source",
				Value: result.Source,
			},
			{
				Name:  "Tags",
				Value: "```\n" + result.Tags + "\n```",
			},
		},
	}

	// Add the image
	embed.Image = &discordgo.MessageEmbedImage{
		URL: result.FileURL,
	}

	return embed
}
