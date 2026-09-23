package getimages

import (
	"net/http"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

// maxAttempts is how many random picks are tried before giving up when the
// NSFW filter keeps rejecting results.
const maxAttempts = 3

var httpClient = &http.Client{Timeout: 15 * time.Second}

func tagsOption(description string) []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "tags",
			Description: description,
			Required:    true,
		},
	}
}

func init() {
	command.Register(
		&command.Command{
			Name:        "safebooru",
			Aliases:     []string{"sbrandom", "sbr"},
			Description: "Random image from SafeBooru",
			Options:     tagsOption("SafeBooru tags, combine tags using +"),
			Handler:     Safebooru,
		},
		&command.Command{
			Name:        "zerochan",
			Aliases:     []string{"zcrandom", "zcr"},
			Description: "Random image from ZeroChan",
			Options:     tagsOption("ZeroChan tags, combine tags using +"),
			Handler:     Zerochan,
		},
		&command.Command{
			Name:        "danbooru",
			Aliases:     []string{"danboorurandom", "dbr"},
			Description: "Random image from Danbooru (NSFW channels only)",
			Options:     tagsOption("A Danbooru tag"),
			GuildOnly:   true,
			Handler:     Danbooru,
		},
		&command.Command{
			Name:        "pixivrandom",
			Aliases:     []string{"pxr"},
			Description: "Random image from Pixiv",
			Options:     tagsOption("What to search for on Pixiv"),
			Handler:     PixivRandom,
		},
		&command.Command{
			Name:        "pixivshow",
			Aliases:     []string{"pxs"},
			Description: "Display a Pixiv post in the bot's format",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "post",
					Description: "A Pixiv artwork URL or illustration ID",
					Required:    true,
				},
			},
			Handler: PixivShow,
		},
		&command.Command{
			Name:        "more",
			Description: "Run your last image search again (within 15 seconds)",
			Handler:     More,
		},
	)
}

// allowNSFW reports whether results may skip the NSFW filters (NSFW channels and DMs).
func allowNSFW(c *command.Ctx) bool {
	return c.IsDM() || c.IsNSFW()
}

func newRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.GetConfig().UserAgent)
	return req, nil
}

const internetBroke = "Buzzle's Internet broke :(\n(Try again in a few minutes, server is under high load)"

// "more" remembers each user's last search per channel for a short while.
const moreTimeout = 15 * time.Second

type lastSearch struct {
	command string
	args    map[string]any
	expires time.Time
}

var (
	lastSearchesMu sync.Mutex
	lastSearches   = map[string]lastSearch{}
)

func remember(c *command.Ctx) {
	lastSearchesMu.Lock()
	defer lastSearchesMu.Unlock()
	now := time.Now()
	for key, entry := range lastSearches {
		if now.After(entry.expires) {
			delete(lastSearches, key)
		}
	}
	lastSearches[c.ChannelID+":"+c.Author.ID] = lastSearch{
		command: c.Command.Name,
		args:    c.Args(),
		expires: now.Add(moreTimeout),
	}
}

func More(c *command.Ctx) {
	lastSearchesMu.Lock()
	entry, ok := lastSearches[c.ChannelID+":"+c.Author.ID]
	lastSearchesMu.Unlock()

	if !ok || time.Now().After(entry.expires) {
		c.ReplyPrivate("I can't remember what you were doing~~")
		return
	}
	cmd := command.Lookup(entry.command)
	if cmd == nil {
		c.ReplyPrivate("I can't remember what you were doing~~")
		return
	}
	c.Redirect(cmd, entry.args)
}
