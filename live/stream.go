package live

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/arandu-io/ayra/live/internal/ws"
)

// State is where a stream is, from the viewer's side.
type State int

// The states of a stream.
const (
	// StateIdle is a stream that was never started, or was stopped.
	StateIdle State = iota
	// StateConnecting is a pass being asked for or a socket being opened.
	StateConnecting
	// StateWaiting is a connected stream whose camera has no picture yet.
	StateWaiting
	// StateStarting is a camera whose picture is being converted for the
	// viewer.
	StateStarting
	// StateLive is a stream whose pictures are arriving.
	StateLive
	// StateLost is a camera that had a picture and lost it. The socket stays
	// open, and the state returns to [StateLive] when the picture does.
	StateLost
	// StateBusy is a node that cannot convert the camera now.
	StateBusy
	// StateFull is a camera that has all the viewers it allows.
	StateFull
	// StatePaused is a stream closed on purpose, by the caller or because
	// nothing was drawing it.
	StatePaused
	// StateFailed is a stream that ended with an error, in [Status.Err].
	StateFailed
)

// String names the state for a diagnostic and for a test failure.
func (s State) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateWaiting:
		return "waiting"
	case StateStarting:
		return "starting"
	case StateLive:
		return "live"
	case StateLost:
		return "lost"
	case StateBusy:
		return "busy"
	case StateFull:
		return "full"
	case StatePaused:
		return "paused"
	case StateFailed:
		return "failed"
	}
	return "idle"
}

// Terminal reports whether the stream stays in this state until the caller
// asks for something: a retry after a failure, a resume after a pause. The
// other states change by themselves.
func (s State) Terminal() bool {
	switch s {
	case StateIdle, StateBusy, StateFull, StatePaused, StateFailed:
		return true
	}
	return false
}

// Errors a failed stream carries.
var (
	// ErrRefused is a node that refused the pass. The application's own answer
	// is what to do next: a pass is short-lived and a new one is asked for.
	ErrRefused = errors.New("live: the node refused the pass")
	// ErrSilent is a socket that sent nothing for longer than the silence
	// limit. The node sends a keepalive every few seconds, so this is a
	// connection that is gone and has not said so.
	ErrSilent = errors.New("live: the node sent nothing for too long")
	// ErrEnded is a socket the node closed normally: the camera was removed or
	// the node is stopping.
	ErrEnded = errors.New("live: the node ended the stream")
	// ErrRenewing is a node that keeps ending sessions at once, which a viewer
	// cannot fix by asking again.
	ErrRenewing = errors.New("live: the node keeps ending the session as soon as it opens")
)

// Status is what a viewer is told about a stream.
type Status struct {
	State State
	// Err is why the stream failed, for [StateFailed].
	Err error
	// Track is the configuration of the picture, once known.
	Track Track
}

// DefaultSilence is how long a stream waits for any message before it is
// called lost. The node sends one every 15 seconds.
const DefaultSilence = 45 * time.Second

// StreamConfig is what a stream needs to reach a camera.
//
// The handlers run on the stream's reader goroutine, one message at a time and
// in order, and they must not wait: a handler that blocks holds the socket, and
// the node answers a viewer that falls behind by skipping to the next keyframe.
type StreamConfig struct {
	// Endpoint is the address of the camera's socket without the pass:
	// wss://api1.alpr.sh/live/{camera}.
	Endpoint string
	// Pass asks the application for a pass. It runs before the first
	// connection and again each time the node ends a session, which is how a
	// session that expired is renewed without the viewer noticing. Its context
	// ends when the stream is stopped.
	Pass func(ctx context.Context) (string, error)

	// Header is added to the upgrade request. The node compares Origin against
	// the pages it allows, so an application that was given an origin sets it
	// here.
	Header http.Header
	// TLS configures a wss connection. Nil takes the defaults.
	TLS *tls.Config
	// Silence is how long to wait for any message. Zero takes [DefaultSilence].
	Silence time.Duration
	// MaxPicture bounds one assembled picture. Zero takes [DefaultMaxPicture].
	MaxPicture int
	// Dial replaces how the connection is opened, for a test.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)

	// OnTrack is called when the decoder's configuration arrives.
	OnTrack func(Track)
	// OnSample is called with every complete picture.
	OnSample func(Sample)
	// OnMessage is called with a text message the node relayed from the
	// application's own feed, such as the boxes drawn over the picture.
	OnMessage func(json.RawMessage)
	// OnStatus is called after every change of state, outside the stream's
	// lock.
	OnStatus func(Status)
	// OnRenew is called each time the node ended a session at its limit and the
	// stream is about to open another.
	OnRenew func()
	// OnDrop is called with the error of a message that could not be read and
	// was skipped; the stream goes on.
	OnDrop func(error)
}

