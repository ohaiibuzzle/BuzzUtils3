package bot

import (
	"testing"
	"time"
)

func TestIdentifyBackoffNextDelay(t *testing.T) {
	var b identifyBackoff
	now := time.Now()

	want := []time.Duration{
		0,
		5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second,
		80 * time.Second, 160 * time.Second, 5 * time.Minute, 5 * time.Minute,
	}
	for i, w := range want {
		got := b.nextDelay(now)
		if got != w {
			t.Fatalf("login %d: got %s, want %s", i, got, w)
		}
		// Each connection dies a few seconds after it logs in
		now = now.Add(got + 3*time.Second)
	}

	// The sleep itself must not count as uptime, or the cap would reset itself
	if got := b.nextDelay(now.Add(identifyMaxDelay)); got != identifyMaxDelay {
		t.Fatalf("after a short connection: got %s, want %s", got, identifyMaxDelay)
	}

	if got := b.nextDelay(b.last.Add(identifyStableAfter)); got != 0 {
		t.Fatalf("after a stable connection: got %s, want 0", got)
	}
}
