package live_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arandu-io/ayra/live"
	"github.com/arandu-io/ayra/live/internal/wstest"
)

// fakePlayer builds a player over a fake decoder and a server that plays a clip.
func fakePlayer(t *testing.T, server *wstest.Server, p *passes, autoPause time.Duration) (*live.Player, *decoderBox) {
	t.Helper()
	box := &decoderBox{}
	player := live.NewPlayer(live.PlayerConfig{
		Endpoint:  server.URL("/live/cam"),
		Pass:      p.ask,
		Silence:   2 * time.Second,
		AutoPause: autoPause,
		Decoder:   func(sink live.FrameSink) (live.Decoder, error) { return box.open(sink, nil) },
	})
	t.Cleanup(player.Stop)
	return player, box
}

func TestAPlayerShowsThePicturesTheNodeSends(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples, 500)
		time.Sleep(time.Second)
	})
	var p passes
	var redraws atomic.Int32
	player, _ := fakePlayer(t, server, &p, -1)
	player.SetRedraw(func() { redraws.Add(1) })
	player.Start()

	eventually(t, "the last picture", func() bool {
		f := player.Frame()
		return f.Image != nil && shadeOf(f.Image) == uint8(samples[len(samples)-1].PTS/(10*time.Millisecond))
	})
	f := player.Frame()
	if f.Size().X != track.Width || f.Size().Y != track.Height {
		t.Errorf("the frame is %v, want %dx%d", f.Size(), track.Width, track.Height)
	}
	if f.Wall.IsZero() || !f.Wall.Equal(samples[len(samples)-1].Wall) {
		t.Errorf("the frame's wall clock is %v, want %v", f.Wall, samples[len(samples)-1].Wall)
	}
	if redraws.Load() == 0 {
		t.Error("the window was never asked for a frame")
	}
	stats := player.Stats()
	if stats.Samples != uint64(len(samples)) || stats.Decoded == 0 {
		t.Errorf("stats %+v", stats)
	}
	if player.Status().State != live.StateLive {
		t.Errorf("the player is %v", player.Status().State)
	}
}

// TestAFrameDoesNotChangeUnderTheWindow is the promise the engine depends on:
// it reads the picture again when the frame is rasterised, after layout, so the
// buffer handed out is not written to until the window asks for the next.
func TestAFrameDoesNotChangeUnderTheWindow(t *testing.T) {
	track, samples := load(t, "h264")
	release := make(chan struct{})
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples[:2], 4096)
		<-release
		sendSamples(t, s, samples[2:], 4096)
		time.Sleep(time.Second)
	})
	var p passes
	player, _ := fakePlayer(t, server, &p, -1)
	player.Start()

	var held live.Frame
	eventually(t, "the second picture", func() bool {
		held = player.Frame()
		return held.Image != nil && held.PTS == samples[1].PTS
	})
	shade, seq := shadeOf(held.Image), held.Seq
	close(release)

	eventually(t, "the rest of the clip", func() bool { return player.Stats().Decoded == uint64(len(samples)) })
	if shadeOf(held.Image) != shade {
		t.Fatal("the buffer the window holds was written to")
	}

	next := player.Frame()
	if next.Seq <= seq || shadeOf(next.Image) != uint8(samples[len(samples)-1].PTS/(10*time.Millisecond)) {
		t.Errorf("the next frame is %d (shade %d), want the newest picture", next.Seq, shadeOf(next.Image))
	}
	again := player.Frame()
	if again.Seq != next.Seq || again.Image != next.Image {
		t.Error("asking again without a new picture returned a different frame")
	}
}

func TestAPlayerWaitsForAKeyframeAfterADroppedPicture(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples, 4096)
		time.Sleep(time.Second)
	})
	var p passes
	box := &decoderBox{}
	player := live.NewPlayer(live.PlayerConfig{
		Endpoint: server.URL("/live/cam"), Pass: p.ask, AutoPause: -1,
		Decoder: func(sink live.FrameSink) (live.Decoder, error) {
			// The third picture is refused as busy.
			return box.open(sink, func(d *fakeDecoder) { d.busyAt = map[int]bool{3: true} })
		},
	})
	t.Cleanup(player.Stop)
	player.Start()

	eventually(t, "the clip", func() bool { return player.Stats().Samples == uint64(len(samples)) })
	eventually(t, "the last picture decoded", func() bool { return box.get().count() >= 8 })

	// The keyframes are at 0 and 6. The picture refused is the third (PTS 200 ms);
	// the ones after it, up to the keyframe, are built on it and must not be
	// decoded.
	d := box.get()
	d.mu.Lock()
	defer d.mu.Unlock()
	var got []time.Duration
	for _, s := range d.decoded {
		got = append(got, s.PTS)
		if !s.Key && s.PTS > samples[2].PTS && s.PTS < samples[6].PTS {
			t.Errorf("the picture at %v was decoded though its reference was lost", s.PTS)
		}
	}
	if len(d.decoded) != 8 {
		t.Errorf("%d pictures decoded (%v), want 8: 0..1 before the loss, then the keyframe at 6 and what follows", len(d.decoded), got)
	}
}

