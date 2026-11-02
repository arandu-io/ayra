package shell

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/io/event"
)

// The system's ask for less movement, and which of the two answers a frame is
// given. The platform's own reading is replaced by one the test controls, and the
// clock by one the test moves, because what is under test is the choice and the
// cache and not what a particular phone says.

// asking replaces the platform with one that answers what it is told and counts
// the questions, and puts the cache and the clock back when the test ends.
type asking struct {
	answer  bool
	asked   int
	elapsed time.Duration
}

func platform(t *testing.T, answer bool) *asking {
	t.Helper()
	a := &asking{answer: answer}
	start := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	was := struct {
		probe func() bool
		now   func() time.Time
	}{probe, now}
	probe = func() bool { a.asked++; return a.answer }
	now = func() time.Time { return start.Add(a.elapsed) }
	reading.mu.Lock()
	reading.at, reading.value = time.Time{}, false
	reading.mu.Unlock()

	t.Cleanup(func() {
		probe, now = was.probe, was.now
		reading.mu.Lock()
		reading.at, reading.value = time.Time{}, false
		reading.mu.Unlock()
	})
	return a
}

// frames plays n frames through a window with this configuration and says what
// each one was told.
func frames(t *testing.T, cfg Config, n int) []bool {
	t.Helper()
	events := make([]event.Event, 0, n+1)
	for i := 0; i < n; i++ {
		events = append(events, frame())
	}
	win := &scripted{events: append(events, app.DestroyEvent{})}

	var told []bool
	err := loop(win, cfg, nil, func(c ayra.Context) ayra.Dimensions {
		told = append(told, c.ReduceMotion)
		return ayra.Dimensions{}
	})
	if err != nil {
		t.Fatalf("the loop failed: %v", err)
	}
	return told
}

// TestAWindowThatSetNothingTakesTheSystemsAsk is the default: an application that
// never heard of Config.ReduceMotion still stops its skeletons when the person
// asked the phone for less movement.
func TestAWindowThatSetNothingTakesTheSystemsAsk(t *testing.T) {
	for _, answer := range []bool{true, false} {
		platform(t, answer)
		for i, got := range frames(t, Config{Title: "x"}, 3) {
			if got != answer {
				t.Errorf("with the system answering %v, frame %d was told %v", answer, i, got)
			}
		}
	}
}

// TestTheApplicationsOwnAnswerWinsOverTheSystems is the application's own switch:
// it is the person's word in the application's settings, and the system is not
// asked at all.
func TestTheApplicationsOwnAnswerWinsOverTheSystems(t *testing.T) {
	for _, theirs := range []bool{true, false} {
		a := platform(t, theirs)
		own := !theirs
		for i, got := range frames(t, Config{Title: "x", ReduceMotion: func() bool { return own }}, 3) {
			if got != own {
				t.Errorf("the application answered %v and the system %v, frame %d was told %v", own, theirs, i, got)
			}
		}
		if a.asked != 0 {
			t.Errorf("the system was asked %d times although the application answered", a.asked)
		}
	}
}

// TestTheSystemIsAskedOnceASecondAtMost plays a hundred frames and then moves the
// clock: one question for the hundred, a second one after a second, and the new
// answer reaching the screen when the person changed the setting meanwhile.
func TestTheSystemIsAskedOnceASecondAtMost(t *testing.T) {
	a := platform(t, false)

	frames(t, Config{Title: "x"}, 100)
	if a.asked != 1 {
		t.Fatalf("the system was asked %d times for a hundred frames drawn together, want 1", a.asked)
	}

	a.elapsed = 900 * time.Millisecond
	frames(t, Config{Title: "x"}, 10)
	if a.asked != 1 {
		t.Errorf("the system was asked again after 900 ms: %d questions", a.asked)
	}

	a.answer, a.elapsed = true, time.Second
	got := frames(t, Config{Title: "x"}, 2)
	if a.asked != 2 {
		t.Errorf("the system was asked %d times a second later, want 2", a.asked)
	}
	if !got[0] || !got[1] {
		t.Errorf("the setting changed and the frames were told %v, want true", got)
	}
}

// TestAClockThatWentBackwardsDoesNotFreezeTheReading is a phone whose time was
// corrected: a reading made "in the future" is asked again rather than kept until
// the clock catches up.
func TestAClockThatWentBackwardsDoesNotFreezeTheReading(t *testing.T) {
	a := platform(t, false)
	a.elapsed = time.Hour
	SystemReduceMotion()

	a.elapsed, a.answer = 0, true
	if !SystemReduceMotion() {
		t.Error("the clock went back an hour and the old answer was kept")
	}
}

// TestTheAnimationScaleIsReadAsAndroidKeepsIt is the text Settings.Global holds:
// a float, and "0" or "0.0" only when the person removed the animations.
func TestTheAnimationScaleIsReadAsAndroidKeepsIt(t *testing.T) {
	for setting, want := range map[string]bool{
		"0": true, "0.0": true, "0.00": true, " 0.0\n": true,
		"1": false, "1.0": false, "0.5": false, "10": false,
		"": false, "off": false, "NaN": false,
	} {
		if got := animationsOff(setting); got != want {
			t.Errorf("animationsOff(%q) = %v, want %v", setting, got, want)
		}
	}
}
