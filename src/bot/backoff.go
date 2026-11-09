package bot

import (
	"context"
	"errors"
	"log"
	"time"
)

const (
	identifyMinDelay = 5 * time.Second
	// Caps logins at 288 a day, well under the 1000 that gets a bot's token reset
	identifyMaxDelay = 5 * time.Minute
	// A login this long after the previous one means that connection was healthy
	identifyStableAfter = 10 * time.Minute
)

// identifyBackoff spaces out gateway logins. disgo's own backoff starts over after every
// successful connect, so a connection that dies right after READY would log in forever.
// disgo only calls Wait while holding its connection lock, so this needs no lock of its own.
type identifyBackoff struct {
	last  time.Time
	delay time.Duration
	stop  chan struct{}
}

// nextDelay returns how long to hold a login that is being attempted at now.
func (b *identifyBackoff) nextDelay(now time.Time) time.Duration {
	switch {
	case b.last.IsZero() || now.Sub(b.last) >= identifyStableAfter:
		b.delay = 0
	case b.delay == 0:
		b.delay = identifyMinDelay
	default:
		b.delay = min(b.delay*2, identifyMaxDelay)
	}
	b.last = now.Add(b.delay)
	return b.delay
}

func (b *identifyBackoff) Wait(ctx context.Context, _ int) error {
	delay := b.nextDelay(time.Now())
	if delay == 0 {
		return nil
	}
	log.Printf("Logging in to the gateway repeatedly, waiting %s first", delay)
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-b.stop:
		return errors.New("bot is shutting down")
	}
}

func (b *identifyBackoff) Unlock(int) {}

// Close interrupts a Wait in progress, which would otherwise block closing the gateway.
func (b *identifyBackoff) Close(context.Context) {
	close(b.stop)
}