func TestADecoderThatDoesNotExistIsAFailedStreamWithItsReason(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples, 4096)
		time.Sleep(500 * time.Millisecond)
	})
	var p passes
	player := live.NewPlayer(live.PlayerConfig{
		Endpoint: server.URL("/live/cam"), Pass: p.ask, AutoPause: -1,
		Decoder: func(live.FrameSink) (live.Decoder, error) { return nil, live.ErrUnsupported },
	})
	t.Cleanup(player.Stop)
	player.Start()

	eventually(t, "the failure", func() bool { return player.Status().State == live.StateFailed })
	if err := player.Status().Err; !errors.Is(err, live.ErrUnsupported) {
		t.Errorf("failed with %v, want ErrUnsupported", err)
	}
}

func TestADecoderThatFailsMidStreamFailsThePlayer(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples, 4096)
		time.Sleep(500 * time.Millisecond)
	})
	boom := errors.New("codec crashed")
	var p passes
	player := live.NewPlayer(live.PlayerConfig{
		Endpoint: server.URL("/live/cam"), Pass: p.ask, AutoPause: -1,
		Decoder: func(sink live.FrameSink) (live.Decoder, error) {
			d := newFake(sink)
			d.failWith = boom
			return d, nil
		},
	})
	t.Cleanup(player.Stop)
	player.Start()

	eventually(t, "the failure", func() bool { return player.Status().State == live.StateFailed })
	if err := player.Status().Err; !errors.Is(err, boom) {
		t.Errorf("failed with %v", err)
	}
}

// TestAPlayerNobodyDrawsPausesAndResumesWhenDrawnAgain is the background rule:
// a window that stops drawing stops asking for frames, and the stream closes.
func TestAPlayerNobodyDrawsPausesAndResumesWhenDrawnAgain(t *testing.T) {
	track, samples := load(t, "h264")
	var sessions atomic.Int32
	server := wstest.New(t, func(s *wstest.Session) {
		sessions.Add(1)
		sendClip(t, s, track, samples[:3], 4096)
		s.Read(5 * time.Second) // until the client leaves
	})
	var p passes
	player, _ := fakePlayer(t, server, &p, 200*time.Millisecond)
	player.Start()

	// Drawn: it stays open past the pause period.
	until := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(until) {
		player.Frame()
		time.Sleep(20 * time.Millisecond)
	}
	if st := player.Status().State; st == live.StatePaused {
		t.Fatalf("a stream that was being drawn paused")
	}

	// Not drawn: it pauses.
	eventually(t, "the pause", func() bool { return player.Status().State == live.StatePaused })

	// The last picture is still there to draw.
	if f := player.Frame(); f.Image == nil && sessions.Load() == 1 {
		// Drawing again resumes, so this call itself reopened the stream.
		t.Log("resumed by the draw")
	}
	eventually(t, "the second session", func() bool { return sessions.Load() == 2 })
	if p.n.Load() != 2 {
		t.Errorf("%d passes asked for; the resume needs a new one", p.n.Load())
	}
}

func TestAnExplicitPauseIsNotUndoneByDrawing(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples[:2], 4096)
		s.Read(2 * time.Second)
	})
	var p passes
	player, _ := fakePlayer(t, server, &p, -1)
	player.Start()
	eventually(t, "a picture", func() bool { return player.Frame().Image != nil })

	player.Pause()
	for i := 0; i < 5; i++ {
		player.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if player.Status().State != live.StatePaused {
		t.Errorf("drawing resumed a stream that was paused on purpose: %v", player.Status().State)
	}
	if player.Frame().Image == nil {
		t.Error("the last picture was lost with the pause")
	}

	player.Resume()
	eventually(t, "the stream back", func() bool { return len(server.Requests()) == 2 })
}

func TestAStoppedPlayerReleasesItsDecoder(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples[:2], 4096)
		time.Sleep(time.Second)
	})
	var p passes
	player, box := fakePlayer(t, server, &p, -1)
	player.Start()
	eventually(t, "a picture", func() bool { return player.Frame().Image != nil })

	player.Stop()
	fake := box.get()
	fake.mu.Lock()
	closed := fake.closed
	fake.mu.Unlock()
	if !closed {
		t.Error("the decoder was not closed")
	}
	player.Start() // a stopped player stays stopped
	time.Sleep(50 * time.Millisecond)
	if len(server.Requests()) != 1 {
		t.Error("a stopped player opened a socket")
	}
}

func TestANewTrackReconfiguresTheSameDecoder(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples[:3], 4096)
		sendClip(t, s, track, samples[6:9], 4096) // the camera's stream started over
		time.Sleep(time.Second)
	})
	var p passes
	player, box := fakePlayer(t, server, &p, -1)
	player.Start()

	eventually(t, "six pictures", func() bool { return player.Stats().Decoded == 6 })
	d := box.get()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.configured != 2 {
		t.Errorf("the decoder was configured %d times, want 2", d.configured)
	}
}
