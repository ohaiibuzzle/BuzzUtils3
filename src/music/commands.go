// Package music plays YouTube (and Spotify, via YouTube) audio in voice channels.
package music

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
)

const (
	queuePageSize = 10
	searchResults = 10
	searchTimeout = 30 * time.Second
)

func init() {
	str := func(name, description string) []command.Option {
		return []command.Option{{Type: discord.ApplicationCommandOptionTypeString, Name: name, Description: description, Required: true}}
	}
	integer := func(name, description string, required bool, lo, hi int) []command.Option {
		return []command.Option{{Type: discord.ApplicationCommandOptionTypeInt, Name: name, Description: description, Required: required, MinValue: &lo, MaxValue: &hi}}
	}
	cmd := func(name, description string, options []command.Option, handler func(c *command.Ctx), aliases ...string) *command.Command {
		return &command.Command{Name: name, Aliases: aliases, Description: description, Options: options, GuildOnly: true, Handler: handler}
	}
	command.Register(
		cmd("join", "Join your voice channel", nil, Join, "summon"),
		cmd("leave", "Leave the voice channel and clear the queue", nil, Leave, "dc", "disconnect"),
		cmd("play", "Play a YouTube/Spotify link, or the top YouTube result for a search", str("query", "A link, or what to search for"), Play, "p"),
		cmd("search", "Search YouTube and pick what to play", str("query", "What to search for"), Search),
		cmd("pause", "Pause the music", nil, Pause),
		cmd("resume", "Resume the music", nil, Resume),
		cmd("skip", "Skip the current song (and more)", integer("amount", "How many songs to skip (default 1)", false, 1, 1000), Skip),
		cmd("remove", "Remove a song from the queue", integer("position", "The song's position in the queue", true, 1, 10000), Remove),
		cmd("queue", "Show the queue", integer("page", "Which page to show", false, 1, 1000), Queue, "q"),
		cmd("nowplaying", "Show the current song", nil, NowPlaying, "np"),
		cmd("loop", "Loop/unloop the current song", nil, Loop),
		cmd("queueloop", "Loop/unloop the whole queue", nil, QueueLoop),
		cmd("volume", "Set the volume", integer("percent", "Volume from 0 to 100", true, 0, 100), Volume, "vol"),
		cmd("seek", "Jump to a point in the current song", str("timestamp", "Where to jump to, e.g. 1:30"), Seek),
		cmd("exportqueue", "Export the queue as a text file", nil, ExportQueue),
	)
}

// userVoiceChannel returns the voice channel the user is in, or 0.
func userVoiceChannel(c *command.Ctx) snowflake.ID {
	if state, ok := c.Client.Caches.VoiceState(c.GuildID, c.Author.ID); ok && state.ChannelID != nil {
		return *state.ChannelID
	}
	return 0
}

// joinUser returns the server's player, connecting to the user's voice channel if
// needed, and whether it just joined. It replies and returns nil if that isn't possible.
func joinUser(c *command.Ctx) (*Player, bool) {
	channelID := userVoiceChannel(c)
	if channelID == 0 {
		c.ReplyPrivate("Hmm? What should I join here? Hop in a voice channel first!")
		return nil, false
	}
	p, created, err := getOrConnect(c.Client, c.GuildID, channelID, c.Author.ID, c.ChannelID)
	if err != nil {
		log.Default().Println("Error joining voice channel: " + err.Error())
		c.ReplyPrivate("I couldn't join your voice channel. Do I have permission to connect and speak there?")
		return nil, false
	}
	if p.ChannelID() != channelID {
		c.ReplyPrivate("I'm already playing in " + discord.ChannelMention(p.ChannelID()) + "!")
		return nil, false
	}
	return p, created
}

// controlledPlayer returns the server's player if the user is listening to it, so
// people outside the channel can't mess with the music. It replies and returns nil
// otherwise.
func controlledPlayer(c *command.Ctx) *Player {
	p := listenedPlayer(c)
	if p != nil && userVoiceChannel(c) != p.ChannelID() {
		c.ReplyPrivate("You need to be in " + discord.ChannelMention(p.ChannelID()) + " to do that!")
		return nil
	}
	return p
}

