package getimages

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

// Pixiv's Android app credentials, as used by pixivpy and friends.
const (
	pixivClientID     = "MOBrBDS8blbauoSck0ZfDbtuzpyT"
	pixivClientSecret = "lsACyCD94FhDUtGTXi3QzcFE2uU1hqtDaKeqrdwj"
	pixivHashSecret   = "28c1fdd170a5204386cb1313c7077b34f83e4aaf4aa829ce78c231e05b0bae2c"
	pixivUserAgent    = "PixivAndroidApp/5.0.234 (Android 11; Pixel 5)"
	pixivAppAPI       = "https://app-api.pixiv.net"

	// Search offsets past this start failing, so random picks are capped
	pixivMaxOffset = 1450
	// Discord's upload limit for bots without boosts
	pixivMaxUpload = 10 << 20
)

var (
	errPixivRestricted = errors.New("restricted illustration")
	errPixivNoToken    = errors.New("no pixiv refresh token configured")

	pixivIDPattern = regexp.MustCompile(`\d+`)

	pixivTokenMu      sync.Mutex
	pixivAccessToken  string
	pixivTokenExpires time.Time
)

type pixivIllust struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	XRestrict   int    `json:"x_restrict"`
	SanityLevel int    `json:"sanity_level"`
	ImageURLs   struct {
		Large string `json:"large"`
	} `json:"image_urls"`
	User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
	Tags []struct {
		Name           string `json:"name"`
		TranslatedName string `json:"translated_name"`
	} `json:"tags"`
}

// isSafe reports whether the illustration is all-ages.
func (p *pixivIllust) isSafe() bool {
	return p.XRestrict == 0 && p.SanityLevel <= 5
}

func PixivRandom(c *command.Ctx) {
	c.Defer()
	query := c.String("tags")
	filter := !allowNSFW(c)

	total := pixivResultCount(query)
	if total == 0 {
		c.Reply("Nothing found :(\nCheck your query")
		return
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		illust, err := pixivSearch(query, rand.Intn(min(total, pixivMaxOffset)))
		if err != nil {
			log.Default().Println("Error searching Pixiv: " + err.Error())
			c.Reply(pixivErrorMessage(err))
			return
		}
		if illust == nil {
			continue
		}
		if filter && !illust.isSafe() {
			continue
		}

		if sendPixivIllust(c, illust, "") {
			remember(c)
		}
		return
	}
	c.Reply("Your search returned no result :(")
}

func PixivShow(c *command.Ctx) {
	c.Defer()
	id := pixivIDPattern.FindString(c.String("post"))
	if id == "" {
		c.ReplyPrivate("That doesn't look like a Pixiv link or ID!")
		return
	}

	illust, err := pixivIllustDetail(id)
	if err == nil && !allowNSFW(c) && !illust.isSafe() {
		err = errPixivRestricted
	}
	if err != nil {
		log.Default().Println("Error fetching Pixiv illustration: " + err.Error())
		c.Reply(pixivErrorMessage(err))
		return
	}

	if sendPixivIllust(c, illust, "Image fetched for "+c.Author.Username) {
		// The invocation is just a link, so remove it to avoid a duplicate embed
		c.DeleteInvocation()
	}
}

func pixivErrorMessage(err error) string {
	switch {
	case errors.Is(err, errPixivRestricted):
		return "This image is restricted :("
	case errors.Is(err, errPixivNoToken):
		return "Buzzle hasn't set up Pixiv yet :("
	}
	return internetBroke
}

// sendPixivIllust uploads the illustration (Pixiv blocks hotlinking) and sends its embed.
func sendPixivIllust(c *command.Ctx, illust *pixivIllust, content string) bool {
	data, err := pixivDownload(illust.ImageURLs.Large)
	if err != nil {
		log.Default().Println("Error downloading Pixiv image: " + err.Error())
		c.Reply(internetBroke)
		return false
	}

	filename := path.Base(illust.ImageURLs.Large)
	var tags []string
	for _, tag := range illust.Tags {
		if tag.TranslatedName != "" {
			tags = append(tags, tag.TranslatedName)
		} else {
			tags = append(tags, tag.Name)
		}
	}

	_, err = c.Send(&discordgo.MessageSend{
		Content: content,
		Embeds: []*discordgo.MessageEmbed{{
			Title: illust.Title,
			URL:   "https://www.pixiv.net/en/artworks/" + strconv.Itoa(illust.ID),
			Fields: []*discordgo.MessageEmbedField{
				command.Field("Title", illust.Title, false),
				command.Field("Author", fmt.Sprintf("%s, Pixiv ID: %d", illust.User.Name, illust.User.ID), false),
				command.Field("Tags", strings.Join(tags, ", "), false),
			},
			Image: &discordgo.MessageEmbedImage{URL: "attachment://" + filename},
		}},
		Files: []*discordgo.File{{
			Name:   filename,
			Reader: bytes.NewReader(data),
		}},
	})
	return err == nil
}

