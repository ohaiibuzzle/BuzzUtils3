package music

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
)

const (
	frameDuration = 20 * time.Millisecond
	idleTimeout   = 3 * time.Minute
	checkInterval = 15 * time.Second
	defaultVolume = 50
)

var (
	playersMu sync.Mutex
	players   = map[snowflake.ID]*Player{}
	joining   sync.Map // guild ID -> *sync.Mutex, so two commands don't both connect
)

// Player plays a server's queue into its voice connection. Track changes happen on
// the player's own goroutine (run); disgo's audio sender pulls frames through
// ProvideOpusFrame every 20ms.
type Player struct {
	client   *bot.Client
	guildID  snowflake.ID
	conn     voice.Conn
	summoner snowflake.ID

	mu          sync.Mutex
	queue       []*Track
	current     *Track
	stream      *stream
	offset      time.Duration // where in the track the stream started
	frames      int           // frames played since then
	paused      bool
	loop        bool
	queueLoop   bool
	volume      int
	skips       int  // bumped by skip, so a track that was skipped while loading is dropped
	skipped     bool // the current track was skipped, so don't loop it
	idleSince   time.Time
	textChannel snowflake.ID // where to say goodbye when leaving on our own

	wake chan struct{}
	quit chan struct{}
	once sync.Once
}

func getPlayer(guildID snowflake.ID) *Player {
	playersMu.Lock()
	defer playersMu.Unlock()
	return players[guildID]
}

// getOrConnect returns the server's player, or joins the voice channel and starts
// an empty player there. created reports whether it was just created.
func getOrConnect(client *bot.Client, guildID, channelID, summoner, textChannel snowflake.ID) (p *Player, created bool, err error) {
	mu, _ := joining.LoadOrStore(guildID, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	if p := getPlayer(guildID); p != nil {
		if p.ChannelID() != 0 {
			return p, false, nil
		}
		// Kicked from the channel, but not cleaned up yet
		p.leave()
	}
	p, err = connect(client, guildID, channelID, summoner, textChannel)
	return p, err == nil, err
}

func connect(client *bot.Client, guildID, channelID, summoner, textChannel snowflake.ID) (*Player, error) {
	conn := client.VoiceManager.CreateConn(guildID)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Deafened, as the bot never listens
	if err := conn.Open(ctx, channelID, false, true); err != nil {
		conn.Close(context.Background())
		return nil, err
	}

	p := &Player{
		client:      client,
		guildID:     guildID,
		conn:        conn,
		summoner:    summoner,
		volume:      defaultVolume,
		idleSince:   time.Now(),
		textChannel: textChannel,
		wake:        make(chan struct{}, 1),
		quit:        make(chan struct{}),
	}
	playersMu.Lock()
	players[guildID] = p
	playersMu.Unlock()

	conn.SetOpusFrameProvider(p)
	go p.run()
	return p, nil
}

// ChannelID returns the voice channel the bot is in, or 0 if it was disconnected.
func (p *Player) ChannelID() snowflake.ID {
	if id := p.conn.ChannelID(); id != nil {
		return *id
	}
	return 0
}

// ProvideOpusFrame implements voice.OpusFrameProvider.
func (p *Player) ProvideOpusFrame() ([]byte, error) {
	p.mu.Lock()
	s := p.stream
	if p.paused || s == nil {
		p.mu.Unlock()
		return nil, nil
	}
	p.mu.Unlock()

	// Read without the lock, as ffmpeg may be waiting on the network
	frame, err := s.next()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream != s {
		return nil, nil // skipped or seeked meanwhile
	}
	if err != nil {
		if !errors.Is(err, io.EOF) {
			log.Default().Println("Error reading audio stream: " + err.Error())
		}
		s.close()
		p.stream = nil
		p.signal()
		return nil, nil
	}
	p.frames++
	return frame, nil
}

// Close implements voice.OpusFrameProvider; the player is shut down by leave.
func (p *Player) Close() {}

func (p *Player) signal() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *Player) run() {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.quit:
			return
		case <-p.wake:
			p.advance()
		case <-ticker.C:
			if reason := p.leaveReason(); reason != "" {
				if reason != "disconnected" {
					p.client.Rest.CreateMessage(p.textChannel, discord.MessageCreate{Content: reason})
				}
				p.leave()
				return
			}
		}
	}
}

