package live_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra/live"
	"github.com/arandu-io/ayra/live/internal/wstest"
)

func TestAStreamReadsTheTrackAndEveryPictureInOrder(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples, 300) // small enough that a keyframe comes in pieces
		time.Sleep(time.Second)
	})

	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam-7"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "every picture", func() bool { return rec.sampleCount() == len(samples) })

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.tracks) != 1 || rec.tracks[0].Codec != track.Codec || rec.tracks[0].Width != track.Width {
		t.Errorf("tracks: %+v", rec.tracks)
	}
	for i, got := range rec.samples {
		if got.PTS != samples[i].PTS || got.Key != samples[i].Key || string(got.Data) != string(samples[i].Data) {
			t.Fatalf("picture %d differs from the one sent", i)
		}
	}
	if want := []live.State{live.StateConnecting, live.StateWaiting, live.StateLive}; !slices.Equal(rec.states, want) {
		t.Errorf("states %v, want %v", rec.states, want)
	}
	if len(rec.drops) != 0 {
		t.Errorf("drops: %v", rec.drops)
	}

	req := server.Requests()[0]
	if req.URL.Path != "/live/cam-7" || req.URL.Query().Get("pass") != "pass-1" {
		t.Errorf("the request was %s", req.URL)
	}
}

func TestTheOriginReachesTheNode(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) { time.Sleep(200 * time.Millisecond) })
	var rec recorder
	var p passes
	cfg := rec.config(server.URL("/live/x"), p.ask)
	cfg.Header = map[string][]string{"Origin": {"https://alpr.sh"}}
	stream := live.NewStream(cfg)
	stream.Start()
	defer stream.Stop()

	eventually(t, "the request", func() bool { return len(server.Requests()) == 1 })
	if got := server.Requests()[0].Header.Get("Origin"); got != "https://alpr.sh" {
		t.Errorf("origin %q", got)
	}
}

// TestAnEndedSessionIsRenewedWithANewPass is the 4001 close. The viewer sees
// nothing: no failure, no flicker back to connecting, and the pictures go on.
func TestAnEndedSessionIsRenewedWithANewPass(t *testing.T) {
	track, samples := load(t, "h264")
	var opened int
	server := wstest.New(t, func(s *wstest.Session) {
		opened++
		sendClip(t, s, track, samples[:3], 4096)
		if opened == 1 {
			// A session that lasted, so it is not a refusal in disguise; the node's
			// keepalive is what holds the socket open meanwhile.
			for i := 0; i < 5; i++ {
				time.Sleep(450 * time.Millisecond)
				s.Text(`{"type":"ping"}`)
			}
			s.Close(live.StatusSessionEnded, "session ended")
			return
		}
		time.Sleep(time.Second)
	})

	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "pictures from two sessions", func() bool { return rec.sampleCount() >= 6 })

	if p.n.Load() != 2 {
		t.Errorf("%d passes were asked for, want 2", p.n.Load())
	}
	if rec.renewals.Load() != 1 {
		t.Errorf("%d renewals", rec.renewals.Load())
	}
	for _, state := range rec.history() {
		if state == live.StateFailed {
			t.Errorf("a session that ended at its limit was reported as a failure: %v", rec.history())
		}
	}
	reqs := server.Requests()
	if len(reqs) != 2 || reqs[0].URL.Query().Get("pass") != "pass-1" || reqs[1].URL.Query().Get("pass") != "pass-2" {
		t.Errorf("the second connection did not carry the new pass")
	}
	if stream.Status().State != live.StateLive {
		t.Errorf("the stream is %v after the renewal", stream.Status().State)
	}
}

func TestSessionsThatEndAtOnceAreGivenUpOn(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) { s.Close(live.StatusSessionEnded, "session ended") })
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "the failure", func() bool { return stream.Status().State == live.StateFailed })
	if err := stream.Status().Err; !errors.Is(err, live.ErrRenewing) {
		t.Errorf("failed with %v, want ErrRenewing", err)
	}
}

func TestARefusedPassFailsTheStream(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) {})
	server.Refuse(401)
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "the failure", func() bool { return stream.Status().State == live.StateFailed })
	if err := stream.Status().Err; !errors.Is(err, live.ErrRefused) {
		t.Errorf("failed with %v, want ErrRefused", err)
	}
}

func TestAPassTheApplicationCannotGiveFailsTheStream(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) {})
	boom := errors.New("signed out")
	p := passes{err: boom}
	var rec recorder
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "the failure", func() bool { return stream.Status().State == live.StateFailed })
	if err := stream.Status().Err; !errors.Is(err, boom) {
		t.Errorf("failed with %v, want the application's error", err)
	}
	if len(server.Requests()) != 0 {
		t.Error("a socket was opened without a pass")
	}
}

// TestBusyAndFullEndTheStreamWithTheirOwnState: the node tells the viewer why
// there is no picture and then closes the socket, and the reason is what the
// screen shows.
func TestBusyAndFullEndTheStreamWithTheirOwnState(t *testing.T) {
	for _, want := range []live.State{live.StateBusy, live.StateFull} {
		server := wstest.New(t, func(s *wstest.Session) {
			s.Text(wstest.SignalMessage(want.String()))
			s.Close(1013, want.String())
		})
		var rec recorder
		var p passes
		stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
		stream.Start()

		eventually(t, want.String(), func() bool { return stream.Status().State == want })
		time.Sleep(50 * time.Millisecond)
		if got := stream.Status(); got.State != want || got.Err != nil {
			t.Errorf("%v: the stream settled on %+v", want, got)
		}
		if !want.Terminal() {
			t.Errorf("%v should be terminal", want)
		}
		stream.Stop()
	}
}