// listenedPlayer returns the server's player, replying and returning nil if there isn't one.
func listenedPlayer(c *command.Ctx) *Player {
	p := getPlayer(c.GuildID)
	if p == nil {
		c.ReplyPrivate("I'm not in a voice channel right now!")
	}
	return p
}

func Join(c *command.Ctx) {
	p, created := joinUser(c)
	switch {
	case p == nil:
	case created:
		c.Reply("Joined " + discord.ChannelMention(p.ChannelID()) + "!")
	default:
		c.ReplyPrivate("I'm already here!")
	}
}

func Leave(c *command.Ctx) {
	p := listenedPlayer(c)
	if p == nil {
		return
	}
	// Admins and whoever summoned the bot can always disconnect it; others only
	// when nobody else is listening
	if !c.HasPermission(discord.PermissionManageChannels) && c.Author.ID != p.summoner {
		if userVoiceChannel(c) != p.ChannelID() {
			c.ReplyPrivate("You are not in the voice channel I'm in.")
			return
		}
		for state := range c.Client.Caches.VoiceStates(c.GuildID) {
			if state.ChannelID != nil && *state.ChannelID == p.ChannelID() &&
				state.UserID != c.Author.ID && state.UserID != c.Client.ID() {
				c.ReplyPrivate("Others are listening right now. Only an admin or whoever summoned me can disconnect me.")
				return
			}
		}
	}
	p.leave()
	c.Reply("Disconnected and cleared the queue 👋")
}

func Play(c *command.Ctx) {
	p, _ := joinUser(c)
	if p == nil {
		return
	}
	c.Defer()
	query := strings.Trim(strings.TrimSpace(c.String("query")), "<>")

	tracks, name, isSpotify, err := spotifyTracks(query)
	switch {
	case isSpotify && err != nil && len(tracks) == 0:
		if !errors.Is(err, errSpotifyDisabled) && !errors.Is(err, errPlaylistHidden) {
			log.Default().Println("Error reading Spotify link: " + err.Error())
			err = errors.New("Something funky happened talking to Spotify :(")
		}
		c.Reply(err.Error())
		return
	case !isSpotify:
		t := &Track{Query: query}
		if err := t.resolve(context.Background()); err != nil {
			log.Default().Println("Error resolving track: " + err.Error())
			c.Reply("I couldn't find anything to play for that :(")
			return
		}
		tracks = []*Track{t}
	}
	enqueue(c, p, name, tracks)
}

func Search(c *command.Ctx) {
	p, _ := joinUser(c)
	if p == nil {
		return
	}
	c.Defer()
	query := c.String("query")
	results, err := search(context.Background(), query, searchResults)
	if err != nil {
		log.Default().Println("Error searching YouTube: " + err.Error())
		c.Reply("YouTube is having a moment :( Try again later")
		return
	}
	if len(results) == 0 {
		c.Reply("I couldn't find anything for that :(")
		return
	}

	var choices []command.Choice
	for i, t := range results {
		choices = append(choices, command.Choice{
			Label:       t.Title,
			Description: t.Uploader + " · " + formatDuration(t.Duration),
			Value:       strconv.Itoa(i),
		})
	}
	answer, ok := c.AskChoice(discord.MessageCreate{Content: "Search results for **" + query + "**"},
		c.Author.ID, "Pick something to play", choices, searchTimeout)
	if !ok {
		c.Reply("Timeout!")
		return
	}
	i, _ := strconv.Atoi(answer)
	picked := results[i]
	if err := picked.resolve(context.Background()); err != nil {
		log.Default().Println("Error resolving track: " + err.Error())
		c.Reply("I couldn't play that one :(")
		return
	}
	// The player may have left while the user was picking
	if p, _ = joinUser(c); p != nil {
		enqueue(c, p, "", []*Track{picked})
	}
}