// advance starts the next track once the current one has finished.
func (p *Player) advance() {
	p.mu.Lock()
	if p.stream != nil {
		p.mu.Unlock()
		return
	}
	next := p.current
	replay := next != nil && p.loop && !p.skipped
	p.skipped = false
	if !replay {
		if next != nil && p.queueLoop {
			p.queue = append(p.queue, next)
		}
		next = nil
		if len(p.queue) > 0 {
			next, p.queue = p.queue[0], p.queue[1:]
		}
	}
	p.current = next
	if next == nil {
		p.idleSince = time.Now()
		p.mu.Unlock()
		return
	}
	skips, volume := p.skips, p.volume
	p.mu.Unlock()

	// Looking the track up is slow, so it happens without the lock, on a copy
	// that commands showing the queue won't be reading
	resolved := *next
	err := resolved.resolve(context.Background())
	var s *stream
	if err == nil {
		s, err = newStream(resolved.streamURL, 0, float64(volume)/100)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.quit:
		if s != nil {
			s.close()
		}
		return
	default:
	}
	if err == nil {
		*next = resolved
	}
	if err != nil {
		log.Default().Println("Error starting track: " + err.Error())
		p.client.Rest.CreateMessage(next.TextChannel, discord.MessageCreate{
			Content:         "I couldn't play **" + next.Title + "**, skipping it :(",
			AllowedMentions: &discord.AllowedMentions{},
		})
		// Drop it, so looping doesn't retry it forever
		p.current = nil
		p.signal()
		return
	}
	if p.skips != skips {
		s.close()
		p.signal()
		return
	}
	p.stream, p.offset, p.frames = s, 0, 0
	if !replay {
		announcement := discord.MessageCreate{
			Embeds:          []discord.Embed{next.embed("Now Playing", 0)},
			AllowedMentions: &discord.AllowedMentions{},
		}
		go p.client.Rest.CreateMessage(next.TextChannel, announcement)
	}
}

// leaveReason says why the player should leave on its own, or "" to stay.
func (p *Player) leaveReason() string {
	channelID := p.ChannelID()
	if channelID == 0 {
		return "disconnected"
	}
	alone := true
	for state := range p.client.Caches.VoiceStates(p.guildID) {
		if state.UserID != p.client.ID() && state.ChannelID != nil && *state.ChannelID == channelID {
			alone = false
			break
		}
	}
	if alone {
		return "Everyone left, so I did too 👋"
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil && p.stream == nil && time.Since(p.idleSince) > idleTimeout {
		return "Nothing's been playing for a while, so I left 👋"
	}
	return ""
}

// leave stops playback and disconnects. It is safe to call more than once.
func (p *Player) leave() {
	p.once.Do(func() {
		close(p.quit)
		p.mu.Lock()
		if p.stream != nil {
			p.stream.close()
			p.stream = nil
		}
		p.queue, p.current = nil, nil
		p.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.conn.Close(ctx)

		playersMu.Lock()
		if players[p.guildID] == p {
			delete(players, p.guildID)
		}
		playersMu.Unlock()
	})
}

// enqueue adds tracks to the queue and returns the position of the first one
// (0 if it plays right away).
func (p *Player) enqueue(tracks ...*Track) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	position := len(p.queue) + 1
	if p.current == nil {
		position = 0
	}
	p.queue = append(p.queue, tracks...)
	p.signal()
	return position
}

// skip drops the current track and the next n-1 queued ones.
func (p *Player) skip(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n = min(n-1, len(p.queue))
	p.queue = p.queue[n:]
	p.skips++
	p.skipped = true
	if p.stream != nil {
		p.stream.close()
		p.stream = nil
	}
	p.signal()
}

// remove deletes the queued track at a 1-based position.
func (p *Player) remove(position int) (Track, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if position < 1 || position > len(p.queue) {
		return Track{}, false
	}
	t := *p.queue[position-1]
	p.queue = append(p.queue[:position-1], p.queue[position:]...)
	return t, true
}

func (p *Player) setPaused(paused bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil || p.paused == paused {
		return false
	}
	p.paused = paused
	return true
}

func (p *Player) toggleLoop(queue bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if queue {
		p.queueLoop = !p.queueLoop
		return p.queueLoop
	}
	p.loop = !p.loop
	return p.loop
}

// seek restarts the current track at the given position.
func (p *Player) seek(to time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.restart(to)
}

func (p *Player) setVolume(volume int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = volume
	if p.stream == nil {
		return nil
	}
	return p.restart(p.position())
}

// restart replaces the stream of the current track. The caller holds the lock.
func (p *Player) restart(at time.Duration) error {
	if p.current == nil || p.stream == nil {
		return errNothingPlaying
	}
	s, err := newStream(p.current.streamURL, at, float64(p.volume)/100)
	if err != nil {
		return err
	}
	p.stream.close()
	p.stream, p.offset, p.frames = s, at, 0
	return nil
}

// position is how far into the current track playback is. The caller holds the lock.
func (p *Player) position() time.Duration {
	return p.offset + time.Duration(p.frames)*frameDuration
}

var errNothingPlaying = errors.New("nothing is playing")

// snapshot is a copy of the player's state for display.
type snapshot struct {
	current   *Track // nil when nothing is playing
	position  time.Duration
	queue     []Track
	paused    bool
	loop      bool
	queueLoop bool
	volume    int
}

func (p *Player) snapshot() snapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	snap := snapshot{
		position:  p.position(),
		paused:    p.paused,
		loop:      p.loop,
		queueLoop: p.queueLoop,
		volume:    p.volume,
	}
	if p.current != nil {
		current := *p.current
		snap.current = &current
	}
	for _, t := range p.queue {
		snap.queue = append(snap.queue, *t)
	}
	return snap
}