// Stream is the connection to one camera: it asks for a pass, opens the socket,
// reads the messages and renews the session when the node ends it.
//
// It owns one goroutine while it runs and holds no picture: whatever it reads
// goes to the handlers and is gone.
type Stream struct {
	cfg StreamConfig

	mu     sync.Mutex
	status Status
	run    *run
}

type run struct {
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
	conn   *ws.Conn
}

// NewStream makes a stream. Nothing is opened until [Stream.Start].
func NewStream(cfg StreamConfig) *Stream {
	if cfg.Silence <= 0 {
		cfg.Silence = DefaultSilence
	}
	return &Stream{cfg: cfg}
}

// Status answers where the stream is.
func (s *Stream) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Start opens the stream. It does nothing while one is running.
func (s *Stream) Start() {
	s.mu.Lock()
	if s.run != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{cancel: cancel, done: make(chan struct{})}
	s.run = r
	s.mu.Unlock()

	s.set(r, Status{State: StateConnecting})
	go s.loop(ctx, r)
}

// Retry opens the stream again after it failed, was refused a place or was
// paused. Called on a stream that is running it does nothing.
func (s *Stream) Retry() { s.Start() }

// Pause closes the socket and keeps the state as [StatePaused]. [Stream.Start]
// opens it again.
func (s *Stream) Pause() { s.end(StatePaused) }

// Stop closes the socket and leaves the stream idle.
func (s *Stream) Stop() { s.end(StateIdle) }

// end stops the running goroutine and waits for it, so that when it returns no
// handler is running and none will.
func (s *Stream) end(to State) {
	s.mu.Lock()
	r := s.run
	s.run = nil
	if r == nil {
		// Nothing is running, so there is nothing to end: a stream that failed
		// stays failed, which is the more useful thing to read.
		s.mu.Unlock()
		return
	}
	status := Status{State: to, Track: s.status.Track}
	s.status = status
	s.mu.Unlock()

	r.cancel()
	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close(ws.CloseGoingAway, "")
	}
	r.mu.Unlock()
	<-r.done

	if s.cfg.OnStatus != nil {
		s.cfg.OnStatus(status)
	}
}

// set records a new status if the run is still the current one, and reports
// it.
func (s *Stream) set(r *run, status Status) {
	s.mu.Lock()
	if s.run != r {
		s.mu.Unlock()
		return
	}
	if status.Track.Codec == "" {
		status.Track = s.status.Track
	}
	changed := status.State != s.status.State || status.Err != nil || status.Track.Codec != s.status.Track.Codec
	s.status = status
	s.mu.Unlock()

	if changed && s.cfg.OnStatus != nil {
		s.cfg.OnStatus(status)
	}
}

// finish ends the run on its own: the state is terminal, and a later Start
// opens a new one.
func (s *Stream) finish(r *run, status Status) {
	s.mu.Lock()
	if s.run != r {
		s.mu.Unlock()
		return
	}
	s.run = nil
	if status.Track.Codec == "" {
		status.Track = s.status.Track
	}
	s.status = status
	s.mu.Unlock()
	if s.cfg.OnStatus != nil {
		s.cfg.OnStatus(status)
	}
}

// renewalFloor is how long a session must have lasted for the end of it to be
// the node's limit and not a refusal in disguise.
const renewalFloor = 2 * time.Second

// maxQuickRenewals is how many sessions in a row may end sooner than
// renewalFloor before the stream gives up.
const maxQuickRenewals = 4

func (s *Stream) loop(ctx context.Context, r *run) {
	defer close(r.done)

	quick := 0
	for ctx.Err() == nil {
		started := time.Now()
		outcome := s.session(ctx, r)
		if ctx.Err() != nil {
			return
		}
		switch {
		case outcome.renew:
			if s.cfg.OnRenew != nil {
				s.cfg.OnRenew()
			}
			if time.Since(started) < renewalFloor {
				quick++
				if quick >= maxQuickRenewals {
					s.finish(r, Status{State: StateFailed, Err: ErrRenewing})
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(quick) * 250 * time.Millisecond):
				}
			} else {
				quick = 0
			}
			// The picture on screen stays while the next session opens.
			continue
		case outcome.status.State != StateIdle:
			s.finish(r, outcome.status)
			return
		}
	}
}

type outcome struct {
	renew  bool
	status Status
}

