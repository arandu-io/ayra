package live_test

import (
	"context"
	"encoding/json"
	"image"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arandu-io/ayra/live"
	"github.com/arandu-io/ayra/live/internal/wstest"
)

// fakeDecoder stands in for the hardware: it paints each picture a colour taken
// from its presentation time, so a test can tell which picture it is looking at
// without a codec.
type fakeDecoder struct {
	sink       live.FrameSink
	mu         sync.Mutex
	track      live.Track
	configured int
	decoded    []live.Sample
	failWith   error
	busyAt     map[int]bool
	closed     bool
}

func newFake(sink live.FrameSink) *fakeDecoder { return &fakeDecoder{sink: sink} }

func (d *fakeDecoder) Configure(t live.Track) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.track = t
	d.configured++
	return nil
}

func (d *fakeDecoder) Decode(s live.Sample) error {
	d.mu.Lock()
	n := len(d.decoded) + 1
	if d.failWith != nil {
		d.mu.Unlock()
		return d.failWith
	}
	if d.busyAt[n] {
		d.busyAt[n] = false
		d.mu.Unlock()
		return live.ErrBusy
	}
	d.decoded = append(d.decoded, s)
	track := d.track
	d.mu.Unlock()

	img := d.sink.Acquire(track.Width, track.Height)
	if img == nil {
		return live.ErrBusy
	}
	shade := uint8(s.PTS / (10 * time.Millisecond))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = shade, 0, 0, 255
	}
	d.sink.Present(img, s.PTS)
	return nil
}

func (d *fakeDecoder) Close() error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	return nil
}

func (d *fakeDecoder) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.decoded)
}

// shadeOf reads the colour the fake painted into a frame.
func shadeOf(img *image.RGBA) uint8 { return img.Pix[0] }

// recorder collects what a stream says.
type recorder struct {
	mu       sync.Mutex
	states   []live.State
	tracks   []live.Track
	samples  []live.Sample
	messages []string
	drops    []error
	renewals atomic.Int32
}

func (r *recorder) config(endpoint string, pass func(context.Context) (string, error)) live.StreamConfig {
	return live.StreamConfig{
		Endpoint: endpoint, Pass: pass, Silence: 2 * time.Second,
		OnTrack:  func(t live.Track) { r.mu.Lock(); r.tracks = append(r.tracks, t); r.mu.Unlock() },
		OnSample: func(s live.Sample) { r.mu.Lock(); r.samples = append(r.samples, s); r.mu.Unlock() },
		OnMessage: func(m json.RawMessage) {
			r.mu.Lock()
			r.messages = append(r.messages, string(m))
			r.mu.Unlock()
		},
		OnStatus: func(s live.Status) {
			r.mu.Lock()
			if len(r.states) == 0 || r.states[len(r.states)-1] != s.State {
				r.states = append(r.states, s.State)
			}
			r.mu.Unlock()
		},
		OnDrop:  func(err error) { r.mu.Lock(); r.drops = append(r.drops, err); r.mu.Unlock() },
		OnRenew: func() { r.renewals.Add(1) },
	}
}

func (r *recorder) sampleCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.samples)
}

func (r *recorder) has(state live.State) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.states {
		if s == state {
			return true
		}
	}
	return false
}

func (r *recorder) history() []live.State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]live.State(nil), r.states...)
}

// eventually waits for a condition, which is how a test reads a goroutine.
func eventually(t testing.TB, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// sendClip writes a fixture to a session the way the node does: the track
// first, then every picture, in messages of at most max bytes.
func sendClip(t testing.TB, s *wstest.Session, track live.Track, samples []live.Sample, max int) {
	t.Helper()
	if err := s.Text(wstest.TrackMessage(track.Codec, track.Width, track.Height, track.Description)); err != nil {
		t.Logf("track: %v", err)
		return
	}
	sendSamples(t, s, samples, max)
}

// sendSamples writes pictures without a track ahead of them, as the node does
// for the second and later pictures of a stream.
func sendSamples(t testing.TB, s *wstest.Session, samples []live.Sample, max int) {
	t.Helper()
	for _, sample := range samples {
		for _, piece := range wstest.SampleMessages(sample.Key, sample.Wall, sample.PTS, sample.Data, max) {
			if err := s.Binary(piece); err != nil {
				t.Logf("sample: %v", err)
				return
			}
		}
	}
}

// passes counts the passes asked for and hands out a numbered one each time.
type passes struct {
	n   atomic.Int32
	err error
}

func (p *passes) ask(context.Context) (string, error) {
	n := p.n.Add(1)
	if p.err != nil {
		return "", p.err
	}
	return "pass-" + string(rune('0'+n)), nil
}

// decoderBox holds the decoder a player opened, which exists only once the
// track has arrived.
type decoderBox struct {
	mu sync.Mutex
	d  *fakeDecoder
}

func (b *decoderBox) open(sink live.FrameSink, configure func(*fakeDecoder)) (live.Decoder, error) {
	d := newFake(sink)
	if configure != nil {
		configure(d)
	}
	b.mu.Lock()
	b.d = d
	b.mu.Unlock()
	return d, nil
}

func (b *decoderBox) get() *fakeDecoder {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.d
}
