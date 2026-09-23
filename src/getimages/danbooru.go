package getimages

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

const (
	danbooruPageSize = 100
	danbooruMaxPages = 1000 // Danbooru doesn't allow paging past page 1000
)

type danbooruTag struct {
	Name      string `json:"name"`
	PostCount int    `json:"post_count"`
}

type danbooruPost struct {
	ID           int    `json:"id"`
	FileURL      string `json:"file_url"`
	LargeFileURL string `json:"large_file_url"`
	HasLarge     bool   `json:"has_large"`
	Source       string `json:"source"`
	TagString    string `json:"tag_string"`
}

func Danbooru(c *command.Ctx) {
	if !c.IsNSFW() {
		c.ReplyPrivate("This command cannot be ran on channels that aren't marked NSFW!")
		return
	}
	c.Defer()

	query := c.String("tags")
	post, err := searchDanbooru(query)
	if errors.Is(err, errNoResults) {
		c.Reply("Your search returned no result :(")
		return
	}
	if err != nil {
		log.Default().Println("Error searching Danbooru: " + err.Error())
		c.Reply(internetBroke)
		return
	}

	image := post.FileURL
	if post.HasLarge && post.LargeFileURL != "" {
		image = post.LargeFileURL
	}
	c.ReplyEmbed(&discordgo.MessageEmbed{
		Title: query,
		URL:   "https://danbooru.donmai.us/posts/" + strconv.Itoa(post.ID),
		Fields: []*discordgo.MessageEmbedField{
			command.Field("Source", post.Source, false),
			command.CodeField("Tags", post.TagString),
		},
		Image: &discordgo.MessageEmbedImage{URL: image},
	})
	remember(c)
}

// searchDanbooru resolves the query to the closest tag and returns a random post with it.
func searchDanbooru(query string) (*danbooruPost, error) {
	query = strings.ReplaceAll(strings.TrimSpace(query), " ", "_")

	// Prefer tags starting with the query (most popular first), then fuzzy matches
	tag, err := findDanbooruTag("name_or_alias_matches", query+"*")
	if err == nil && tag == nil {
		tag, err = findDanbooruTag("fuzzy_name_matches", query)
	}
	if err != nil {
		return nil, err
	}
	if tag == nil || tag.PostCount == 0 {
		return nil, errNoResults
	}

	choice := rand.Intn(min(tag.PostCount, danbooruPageSize*danbooruMaxPages))
	var posts []danbooruPost
	err = getJSON(fmt.Sprintf("https://danbooru.donmai.us/posts.json?tags=%s&limit=%d&page=%d",
		url.QueryEscape(tag.Name), danbooruPageSize, choice/danbooruPageSize+1), &posts)
	if err != nil {
		return nil, err
	}
	// Some posts may be hidden (e.g. banned artists), so pages can be short
	var usable []danbooruPost
	for _, post := range posts {
		if post.FileURL != "" {
			usable = append(usable, post)
		}
	}
	if len(usable) == 0 {
		return nil, errNoResults
	}
	return &usable[(choice%danbooruPageSize)%len(usable)], nil
}

func findDanbooruTag(searchType, value string) (*danbooruTag, error) {
	var tags []danbooruTag
	err := getJSON(fmt.Sprintf("https://danbooru.donmai.us/tags.json?search[%s]=%s&search[order]=count&search[hide_empty]=1&limit=1",
		searchType, url.QueryEscape(value)), &tags)
	if err != nil || len(tags) == 0 {
		return nil, err
	}
	return &tags[0], nil
}

func getJSON(url string, out any) error {
	req, err := newRequest(url)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s returned %s", req.URL.Host, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
