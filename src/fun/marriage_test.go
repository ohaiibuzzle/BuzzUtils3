package fun

import (
	"testing"
	"time"
)

func TestHumanDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		30 * time.Second:              "0 minutes",
		time.Minute:                   "1 minute",
		3*time.Hour + 5*time.Minute:   "3 hours, 5 minutes",
		49*time.Hour + 30*time.Minute: "2 days, 1 hour",
		24 * time.Hour:                "1 day",
	} {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