// enqueue queues the tracks and tells the user where they ended up.
func enqueue(c *command.Ctx, p *Player, collection string, tracks []*Track) {
	for _, t := range tracks {
		t.Requester, t.TextChannel = c.Author, c.ChannelID
	}
	position := p.enqueue(tracks...)
	if len(tracks) > 1 {
		c.Send(discord.MessageCreate{
			Content:         fmt.Sprintf("Added %d songs from **%s** to the queue!", len(tracks), collection),
			AllowedMentions: &discord.AllowedMentions{},
		})
		return
	}
	where := "Up next!"
	if position == 0 {
		where = "Starting now!"
	} else if position > 1 {
		where = fmt.Sprintf("Position %d in the queue.", position)
	}
	c.Send(discord.MessageCreate{
		Content:         "Added " + tracks[0].label() + " to the queue! " + where,
		AllowedMentions: &discord.AllowedMentions{},
	})
}

func Pause(c *command.Ctx) {
	if p := controlledPlayer(c); p != nil {
		if p.setPaused(true) {
			c.Reply("Paused ⏸️")
		} else {
			c.ReplyPrivate("Nothing to pause!")
		}
	}
}

func Resume(c *command.Ctx) {
	if p := controlledPlayer(c); p != nil {
		if p.setPaused(false) {
			c.Reply("Resumed ▶️")
		} else {
			c.ReplyPrivate("Nothing is paused!")
		}
	}
}

func Skip(c *command.Ctx) {
	p := controlledPlayer(c)
	if p == nil {
		return
	}
	if p.snapshot().current == nil {
		c.ReplyPrivate("Not playing any music right now...")
		return
	}
	n := 1
	if amount, ok := c.Int("amount"); ok && amount > 1 {
		n = int(amount)
	}
	p.skip(n)
	if n == 1 {
		c.Reply("Skipped ⏭️")
	} else {
		c.Reply(fmt.Sprintf("Skipped %d songs ⏭️", n))
	}
}

func Remove(c *command.Ctx) {
	p := controlledPlayer(c)
	if p == nil {
		return
	}
	position, _ := c.Int("position")
	t, ok := p.remove(int(position))
	if !ok {
		c.ReplyPrivate("There's no song at that position!")
		return
	}
	c.Send(discord.MessageCreate{
		Content:         "Removed " + t.label() + " from the queue.",
		AllowedMentions: &discord.AllowedMentions{},
	})
}

func Queue(c *command.Ctx) {
	p := listenedPlayer(c)
	if p == nil {
		return
	}
	snap := p.snapshot()
	if snap.current == nil && len(snap.queue) == 0 {
		c.Reply("Oh no, the queue is empty :(")
		return
	}

	pages := max(1, (len(snap.queue)+queuePageSize-1)/queuePageSize)
	page := 1
	if n, ok := c.Int("page"); ok {
		page = min(max(int(n), 1), pages)
	}
	var sb strings.Builder
	if snap.current != nil {
		sb.WriteString("**Now playing:** " + snap.current.label() + "\n\n")
	}
	start := (page - 1) * queuePageSize
	var total time.Duration
	for i, t := range snap.queue {
		total += t.Duration
		if i >= start && i < start+queuePageSize {
			fmt.Fprintf(&sb, "**%d.** %s (%s)\n", i+1, t.label(), formatDuration(t.Duration))
		}
	}

	footer := fmt.Sprintf("Page %d/%d · %d queued · %s total", page, pages, len(snap.queue), formatDuration(total))
	if snap.loop {
		footer += " · 🔂 loop"
	}
	if snap.queueLoop {
		footer += " · 🔁 queue loop"
	}
	c.ReplyEmbed(discord.Embed{
		Title:       "Queue",
		Description: sb.String(),
		Color:       embedColor,
		Footer:      &discord.EmbedFooter{Text: footer},
	})
}

func NowPlaying(c *command.Ctx) {
	p := listenedPlayer(c)
	if p == nil {
		return
	}
	snap := p.snapshot()
	if snap.current == nil {
		c.Reply("Nothing is playing right now.")
		return
	}
	heading := "Now Playing"
	if snap.paused {
		heading = "Paused"
	}
	c.ReplyEmbed(snap.current.embed(heading, snap.position))
}

func Loop(c *command.Ctx) {
	if p := controlledPlayer(c); p != nil {
		if p.toggleLoop(false) {
			c.Reply("Looping this song 🔂")
		} else {
			c.Reply("Unlooped")
		}
	}
}

