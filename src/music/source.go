package music

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Stream URLs from YouTube expire after a few hours, so older ones are looked up again.
const streamURLMaxAge = time.Hour

// Track is a queued song. Tracks from Spotify only know their search query until
// they are about to play; resolve fills in the rest.
type Track struct {
	Query       string // a URL, or "ytsearch1:..." for Spotify tracks
	Title       string
	Uploader    string
	URL         string // the page to link to
	Thumbnail   string
	Duration    time.Duration
	Requester   discord.User
	TextChannel snowflake.ID // where "Now playing" is announced

	streamURL  string
	resolvedAt time.Time
}

type ytdlpInfo struct {
	Title      string  `json:"title"`
	Uploader   string  `json:"uploader"`
	Channel    string  `json:"channel"`
	WebpageURL string  `json:"webpage_url"`
	URL        string  `json:"url"`
	Thumbnail  string  `json:"thumbnail"`
	Duration   float64 `json:"duration"`
}

func (i ytdlpInfo) uploader() string {
	if i.Uploader != "" {
		return i.Uploader
	}
	return i.Channel
}

// resolve looks the track up with yt-dlp, unless its stream URL is still fresh.
func (t *Track) resolve(ctx context.Context) error {
	if t.streamURL != "" && time.Since(t.resolvedAt) < streamURLMaxAge {
		return nil
	}
	out, err := ytdlp(ctx, "-j", "--no-playlist", "-f", "bestaudio/best", "--default-search", "ytsearch", t.Query)
	if err != nil {
		return err
	}
	var info ytdlpInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return fmt.Errorf("parsing yt-dlp output: %w", err)
	}
	if info.URL == "" {
		return fmt.Errorf("yt-dlp found no audio for %q", t.Query)
	}

	// Spotify tracks keep their own title/artist, as they are what the user asked for
	if t.Title == "" {
		t.Title, t.Uploader = info.Title, info.uploader()
	}
	if t.Duration == 0 {
		t.Duration = time.Duration(info.Duration * float64(time.Second))
	}
	if t.Thumbnail == "" {
		t.Thumbnail = info.Thumbnail
	}
	// Replay the exact video from now on, rather than searching again
	t.URL, t.Query = info.WebpageURL, info.WebpageURL
	t.streamURL, t.resolvedAt = info.URL, time.Now()
	return nil
}

// search returns the top YouTube results for a query, without resolving their streams.
func search(ctx context.Context, query string, amount int) ([]*Track, error) {
	out, err := ytdlp(ctx, "-j", "--flat-playlist", fmt.Sprintf("ytsearch%d:%s", amount, query))
	if err != nil {
		return nil, err
	}
	var tracks []*Track
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		var info ytdlpInfo
		if json.Unmarshal(scanner.Bytes(), &info) != nil || info.URL == "" {
			continue
		}
		tracks = append(tracks, &Track{
			Query:    info.URL,
			Title:    info.Title,
			Uploader: info.uploader(),
			URL:      info.URL,
			Duration: time.Duration(info.Duration * float64(time.Second)),
		})
	}
	return tracks, nil
}

func ytdlp(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "yt-dlp", append([]string{"--quiet", "--no-warnings"}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// stream is a running ffmpeg process turning a track into 20ms Opus frames.
type stream struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	ogg    *oggOpusReader
}

// newStream starts ffmpeg at the given offset, with volume from 0 to 1 (or more).
func newStream(streamURL string, offset time.Duration, volume float64) (*stream, error) {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if strings.HasPrefix(streamURL, "http") {
		args = append(args, "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5")
	}
	if offset > 0 {
		args = append(args, "-ss", strconv.FormatFloat(offset.Seconds(), 'f', 3, 64))
	}
	args = append(args, "-i", streamURL, "-vn",
		"-af", "volume="+strconv.FormatFloat(volume, 'f', 2, 64),
		"-c:a", "libopus", "-b:a", "128k", "-ar", "48000", "-ac", "2",
		"-frame_duration", "20", "-application", "audio",
		"-f", "ogg", "pipe:1")

	cmd := exec.Command("ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}
	return &stream{cmd: cmd, stdout: stdout, ogg: newOggOpusReader(stdout)}, nil
}

func (s *stream) next() ([]byte, error) {
	return s.ogg.Next()
}

func (s *stream) close() {
	s.cmd.Process.Kill()
	s.stdout.Close()
	s.cmd.Wait()
}
