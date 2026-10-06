package getimages

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/ohaiibuzzle/BuzzUtils3/src/command"
	"golang.org/x/time/rate"
)

// Per-site request budgets, shared by searches, autocomplete and tag splitting.
// Zerochan documents 60/min and bans chronic offenders; Danbooru asks for ~1/s;
// Safebooru doesn't document one, so it gets the same treatment.
var siteLimits = map[string]*rate.Limiter{
	"zerochan.net":       rate.NewLimiter(rate.Every(1200*time.Millisecond), 5),
	"safebooru.org":      rate.NewLimiter(rate.Every(time.Second), 3),
	"danbooru.donmai.us": rate.NewLimiter(rate.Every(time.Second), 5),
}

// rateLimitedTransport waits for the host's budget before sending a request. The
// wait fails immediately if it would outlast the request's deadline, which is what
// keeps autocomplete from queueing behind searches.
type rateLimitedTransport struct{}

func (rateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if limiter := siteLimits[strings.TrimPrefix(req.URL.Hostname(), "www.")]; limiter != nil {
		if err := limiter.Wait(req.Context()); err != nil {
			return nil, fmt.Errorf("rate limited by us for %s: %w", req.URL.Host, err)
		}
	}
	return http.DefaultTransport.RoundTrip(req)
}

const (
	suggestCacheTTL = 10 * time.Minute
	suggestCacheMax = 2000
	// Discord drops autocomplete responses after 3s, and the debounce already used some.
	autocompleteTimeout = 2 * time.Second
	// "hu tao ganyu" style searches are only split up to this many words.
	maxSplitWords = 6
)

type tagSuggestion struct {
	Name  string // the tag to search for
	Label string // what to show in autocomplete
}

// tagSite looks up tag suggestions for one site, with a shared cache.
type tagSite struct {
	name  string
	fetch func(ctx context.Context, query string) ([]tagSuggestion, error)

	mu    sync.Mutex
	cache map[string]cachedSuggestions
}

type cachedSuggestions struct {
	suggestions []tagSuggestion
	expires     time.Time
}

var (
	zerochanTags  = &tagSite{name: "zerochan", fetch: fetchZerochanSuggestions}
	safebooruTags = &tagSite{name: "safebooru", fetch: fetchSafebooruSuggestions}
	danbooruTags  = &tagSite{name: "danbooru", fetch: fetchDanbooruSuggestions}
)

func (s *tagSite) suggest(ctx context.Context, query string) ([]tagSuggestion, error) {
	key := normalizeTag(query)
	now := time.Now()

	s.mu.Lock()
	if entry, ok := s.cache[key]; ok && now.Before(entry.expires) {
		s.mu.Unlock()
		return entry.suggestions, nil
	}
	s.mu.Unlock()

	suggestions, err := s.fetch(ctx, query)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil || len(s.cache) >= suggestCacheMax {
		for k, entry := range s.cache {
			if now.After(entry.expires) {
				delete(s.cache, k)
			}
		}
		if s.cache == nil || len(s.cache) >= suggestCacheMax {
			s.cache = map[string]cachedSuggestions{}
		}
	}
	s.cache[key] = cachedSuggestions{suggestions, now.Add(suggestCacheTTL)}
	return suggestions, nil
}

// autocomplete suggests completions for the last tag of "Tag One, Tag Tw", keeping
// the tags before it. With list false the whole input is a single tag.
func (s *tagSite) autocomplete(list bool) func(c *command.Ctx, typed string) []discord.AutocompleteChoice {
	return func(c *command.Ctx, typed string) []discord.AutocompleteChoice {
		prefix, current := "", strings.TrimSpace(typed)
		if list {
			if cut := strings.LastIndexAny(typed, ",+"); cut >= 0 {
				if earlier := splitTagList(typed[:cut]); len(earlier) > 0 {
					prefix = strings.Join(earlier, ", ") + ", "
				}
				current = strings.TrimSpace(typed[cut+1:])
			}
		}
		if len([]rune(current)) < 2 {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), autocompleteTimeout)
		defer cancel()
		suggestions, err := s.suggest(ctx, current)
		if err != nil {
			return nil
		}

		var choices []discord.AutocompleteChoice
		for _, suggestion := range suggestions {
			value := prefix + suggestion.Name
			if len([]rune(value)) > 100 {
				continue // Discord's limit, and a cut-off tag is useless
			}
			choices = append(choices, discord.AutocompleteChoiceString{
				Name:  command.Truncate(prefix+suggestion.Label, 100),
				Value: value,
			})
		}
		return choices
	}
}