func QueueLoop(c *command.Ctx) {
	if p := controlledPlayer(c); p != nil {
		if p.toggleLoop(true) {
			c.Reply("Looping the queue 🔁")
		} else {
			c.Reply("Unlooped the queue")
		}
	}
}

func Volume(c *command.Ctx) {
	p := controlledPlayer(c)
	if p == nil {
		return
	}
	percent, _ := c.Int("percent")
	if percent < 0 || percent > 100 {
		c.ReplyPrivate("What kind of silly volume is that??? Pick 0 to 100.")
		return
	}
	if err := p.setVolume(int(percent)); err != nil {
		log.Default().Println("Error changing volume: " + err.Error())
		c.ReplyPrivate("I couldn't change the volume :(")
		return
	}
	c.Reply(fmt.Sprintf("Done! The volume is now %d%%", percent))
}

func Seek(c *command.Ctx) {
	p := controlledPlayer(c)
	if p == nil {
		return
	}
	to, err := parseTimestamp(c.String("timestamp"))
	if err != nil {
		c.ReplyPrivate("Heeeeey! That is not a valid timestamp. Try something like `1:30`.")
		return
	}
	if current := p.snapshot().current; current != nil && current.Duration > 0 && to >= current.Duration {
		c.ReplyPrivate("That's past the end of the song!")
		return
	}
	if err := p.seek(to); err != nil {
		if !errors.Is(err, errNothingPlaying) {
			log.Default().Println("Error seeking: " + err.Error())
		}
		c.ReplyPrivate("I can't seek right now :(")
		return
	}
	c.Reply("Jumped to " + formatDuration(to) + " ⏩")
}

func ExportQueue(c *command.Ctx) {
	p := listenedPlayer(c)
	if p == nil {
		return
	}
	snap := p.snapshot()
	tracks := snap.queue
	if snap.current != nil {
		tracks = append([]Track{*snap.current}, tracks...)
	}
	if len(tracks) == 0 {
		c.Reply("Oh no, the queue is empty :(")
		return
	}
	var sb strings.Builder
	for i, t := range tracks {
		fmt.Fprintf(&sb, "%d. %s - %s", i+1, t.Title, t.Uploader)
		if t.URL != "" {
			sb.WriteString(" - " + t.URL)
		}
		sb.WriteString("\n")
	}
	c.Send(discord.MessageCreate{
		Content: "Here's the current queue!",
		Files:   []*discord.File{discord.NewFile("queue-"+c.GuildID.String()+".txt", "", strings.NewReader(sb.String()))},
	})
}

const embedColor = 0x0062FF

// label is the track's title (linked when possible) and uploader, for messages.
func (t *Track) label() string {
	title := "**" + t.Title + "**"
	if t.URL != "" {
		title = "[" + title + "](" + t.URL + ")"
	}
	if t.Uploader == "" {
		return title
	}
	return title + " by " + t.Uploader
}

func (t *Track) embed(heading string, position time.Duration) discord.Embed {
	duration := formatDuration(t.Duration)
	if position > 0 {
		duration = formatDuration(position) + " / " + duration
	}
	e := discord.Embed{
		Title:       heading,
		Description: t.label(),
		Color:       embedColor,
		Fields: []discord.EmbedField{
			command.Field("Requested by", t.Requester.Mention(), true),
			command.Field("Duration", duration, true),
		},
	}
	if t.Thumbnail != "" {
		e.Thumbnail = &discord.EmbedResource{URL: t.Thumbnail}
	}
	return e
}

// formatDuration formats as m:ss or h:mm:ss; unknown durations (e.g. live streams) show as "?".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "?"
	}
	s := int(d.Round(time.Second).Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// parseTimestamp parses "90", "1:30" or "1:02:03".
func parseTimestamp(s string) (time.Duration, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("too many parts")
	}
	var seconds float64
	for i, part := range parts {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil || v < 0 || math.IsInf(v, 0) || (i > 0 && v >= 60) {
			return 0, fmt.Errorf("invalid timestamp %q", s)
		}
		seconds = seconds*60 + v
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
