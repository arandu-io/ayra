package live

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultAutoPause is how long a stream keeps running while nothing draws it.
// It is the page's own figure: the connection closes five seconds after the
// picture stops being visible, and the node stops sending what nobody sees.
const DefaultAutoPause = 5 * time.Second

// PlayerConfig is what a [Player] needs.
type PlayerConfig struct {
	// Endpoint and Pass are the stream's: where the camera's socket is, and how
	// to ask the application for a pass.
	Endpoint string
	Pass     func(ctx context.Context) (string, error)
	// Header, Silence and MaxPicture are the stream's, and zero values take its
	// defaults.
	Header     http.Header
	Silence    time.Duration
	MaxPicture int

	// Decoder opens the decoder for the stream. Nil takes [NewDecoder], the
	// platform's hardware decoder.
	Decoder func(FrameSink) (Decoder, error)

	// OnMessage receives what the application attached to the feed -- the
	// boxes of the plates, for one -- as the node relayed it. It runs on the
	// stream's goroutine and must not wait.
	OnMessage func(json.RawMessage)

	// AutoPause is how long the stream keeps running when [Player.Frame] is not
	// being called. A window that is in the background, or a screen that has
	// scrolled the picture away, stops asking for frames, and after this long
	// the socket is closed; the next call to Frame opens it again. Zero takes
	// [DefaultAutoPause] and a negative value never pauses by itself.
	AutoPause time.Duration
}

// Stats counts what a player did, for a diagnostic.
type Stats struct {
	// Samples is the pictures received, Decoded the frames shown, Dropped the
	// pictures lost to a busy decoder, a malformed message or a wait for a
	// keyframe.
	Samples, Decoded, Dropped uint64
	// Renewals is the sessions the node ended at its limit and the player
	// reopened.
	Renewals uint64
}

// Player is a stream, a decoder and the frame they produce.
//
// It is the state half of a live view: the caller holds it, asks it for the
// frame each time the window is drawn, and gives the frame to the widget. It
// owns the goroutines it needs -- the socket's reader and a timer that pauses
// the stream when nothing is drawing it -- and none of them holds a frame past
// the call that produced it.
type Player struct {
	cfg    PlayerConfig
	stream *Stream
	frames *frames

	mu       sync.Mutex
	decoder  Decoder
	track    Track
	err      error
	needKey  bool
	started  bool
	stopped  bool
	autoStop chan struct{}

	autoPaused atomic.Bool
	lastDrawn  atomic.Int64 // unix nanoseconds of the last Frame call
	redraw     atomic.Pointer[func()]

	samples, decoded, dropped, renewals atomic.Uint64
}

// NewPlayer makes a player. Nothing is opened until [Player.Start].
func NewPlayer(cfg PlayerConfig) *Player {
	if cfg.AutoPause == 0 {
		cfg.AutoPause = DefaultAutoPause
	}
	if cfg.Decoder == nil {
		cfg.Decoder = NewDecoder
	}
	p := &Player{cfg: cfg, frames: newFrames()}
	p.stream = NewStream(StreamConfig{
		Endpoint: cfg.Endpoint, Pass: cfg.Pass, Header: cfg.Header,
		Silence: cfg.Silence, MaxPicture: cfg.MaxPicture,
		OnTrack:   p.onTrack,
		OnSample:  p.onSample,
		OnMessage: cfg.OnMessage,
		OnStatus:  p.onStatus,
		OnDrop:    func(error) { p.dropped.Add(1) },
		OnRenew:   func() { p.renewals.Add(1) },
	})
	return p
}

// SetRedraw sets what the player calls when there is a new picture or a new
// state to draw. It is the window's way of asking for a frame -- the context's
// Redraw -- and it can be set on every frame at no cost.
func (p *Player) SetRedraw(redraw func()) {
	if redraw == nil {
		p.redraw.Store(nil)
		return
	}
	p.redraw.Store(&redraw)
}

func (p *Player) wake() {
	if f := p.redraw.Load(); f != nil {
		(*f)()
	}
}

// Start opens the stream. It does nothing on a player that is running.
func (p *Player) Start() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	first := !p.started
	p.started = true
	if first && p.cfg.AutoPause > 0 {
		p.autoStop = make(chan struct{})
		go p.watch(p.autoStop, p.cfg.AutoPause)
	}
	p.mu.Unlock()

	p.lastDrawn.Store(time.Now().UnixNano())
	p.autoPaused.Store(false)
	p.mu.Lock()
	p.err = nil
	p.needKey = true
	p.mu.Unlock()
	p.stream.Start()
}

