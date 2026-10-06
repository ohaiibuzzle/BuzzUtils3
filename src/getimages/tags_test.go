package getimages

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"
)

// fakeTagSite suggests every known tag starting with the query, and counts lookups.
func fakeTagSite(known ...string) (*tagSite, *int) {
	calls := 0
	return &tagSite{name: "fake", fetch: func(_ context.Context, query string) ([]tagSuggestion, error) {
		calls++
		var out []tagSuggestion
		for _, tag := range known {
			if strings.HasPrefix(normalizeTag(tag), normalizeTag(query)) {
				out = append(out, tagSuggestion{Name: tag, Label: tag})
			}
		}
		return out, nil
	}}, &calls
}

func TestSplitWords(t *testing.T) {
	site, calls := fakeTagSite("Hu Tao", "Hu Tao (Cosplay)", "Blue Hair", "Blue Hair Ribbon", "Ganyu (Genshin Impact)")
	for in, want := range map[string][]string{
		"hu tao ganyu":                      {"Hu Tao", "ganyu"},
		"blue hair hu tao":                  {"Blue Hair", "Hu Tao"},
		"blue hair ribbon ganyu":            {"Blue Hair Ribbon", "ganyu"},
		"ganyu (genshin impact) hu tao":     {"Ganyu (Genshin Impact)", "Hu Tao"},
		"eula ganyu":                        {"eula", "ganyu"},
		"hu_tao ganyu":                      {"hu_tao", "ganyu"},
		"hu tao":                            nil, // a single tag, nothing to split
		"ganyu":                             nil,
		"hu tao, ganyu":                     nil, // already separated
		"hu tao + ganyu":                    nil,
		"one two three four five six seven": nil, // too long to guess
	} {
		if got := site.splitWords(in); !slices.Equal(got, want) {
			t.Errorf("splitWords(%q) = %q, want %q", in, got, want)
		}
	}

	// Lookups are cached, so repeating a search costs nothing
	before := *calls
	site.splitWords("hu tao ganyu")
	if *calls != before {
		t.Errorf("repeated split made %d new lookups", *calls-before)
	}
}

func TestAutocomplete(t *testing.T) {
	site, calls := fakeTagSite("Hu Tao", "Ganyu (Genshin Impact)")
	values := func(choices []discord.AutocompleteChoice) []string {
		var out []string
		for _, choice := range choices {
			out = append(out, choice.(discord.AutocompleteChoiceString).Value)
		}
		return out
	}

	list := site.autocomplete(true)
	for in, want := range map[string][]string{
		"hu t":            {"Hu Tao"},
		"hu tao, gan":     {"hu tao, Ganyu (Genshin Impact)"},
		"hu tao +gan":     {"hu tao, Ganyu (Genshin Impact)"},
		" a , hu tao,gan": {"a, hu tao, Ganyu (Genshin Impact)"},
		"hu tao, g":       nil, // too short to look up
		"hu tao, ":        nil,
	} {
		if got := values(list(nil, in)); !slices.Equal(got, want) {
			t.Errorf("autocomplete(%q) = %q, want %q", in, got, want)
		}
	}

	if got := values(site.autocomplete(false)(nil, "hu tao, gan")); got != nil {
		t.Errorf("single-tag autocomplete treated the comma as a separator: %q", got)
	}

	before := *calls
	list(nil, "hu t")
	if *calls != before {
		t.Error("autocomplete lookups aren't cached")
	}
}

func TestSplitTagList(t *testing.T) {
	got := splitTagList(" Hu Tao ,ganyu+ + amber,")
	if want := []string{"Hu Tao", "ganyu", "amber"}; !slices.Equal(got, want) {
		t.Errorf("splitTagList = %q, want %q", got, want)
	}
}