func TestLostAndStartingAreStatesTheStreamLeaves(t *testing.T) {
	track, samples := load(t, "h264")
	gate := make(chan struct{})
	server := wstest.New(t, func(s *wstest.Session) {
		s.Text(wstest.SignalMessage("starting"))
		sendClip(t, s, track, samples[:2], 4096)
		s.Text(wstest.SignalMessage("lost"))
		<-gate
		sendClip(t, s, track, samples[2:4], 4096)
		time.Sleep(time.Second)
	})
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "lost", func() bool { return stream.Status().State == live.StateLost })
	if stream.Status().State.Terminal() {
		t.Error("lost should not be terminal: the video returns on its own")
	}
	close(gate)
	eventually(t, "live again", func() bool { return rec.sampleCount() == 4 && stream.Status().State == live.StateLive })

	history := rec.history()
	if !slices.Contains(history, live.StateStarting) || !slices.Contains(history, live.StateLost) {
		t.Errorf("states %v", history)
	}
}

func TestAnApplicationMessageIsRelayed(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) {
		s.Text(`{"type":"boxes","plates":[{"text":"ABC1D23"}]}`)
		time.Sleep(300 * time.Millisecond)
	})
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "the message", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.messages) == 1
	})
	if !strings.Contains(rec.messages[0], "ABC1D23") {
		t.Errorf("relayed %q", rec.messages[0])
	}
}

func TestAMalformedMessageIsSkippedAndTheStreamGoesOn(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		s.Text("{broken")
		s.Binary([]byte{9, 9, 9})
		sendClip(t, s, track, samples[:2], 4096)
		time.Sleep(500 * time.Millisecond)
	})
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "the pictures", func() bool { return rec.sampleCount() == 2 })
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.drops) != 2 {
		t.Errorf("%d messages were skipped, want 2: %v", len(rec.drops), rec.drops)
	}
}

func TestASocketThatGoesSilentFails(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) { time.Sleep(2 * time.Second) })
	var rec recorder
	var p passes
	cfg := rec.config(server.URL("/live/cam"), p.ask)
	cfg.Silence = 150 * time.Millisecond
	stream := live.NewStream(cfg)
	stream.Start()
	defer stream.Stop()

	eventually(t, "the failure", func() bool { return stream.Status().State == live.StateFailed })
	if err := stream.Status().Err; !errors.Is(err, live.ErrSilent) {
		t.Errorf("failed with %v, want ErrSilent", err)
	}
}

// TestAKeepaliveKeepsTheSocketAlive: the node's ping is the message the silence
// limit is waiting for.
func TestAKeepaliveKeepsTheSocketAlive(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) {
		for i := 0; i < 10; i++ {
			s.Text(`{"type":"ping"}`)
			time.Sleep(60 * time.Millisecond)
		}
	})
	var rec recorder
	var p passes
	cfg := rec.config(server.URL("/live/cam"), p.ask)
	cfg.Silence = 200 * time.Millisecond
	stream := live.NewStream(cfg)
	stream.Start()
	defer stream.Stop()

	time.Sleep(500 * time.Millisecond)
	if stream.Status().State == live.StateFailed {
		t.Errorf("a socket that was pinged failed: %v", stream.Status().Err)
	}
}

func TestANormalCloseIsAnEndedStream(t *testing.T) {
	server := wstest.New(t, func(s *wstest.Session) { s.Close(1000, "") })
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	defer stream.Stop()

	eventually(t, "the failure", func() bool { return stream.Status().State == live.StateFailed })
	if err := stream.Status().Err; !errors.Is(err, live.ErrEnded) {
		t.Errorf("failed with %v, want ErrEnded", err)
	}
}

func TestPauseClosesTheSocketAndStartOpensANewOne(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples[:2], 4096)
		s.Read(3 * time.Second) // until the client leaves
	})
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config(server.URL("/live/cam"), p.ask))
	stream.Start()
	eventually(t, "the first pictures", func() bool { return rec.sampleCount() == 2 })

	stream.Pause()
	if stream.Status().State != live.StatePaused {
		t.Fatalf("paused stream is %v", stream.Status().State)
	}
	if !stream.Status().State.Terminal() {
		t.Error("paused should be terminal")
	}

	stream.Start()
	eventually(t, "the pictures of the second socket", func() bool { return rec.sampleCount() == 4 })
	if p.n.Load() != 2 {
		t.Errorf("%d passes, want a new one for the new socket", p.n.Load())
	}
	stream.Stop()
	if stream.Status().State != live.StateIdle {
		t.Errorf("stopped stream is %v", stream.Status().State)
	}
}

func TestAPassCallbackThatBlocksIsCancelledByStop(t *testing.T) {
	var rec recorder
	cancelled := make(chan struct{})
	cfg := rec.config("ws://127.0.0.1:1/live/cam", func(ctx context.Context) (string, error) {
		<-ctx.Done()
		close(cancelled)
		return "", ctx.Err()
	})
	stream := live.NewStream(cfg)
	stream.Start()
	time.Sleep(30 * time.Millisecond)

	done := make(chan struct{})
	go func() { stream.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return while a pass was being asked for")
	}
	select {
	case <-cancelled:
	default:
		t.Error("the pass callback was not cancelled")
	}
}

func TestAnEndpointThatIsNotWebSocketFails(t *testing.T) {
	var rec recorder
	var p passes
	stream := live.NewStream(rec.config("ftp://example.test/live/cam", p.ask))
	stream.Start()
	defer stream.Stop()
	eventually(t, "the failure", func() bool { return stream.Status().State == live.StateFailed })
}