// Stop ends the stream and releases the decoder. The player is not used
// afterwards.
func (p *Player) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	if p.autoStop != nil {
		close(p.autoStop)
		p.autoStop = nil
	}
	p.mu.Unlock()

	p.stream.Stop()

	p.mu.Lock()
	decoder := p.decoder
	p.decoder = nil
	p.mu.Unlock()
	if decoder != nil {
		decoder.Close()
	}
}

// Pause closes the socket and keeps the last picture. [Player.Resume] or the
// next [Player.Frame] opens it again.
func (p *Player) Pause() {
	p.autoPaused.Store(false)
	p.stream.Pause()
}

// Resume opens the stream again after a pause or a failure.
func (p *Player) Resume() { p.Start() }

// Retry is [Player.Resume], named for the moment it is used: the button a
// failed stream offers.
func (p *Player) Retry() { p.Start() }

// Status answers where the stream is. A decoder that failed to start is a
// failed stream even though the socket is fine.
func (p *Player) Status() Status {
	status := p.stream.Status()
	p.mu.Lock()
	err := p.err
	p.mu.Unlock()
	if err != nil {
		status.State = StateFailed
		status.Err = err
	}
	return status
}

// Stats answers what the player has done so far.
func (p *Player) Stats() Stats {
	return Stats{
		Samples: p.samples.Load(), Decoded: p.decoded.Load(),
		Dropped: p.dropped.Load(), Renewals: p.renewals.Load(),
	}
}

// Frame answers the picture to draw now.
//
// Calling it is also how the player knows it is being watched. A window in the
// background stops drawing, so it stops calling this, and after
// [PlayerConfig.AutoPause] the socket is closed; the first call after that
// opens it again, and the last picture is returned meanwhile.
func (p *Player) Frame() Frame {
	p.lastDrawn.Store(time.Now().UnixNano())
	if p.autoPaused.CompareAndSwap(true, false) {
		p.Start()
	}
	return p.frames.next()
}

// watch pauses the stream when nothing has asked for a frame for the period.
func (p *Player) watch(stop <-chan struct{}, after time.Duration) {
	tick := max(after/4, 10*time.Millisecond)
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if p.autoPaused.Load() || p.stream.Status().State.Terminal() {
			continue
		}
		idle := time.Since(time.Unix(0, p.lastDrawn.Load()))
		if idle >= after {
			p.autoPaused.Store(true)
			p.stream.Pause()
			p.wake()
		}
	}
}

func (p *Player) onStatus(Status) { p.wake() }

// onTrack configures the decoder, opening it the first time.
func (p *Player) onTrack(track Track) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return
	}
	p.track = track
	p.needKey = true
	p.frames.reset()

	if p.decoder == nil {
		decoder, err := p.cfg.Decoder(playerSink{p})
		if err != nil {
			p.err = err
			go p.wake()
			return
		}
		p.decoder = decoder
	}
	if err := p.decoder.Configure(track); err != nil {
		p.err = err
		go p.wake()
	}
}

// onSample hands a picture to the decoder.
func (p *Player) onSample(s Sample) {
	p.samples.Add(1)

	p.mu.Lock()
	decoder := p.decoder
	failed := p.err != nil
	waiting := p.needKey
	if s.Key {
		p.needKey = false
		waiting = false
	}
	p.mu.Unlock()

	if decoder == nil || failed || waiting {
		p.dropped.Add(1)
		return
	}
	p.frames.remember(s.PTS, s.Wall)
	if err := decoder.Decode(s); err != nil {
		p.dropped.Add(1)
		if errors.Is(err, ErrBusy) {
			// A picture was lost, so the ones that follow are built on air.
			p.mu.Lock()
			p.needKey = true
			p.mu.Unlock()
			return
		}
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		p.wake()
		return
	}
}

// playerSink is what a decoder writes into: the frame buffers, with the player
// told when a picture is ready.
type playerSink struct{ p *Player }

func (s playerSink) Acquire(width, height int) *image.RGBA { return s.p.frames.Acquire(width, height) }

func (s playerSink) Present(img *image.RGBA, pts time.Duration) {
	s.p.frames.Present(img, pts)
	s.p.decoded.Add(1)
	s.p.wake()
}
