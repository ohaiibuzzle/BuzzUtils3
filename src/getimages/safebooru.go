package getimages

import (
	"encoding/xml"
	"errors"
	"log"
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	imageclassifier "github.com/ohaiibuzzle/BuzzUtils3/src/imageClassifier"
)

const safebooruPageSize = 100

type SafebooruPosts struct {
	XMLName xml.Name        `xml:"posts"`
	Count   string          `xml:"count,attr"`
	Offset  string          `xml:"offset,attr"`
	Posts   []SafebooruPost `xml:"post"`
}

type SafebooruPost struct {
	FileURL    string `xml:"file_url,attr"`
	PreviewURL string `xml:"preview_url,attr"`
	SampleURL  string `xml:"sample_url,attr"`
	Rating     string `xml:"rating,attr"`
	Tags       string `xml:"tags,attr"`
	ID         string `xml:"id,attr"`
	Source     string `xml:"source,attr"`
}

var errNoResults = errors.New("no results")

func Safebooru(c *command.Ctx) {
	c.Defer()
	tags := convertSearchTerm(c.String("tags"))
	filter := !allowNSFW(c)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := getSafebooruResult(tags)
		if errors.Is(err, errNoResults) {
			c.Reply("Your search returned no result :(")
			return
		}
		if err != nil {
			log.Default().Println("Error getting Safebooru result: " + err.Error())
			c.Reply(internetBroke)
			return
		}
		if filter && !imageclassifier.IsSafe(result.FileURL) {
			continue
		}

		c.ReplyEmbed(makeSafebooruEmbed(result))
		remember(c)
		return
	}
	c.Reply("Sorry, I can't find you anything :( \nEither check your search, or Buzzle banned a tag in the result")
}

func getSafebooruResult(tags string) (*SafebooruPost, error) {
	countReq, err := getSafebooruPage(tags, 0, 1)
	if err != nil {
		return nil, err
	}

	postCount, err := strconv.Atoi(countReq.Count)
	if err != nil {
		return nil, err
	}
	if postCount <= 0 {
		return nil, errNoResults
	}

	postIndex := rand.Intn(postCount)
	page, err := getSafebooruPage(tags, postIndex/safebooruPageSize, safebooruPageSize)
	if err != nil {
		return nil, err
	}
	if len(page.Posts) == 0 {
		return nil, errNoResults
	}

	return &page.Posts[(postIndex%safebooruPageSize)%len(page.Posts)], nil
}

func getSafebooruPage(tags string, page int, limit int) (*SafebooruPosts, error) {
	query := url.Values{}
	query.Set("page", "dapi")
	query.Set("s", "post")
	query.Set("q", "index")
	query.Set("limit", strconv.Itoa(limit))
	query.Set("pid", strconv.Itoa(page))
	query.Set("tags", tags)

	req, err := newRequest("https://safebooru.org/index.php?" + query.Encode())
	if err != nil {
		return nil, err
	}

	xmlResp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer xmlResp.Body.Close()

	var results SafebooruPosts
	if err := xml.NewDecoder(xmlResp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return &results, nil
}

// convertSearchTerm converts "Tag One + Tag Two" into SafeBooru's "tag_one tag_two".
func convertSearchTerm(searchTerm string) string {
	var tags []string
	for _, tag := range strings.Split(searchTerm, "+") {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			tags = append(tags, strings.ReplaceAll(tag, " ", "_"))
		}
	}
	if !strings.Contains(searchTerm, "rating:") {
		tags = append(tags, "rating:safe")
	}
	return strings.Join(tags, " ")
}

func makeSafebooruEmbed(result *SafebooruPost) discord.Embed {
	return imageEmbed("Your random image!", "https://safebooru.org/index.php?page=post&s=view&id="+result.ID,
		result.Source, strings.TrimSpace(result.Tags), result.FileURL)
}