// splitWords splits a separator-less search like "hu tao ganyu" into tags, e.g.
// ["Hu Tao", "ganyu"]: at each word it asks the site for tags starting with that
// word and the next, and takes the longest one matching the following words.
// That costs at most one (cached) lookup per tag. Returns nil if there's nothing to split.
func (s *tagSite) splitWords(query string) []string {
	if strings.ContainsAny(query, ",+") {
		return nil
	}
	words := strings.Fields(query)
	if len(words) < 2 || len(words) > maxSplitWords {
		return nil
	}

	var tags []string
	for i := 0; i < len(words); {
		tag, length := words[i], 1
		if i+1 < len(words) {
			ctx, cancel := context.WithTimeout(context.Background(), httpClient.Timeout)
			suggestions, err := s.suggest(ctx, words[i]+" "+words[i+1])
			cancel()
			if err != nil {
				return nil
			}
			for _, suggestion := range suggestions {
				name := normalizeTag(suggestion.Name)
				n := len(strings.Fields(name))
				if n > length && i+n <= len(words) && normalizeTag(strings.Join(words[i:i+n], " ")) == name {
					tag, length = suggestion.Name, n
				}
			}
		}
		tags = append(tags, tag)
		i += length
	}
	if len(tags) < 2 {
		return nil
	}
	return tags
}

// splitTagList splits "Tag One, Tag Two + Tag Three" into its tags.
func splitTagList(query string) []string {
	var tags []string
	for _, tag := range strings.FieldsFunc(query, func(r rune) bool { return r == ',' || r == '+' }) {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// normalizeTag makes "Hu_Tao" and "hu  tao" compare equal.
func normalizeTag(tag string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(tag, "_", " "))), " ")
}

func getWithContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := newRequest(url)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s returned %s", req.URL.Host, resp.Status)
	}
	return resp, nil
}

// Zerochan's search box endpoint answers with "Name|Type|Parent" lines.
func fetchZerochanSuggestions(ctx context.Context, query string) ([]tagSuggestion, error) {
	resp, err := getWithContext(ctx, "https://www.zerochan.net/suggest?q="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var suggestions []tagSuggestion
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "|")
		name := strings.TrimSpace(fields[0])
		if name == "" || hasBannedTag([]string{name}) {
			continue
		}
		label := name
		if len(fields) > 1 && fields[1] != "" {
			label += " (" + fields[1] + ")"
		}
		suggestions = append(suggestions, tagSuggestion{Name: name, Label: label})
	}
	return suggestions, scanner.Err()
}

func fetchSafebooruSuggestions(ctx context.Context, query string) ([]tagSuggestion, error) {
	q := strings.ReplaceAll(normalizeTag(query), " ", "_")
	resp, err := getWithContext(ctx, "https://safebooru.org/autocomplete.php?q="+url.QueryEscape(q))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []struct {
		Label string `json:"label"` // "hu_tao (790)"
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	var suggestions []tagSuggestion
	for _, result := range results {
		suggestions = append(suggestions, tagSuggestion{Name: result.Value, Label: result.Label})
	}
	return suggestions, nil
}

func fetchDanbooruSuggestions(ctx context.Context, query string) ([]tagSuggestion, error) {
	q := strings.ReplaceAll(normalizeTag(query), " ", "_")
	resp, err := getWithContext(ctx, "https://danbooru.donmai.us/autocomplete.json?search[type]=tag_query&limit=20&search[query]="+url.QueryEscape(q))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var results []struct {
		Value     string `json:"value"`
		PostCount int    `json:"post_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	var suggestions []tagSuggestion
	for _, result := range results {
		suggestions = append(suggestions, tagSuggestion{
			Name:  result.Value,
			Label: fmt.Sprintf("%s (%d)", result.Value, result.PostCount),
		})
	}
	return suggestions, nil
}
