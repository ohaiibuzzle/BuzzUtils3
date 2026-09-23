package saucefinder

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
	"golang.org/x/net/html"
)

const (
	iqdbEndpoint = "https://iqdb.org/"
	iqdbMaxSize  = 8 << 20 // IQDB rejects files over 8 MiB
)

var iqdbClient = &http.Client{Timeout: 60 * time.Second}

type IqdbMatch struct {
	Link       string
	Thumbnail  string
	AltText    string
	Rating     string // Safe, Unrated, Ero or Explicit
	Similarity string
}

func IqdbCommand(c *command.Ctx) {
	c.Defer()
	iqdbMessage(c, c.FindMessage(10, MessageHasImages))
}

func iqdbMessage(c *command.Ctx, target *discord.Message) {
	image, ok := sauceTarget(c, target)
	if !ok {
		return
	}

	matches, err := searchIqdb(image)
	if err != nil {
		log.Default().Println("Error querying IQDB: " + err.Error())
		c.Reply("IQDB is having a moment :( Try again later")
		return
	}

	allowExplicit := c.IsDM() || c.IsNSFW()
	for _, match := range matches {
		if !allowExplicit && (match.Rating == "Ero" || match.Rating == "Explicit") {
			continue
		}
		c.ReplyEmbed(discord.Embed{
			Title:     "Sauce found!",
			URL:       match.Link,
			Thumbnail: &discord.EmbedResource{URL: match.Thumbnail},
			Fields: []discord.EmbedField{
				command.Field("Location", match.Link, false),
				command.Field("Similarity", match.Similarity, true),
				command.Field("Rating", match.Rating, true),
				command.Field("Alt Text", match.AltText, false),
			},
		})
		return
	}
	c.Reply("No sauce found")
}

// searchIqdb uploads the image to IQDB and returns the matches, best first.
// The image is uploaded rather than passed by URL, as IQDB's direct query links
// require a cookie check and many hosts block IQDB's hotlinking.
func searchIqdb(imageURL string) ([]IqdbMatch, error) {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.GetConfig().UserAgent)
	imgResp, err := iqdbClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download returned %s", imgResp.Status)
	}
	imgData, err := io.ReadAll(io.LimitReader(imgResp.Body, iqdbMaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(imgData) > iqdbMaxSize {
		return nil, errors.New("image is too large for IQDB")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "image")
	if err != nil {
		return nil, err
	}
	part.Write(imgData)
	writer.Close()

	req, err = http.NewRequest("POST", iqdbEndpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", config.GetConfig().UserAgent)

	resp, err := iqdbClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IQDB returned %s", resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseIqdbResults(doc), nil
}

// parseIqdbResults extracts the match tables from div#pages. Each match table looks like:
//
//	<tr><th>Best match</th></tr>
//	<tr><td class='image'><a href="//danbooru.donmai.us/posts/1"><img src='/thumb.jpg' alt="Rating: g Tags: ..."></a></td></tr>
//	<tr><td>Danbooru ...</td></tr>
//	<tr><td>2311×1300 [Safe]</td></tr>
//	<tr><td>98% similarity</td></tr>
func parseIqdbResults(doc *html.Node) []IqdbMatch {
	pages := findNode(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && attr(n, "id") == "pages"
	})
	if pages == nil {
		return nil
	}

	var matches []IqdbMatch
	for _, table := range findAll(pages, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "table"
	}) {
		th := findNode(table, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "th" })
		if th == nil || !strings.HasSuffix(strings.TrimSpace(textContent(th)), "match") {
			continue // "Your image", "No relevant matches", ...
		}
		link := findNode(table, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "a" })
		img := findNode(table, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "img" })
		if link == nil || img == nil {
			continue
		}

		match := IqdbMatch{
			Link:      absoluteURL(attr(link, "href")),
			Thumbnail: absoluteURL(attr(img, "src")),
			AltText:   attr(img, "alt"),
		}
		for _, td := range findAll(table, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "td" }) {
			text := strings.TrimSpace(textContent(td))
			if strings.HasSuffix(text, "similarity") {
				match.Similarity = strings.TrimSpace(strings.TrimSuffix(text, "similarity"))
			}
			if open := strings.LastIndex(text, "["); open >= 0 && strings.HasSuffix(text, "]") {
				match.Rating = text[open+1 : len(text)-1]
			}
		}
		matches = append(matches, match)
	}
	return matches
}

func absoluteURL(u string) string {
	switch {
	case strings.HasPrefix(u, "//"):
		return "https:" + u
	case strings.HasPrefix(u, "/"):
		return "https://iqdb.org" + u
	}
	return u
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findNode(n *html.Node, match func(*html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findNode(child, match); found != nil {
			return found
		}
	}
	return nil
}

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if match(child) {
			out = append(out, child)
		}
		out = append(out, findAll(child, match)...)
	}
	return out
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		sb.WriteString(textContent(child))
	}
	return sb.String()
}
