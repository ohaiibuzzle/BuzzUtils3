package music

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ohaiibuzzle/BuzzUtils3/src/config"
)

// maxSpotifyTracks caps how much of an album/playlist gets queued.
const maxSpotifyTracks = 200

var (
	spotifyURLPattern = regexp.MustCompile(`https?://open\.spotify\.com/(?:intl-[a-z]+/)?(track|album|playlist)/([A-Za-z0-9]+)`)

	errSpotifyDisabled = errors.New("Spotify links aren't set up on this bot")
	errPlaylistHidden  = errors.New("Spotify doesn't let bots read other people's playlists anymore :( Try an album or single tracks instead")

	spotifyHTTP    = &http.Client{Timeout: 15 * time.Second}
	spotifyTokenMu sync.Mutex
	spotifyToken   string
	spotifyExpiry  time.Time
)

type spotifyTrack struct {
	Name       string `json:"name"`
	DurationMS int    `json:"duration_ms"`
	Artists    []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
}

// toTrack makes a lazily resolved track that searches YouTube for "title artists".
func (s spotifyTrack) toTrack(fallbackThumbnail string) *Track {
	var artists []string
	for _, a := range s.Artists {
		artists = append(artists, a.Name)
	}
	t := &Track{
		Query:     "ytsearch1:" + s.Name + " " + strings.Join(artists, " "),
		Title:     s.Name,
		Uploader:  strings.Join(artists, ", "),
		Duration:  time.Duration(s.DurationMS) * time.Millisecond,
		Thumbnail: fallbackThumbnail,
	}
	if len(s.Album.Images) > 0 {
		t.Thumbnail = s.Album.Images[0].URL
	}
	return t
}

// spotifyTracks returns the tracks behind a Spotify link and a name for the
// collection, or ok = false if the text isn't a Spotify link.
func spotifyTracks(link string) (tracks []*Track, name string, ok bool, err error) {
	m := spotifyURLPattern.FindStringSubmatch(link)
	if m == nil {
		return nil, "", false, nil
	}
	cfg := config.GetConfig()
	if cfg.SpotifyClientID == "" || cfg.SpotifyClientSecret == "" {
		return nil, "", true, errSpotifyDisabled
	}

	kind, id := m[1], m[2]
	switch kind {
	case "track":
		var t spotifyTrack
		if err := spotifyGet("/tracks/"+id, &t); err != nil {
			return nil, "", true, err
		}
		return []*Track{t.toTrack("")}, "", true, nil

	case "album":
		var album struct {
			Name   string `json:"name"`
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
			Tracks spotifyPage[spotifyTrack] `json:"tracks"`
		}
		if err := spotifyGet("/albums/"+id, &album); err != nil {
			return nil, "", true, err
		}
		cover := ""
		if len(album.Images) > 0 {
			cover = album.Images[0].URL
		}
		items, err := collectPages(album.Tracks)
		for _, item := range items {
			tracks = append(tracks, item.toTrack(cover))
		}
		return tracks, album.Name, true, err

	default: // playlist
		var playlist struct {
			Name string `json:"name"`
		}
		if err := spotifyGet("/playlists/"+id+"?fields=name", &playlist); err != nil {
			return nil, "", true, err
		}
		var first spotifyPage[playlistItem]
		if err := spotifyGet("/playlists/"+id+"/items?limit=100", &first); err != nil {
			var status spotifyStatusError
			if errors.As(err, &status) && (status == http.StatusForbidden || status == http.StatusNotFound) {
				return nil, playlist.Name, true, errPlaylistHidden
			}
			return nil, playlist.Name, true, err
		}
		items, err := collectPages(first)
		for _, item := range items {
			if t := item.track(); t != nil {
				tracks = append(tracks, t.toTrack(""))
			}
		}
		if len(tracks) == 0 && err == nil {
			err = errPlaylistHidden
		}
		return tracks, playlist.Name, true, err
	}
}

type playlistItem struct {
	Item  *spotifyTrack `json:"item"`
	Track *spotifyTrack `json:"track"` // the pre-2026 name, in case it's still sent
}

func (p playlistItem) track() *spotifyTrack {
	if p.Item != nil {
		return p.Item
	}
	return p.Track
}

type spotifyPage[T any] struct {
	Items []T     `json:"items"`
	Next  *string `json:"next"`
}

// collectPages follows a paging object's next links, up to maxSpotifyTracks items.
func collectPages[T any](page spotifyPage[T]) ([]T, error) {
	items := page.Items
	for page.Next != nil && len(items) < maxSpotifyTracks {
		next := *page.Next
		page = spotifyPage[T]{}
		if err := spotifyGet(next, &page); err != nil {
			return items, err
		}
		items = append(items, page.Items...)
	}
	if len(items) > maxSpotifyTracks {
		items = items[:maxSpotifyTracks]
	}
	return items, nil
}

type spotifyStatusError int

func (e spotifyStatusError) Error() string {
	return fmt.Sprintf("Spotify returned %d %s", int(e), http.StatusText(int(e)))
}

// spotifyGet fetches an API path (or a full "next" URL) into out.
func spotifyGet(path string, out any) error {
	token, err := spotifyAccessToken()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(path, "https://") {
		path = "https://api.spotify.com/v1" + path
	}
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := spotifyHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return spotifyStatusError(resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// spotifyAccessToken returns a client-credentials token, refreshing it when it expires.
func spotifyAccessToken() (string, error) {
	spotifyTokenMu.Lock()
	defer spotifyTokenMu.Unlock()
	if spotifyToken != "" && time.Now().Before(spotifyExpiry) {
		return spotifyToken, nil
	}

	cfg := config.GetConfig()
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.SpotifyClientID, cfg.SpotifyClientSecret)
	resp, err := spotifyHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Spotify token request: %w", spotifyStatusError(resp.StatusCode))
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	spotifyToken = body.AccessToken
	spotifyExpiry = time.Now().Add(time.Duration(body.ExpiresIn)*time.Second - time.Minute)
	return spotifyToken, nil
}