// session is one pass and one socket.
func (s *Stream) session(ctx context.Context, r *run) outcome {
	if s.Status().State != StateLive {
		s.set(r, Status{State: StateConnecting})
	}
	pass, err := s.cfg.Pass(ctx)
	if err != nil {
		return outcome{status: Status{State: StateFailed, Err: fmt.Errorf("live: asking for a pass: %w", err)}}
	}
	address, err := withPass(s.cfg.Endpoint, pass)
	if err != nil {
		return outcome{status: Status{State: StateFailed, Err: err}}
	}

	opts := ws.Options{Header: s.cfg.Header, TLS: s.cfg.TLS, MaxMessage: s.maxMessage()}
	opts.Dial = s.cfg.Dial
	conn, err := ws.Dial(ctx, address, opts)
	if err != nil {
		var refused *ws.HandshakeError
		if errors.As(err, &refused) && refused.Status == http.StatusUnauthorized {
			return outcome{status: Status{State: StateFailed, Err: ErrRefused}}
		}
		return outcome{status: Status{State: StateFailed, Err: err}}
	}
	r.mu.Lock()
	if ctx.Err() != nil {
		r.mu.Unlock()
		conn.Close(ws.CloseGoingAway, "")
		return outcome{}
	}
	r.conn = conn
	r.mu.Unlock()
	defer conn.Close(ws.CloseGoingAway, "")

	return s.read(ctx, r, conn)
}

// maxMessage is the largest single message a socket may carry. The node never
// writes more than 128 KiB, so a message far above that is not the node.
func (s *Stream) maxMessage() int {
	max := s.cfg.MaxPicture
	if max <= 0 {
		max = DefaultMaxPicture
	}
	return max + SampleHeader
}

// read is the reader loop of one socket.
func (s *Stream) read(ctx context.Context, r *run, conn *ws.Conn) outcome {
	parser := Parser{Max: s.cfg.MaxPicture}
	signalled := StateConnecting
	if s.Status().State == StateConnecting {
		s.set(r, Status{State: StateWaiting})
	}

	for {
		kind, message, err := conn.Read(s.cfg.Silence)
		if err != nil {
			if ctx.Err() != nil {
				return outcome{}
			}
			var closed *ws.CloseError
			switch {
			case errors.As(err, &closed):
				switch {
				case closed.Code == StatusSessionEnded:
					return outcome{renew: true}
				case signalled == StateBusy || signalled == StateFull:
					return outcome{status: Status{State: signalled}}
				case closed.Code == ws.CloseNormal:
					return outcome{status: Status{State: StateFailed, Err: ErrEnded}}
				}
				return outcome{status: Status{State: StateFailed, Err: err}}
			case ws.IsTimeout(err):
				return outcome{status: Status{State: StateFailed, Err: ErrSilent}}
			}
			return outcome{status: Status{State: StateFailed, Err: err}}
		}

		switch kind {
		case ws.Text:
			event, err := parser.Text(message)
			if err != nil {
				s.drop(err)
				continue
			}
			switch event.Kind {
			case KindTrack:
				s.set(r, Status{State: s.Status().State, Track: event.Track})
				if s.cfg.OnTrack != nil {
					s.cfg.OnTrack(event.Track)
				}
			case KindSignal:
				state, known := signalState(event.Signal)
				if !known {
					s.drop(fmt.Errorf("live: unknown signal %q", event.Signal))
					continue
				}
				if state == StateBusy || state == StateFull {
					signalled = state
				}
				s.set(r, Status{State: state})
			case KindApplication:
				if s.cfg.OnMessage != nil {
					s.cfg.OnMessage(event.Raw)
				}
			}
		case ws.Binary:
			event, done, err := parser.Binary(message)
			if err != nil {
				s.drop(err)
			}
			if !done {
				continue
			}
			if s.Status().State != StateLive {
				s.set(r, Status{State: StateLive})
			}
			if s.cfg.OnSample != nil {
				s.cfg.OnSample(event.Sample)
			}
		}
	}
}

func (s *Stream) drop(err error) {
	if s.cfg.OnDrop != nil {
		s.cfg.OnDrop(err)
	}
}

func signalState(signal Signal) (State, bool) {
	switch signal {
	case SignalWaiting:
		return StateWaiting, true
	case SignalStarting:
		return StateStarting, true
	case SignalLost:
		return StateLost, true
	case SignalBusy:
		return StateBusy, true
	case SignalFull:
		return StateFull, true
	}
	return StateIdle, false
}

// withPass puts the pass in the query of the endpoint, escaped.
func withPass(endpoint, pass string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("live: %q is not an address", endpoint)
	}
	if !strings.HasPrefix(u.Scheme, "ws") && !strings.HasPrefix(u.Scheme, "http") {
		return "", fmt.Errorf("live: %q is not a ws or wss address", endpoint)
	}
	q := u.Query()
	q.Set("pass", pass)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
