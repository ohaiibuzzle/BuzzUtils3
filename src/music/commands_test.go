package music

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"90":      90 * time.Second,
		"1:30":    90 * time.Second,
		"1:02:03": time.Hour + 2*time.Minute + 3*time.Second,
		" 0:05 ":  5 * time.Second,
	} {
		if got, err := parseTimestamp(in); err != nil || got != want {
			t.Errorf("parseTimestamp(%q) = %v, %v, want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "1:60", "1:2:3:4", "-5", "1:-1"} {
		if _, err := parseTimestamp(in); err == nil {
			t.Errorf("parseTimestamp(%q) should fail", in)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                                     "?",
		5 * time.Second:                       "0:05",
		213 * time.Second:                     "3:33",
		time.Hour + 2*time.Second:             "1:00:02",
		61*time.Minute + 500*time.Millisecond: "1:01:01",
	} {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