// pixivResultCount returns the (approximate) number of results for a tag search.
func pixivResultCount(query string) int {
	req, err := http.NewRequest("GET", "https://www.pixiv.net/ajax/search/artworks/"+url.PathEscape(query)+
		"?word="+url.QueryEscape(query)+"&s_mode=s_tag&type=all&lang=en", nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", config.GetConfig().UserAgent)

	var result struct {
		Body struct {
			IllustManga struct {
				Total int `json:"total"`
			} `json:"illustManga"`
		} `json:"body"`
	}
	resp, err := httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&result)
	}
	if err != nil {
		// Fall back to the first page of results
		log.Default().Println("Error getting Pixiv result count: " + err.Error())
		return 30
	}
	return result.Body.IllustManga.Total
}

func pixivSearch(query string, offset int) (*pixivIllust, error) {
	params := url.Values{}
	params.Set("word", query)
	params.Set("search_target", "partial_match_for_tags")
	params.Set("sort", "date_desc")
	params.Set("filter", "for_ios")
	params.Set("offset", strconv.Itoa(offset))

	var result struct {
		Illusts []pixivIllust `json:"illusts"`
	}
	if err := pixivAPI("/v1/search/illust?"+params.Encode(), &result); err != nil {
		return nil, err
	}
	if len(result.Illusts) == 0 {
		return nil, nil
	}
	return &result.Illusts[0], nil
}

func pixivIllustDetail(id string) (*pixivIllust, error) {
	var result struct {
		Illust *pixivIllust `json:"illust"`
	}
	if err := pixivAPI("/v1/illust/detail?illust_id="+id, &result); err != nil {
		return nil, err
	}
	if result.Illust == nil {
		return nil, errPixivRestricted
	}
	return result.Illust, nil
}

func pixivAPI(endpoint string, out any) error {
	token, err := pixivToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", pixivAppAPI+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", pixivUserAgent)
	req.Header.Set("App-OS", "android")
	req.Header.Set("App-OS-Version", "11")
	req.Header.Set("App-Version", "5.0.234")
	req.Header.Set("Accept-Language", "en-us")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errPixivRestricted
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
			// The token may have been revoked early, so fetch a new one next time
			pixivTokenMu.Lock()
			pixivAccessToken = ""
			pixivTokenMu.Unlock()
		}
		return fmt.Errorf("pixiv returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func pixivDownload(imageURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", pixivUserAgent)
	req.Header.Set("Referer", pixivAppAPI+"/")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pixiv image download returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, pixivMaxUpload+1))
	if err != nil {
		return nil, err
	}
	if len(data) > pixivMaxUpload {
		return nil, errors.New("pixiv image is too large to upload")
	}
	return data, nil
}

// pixivToken returns a cached access token, refreshing it with the configured
// refresh token when it has expired.
func pixivToken() (string, error) {
	pixivTokenMu.Lock()
	defer pixivTokenMu.Unlock()

	if pixivAccessToken != "" && time.Now().Before(pixivTokenExpires) {
		return pixivAccessToken, nil
	}
	refreshToken := config.GetConfig().PixivToken
	if refreshToken == "" {
		return "", errPixivNoToken
	}

	clientTime := time.Now().UTC().Format("2006-01-02T15:04:05+00:00")
	hash := md5.Sum([]byte(clientTime + pixivHashSecret))

	form := url.Values{}
	form.Set("client_id", pixivClientID)
	form.Set("client_secret", pixivClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("include_policy", "true")
	form.Set("get_secure_url", "true")

	req, err := http.NewRequest("POST", "https://oauth.secure.pixiv.net/auth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", pixivUserAgent)
	req.Header.Set("X-Client-Time", clientTime)
	req.Header.Set("X-Client-Hash", hex.EncodeToString(hash[:]))

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pixiv login returned %s", resp.Status)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", errors.New("pixiv login returned no access token")
	}

	pixivAccessToken = result.AccessToken
	// Refresh a minute early to avoid racing the expiry
	pixivTokenExpires = time.Now().Add(time.Duration(result.ExpiresIn)*time.Second - time.Minute)
	return pixivAccessToken, nil
}
