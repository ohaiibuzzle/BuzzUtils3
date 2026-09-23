package getimages

import "testing"

func TestConvertSearchTerm(t *testing.T) {
	for in, want := range map[string]string{
		"Ganyu (Genshin Impact) + Amber (Genshin Impact)": "ganyu_(genshin_impact) amber_(genshin_impact) rating:safe",
		"hu tao":                  "hu_tao rating:safe",
		"hu_tao + rating:general": "hu_tao rating:general",
		" + ":                     "rating:safe",
	} {
		if got := convertSearchTerm(in); got != want {
			t.Errorf("convertSearchTerm(%q) = %q, want %q", in, got, want)
		}
	}
}
