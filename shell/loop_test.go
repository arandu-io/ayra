package shell

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// The path from a platform's event to the palette on screen, walked with a
// window that is a list of events. Nothing here opens a display: what is under
// test is what the loop does with the scheme the system reports, the choice a
// person makes and the stage the application is in, and each of those is an
// event the platform sends.

// scripted is a window made of the events it will deliver, in order. It
// records what the loop asked of it.
type scripted struct {
	events []event.Event
	at     int
	// bars is the configuration the loop built with the options it sent.
	bars app.Config
	// options counts the calls to Option.
	options int
	// invalidated counts the frames the loop asked for.
	invalidated int
}

func (s *scripted) Event() event.Event {
	e := s.events[s.at]
	s.at++
	return e
}

func (s *scripted) Option(opts ...app.Option) {
	s.options++
	for _, o := range opts {
		o(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &s.bars)
	}
}

func (s *scripted) Invalidate() { s.invalidated++ }

func frame() app.FrameEvent {
	return app.FrameEvent{
		Size:   image.Pt(200, 200),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Frame:  func(*op.Ops) {},
	}
}

func system(a app.Appearance) app.ConfigEvent {
	return app.ConfigEvent{Config: app.Config{Appearance: a}}
}

// drawn is what a screen saw in one frame.
type drawn struct {
	scheme theme.Scheme
	mode   theme.Mode
}

// run plays the events through the loop with a screen that records every frame
// and lets a script act on the context of the frame it is in.
func run(t *testing.T, cfg Config, events []event.Event, act func(frame int, c ayra.Context)) (*scripted, []drawn) {
	t.Helper()
	win := &scripted{events: append(events, app.DestroyEvent{})}
	var seen []drawn
	err := loop(win, cfg, nil, func(c ayra.Context) ayra.Dimensions {
		seen = append(seen, drawn{c.Theme.Scheme, c.Mode})
		if act != nil {
			act(len(seen)-1, c)
		}
		return ayra.Dimensions{}
	})
	if err != nil {
		t.Fatalf("the loop failed: %v", err)
	}
	return win, seen
}

// TestTheWindowOpensInTheSchemeOfTheSystem is the first frame on a phone in the
// dark: the platform reports its scheme before the first frame, and the first
// frame is already drawn in it -- no flash of the light palette.
func TestTheWindowOpensInTheSchemeOfTheSystem(t *testing.T) {
	_, seen := run(t, Config{Title: "x", Mode: theme.System},
		[]event.Event{system(app.AppearanceDark), frame()}, nil)

	if len(seen) != 1 || seen[0].scheme != theme.Dark {
		t.Fatalf("the first frame was drawn %v, want one frame in dark", seen)
	}
	if seen[0].mode != theme.System {
		t.Errorf("the screen was told the mode is %v, want system", seen[0].mode)
	}
}

// TestTheWindowChangesWithTheSystemWhileItRuns is the toggle in the quick
// settings with the application open: no new window, the next frame is in the
// other palette.
func TestTheWindowChangesWithTheSystemWhileItRuns(t *testing.T) {
	win, seen := run(t, Config{Title: "x", Mode: theme.System}, []event.Event{
		system(app.AppearanceLight), frame(),
		system(app.AppearanceDark), frame(),
		system(app.AppearanceLight), frame(),
	}, nil)

	want := []theme.Scheme{theme.Light, theme.Dark, theme.Light}
	for i, scheme := range want {
		if seen[i].scheme != scheme {
			t.Errorf("frame %d was drawn %v, want %v", i, seen[i].scheme, scheme)
		}
	}
	if win.invalidated != 2 {
		t.Errorf("a frame was asked for %d times, want 2: once for each change", win.invalidated)
	}
}

// TestTheSystemBarsFollowTheBackgroundOfThePalette fixes what the bars are
// painted with and when: the background of the palette that is on screen, at
// the first frame and again at each change, and not on the frames between.
func TestTheSystemBarsFollowTheBackgroundOfThePalette(t *testing.T) {
	light, dark := theme.New(theme.Light).Colours.Background, theme.New(theme.Dark).Colours.Background

	win := &scripted{}
	events := []event.Event{
		system(app.AppearanceLight), frame(), frame(),
		system(app.AppearanceDark), frame(), frame(),
		app.DestroyEvent{},
	}
	var afterEach []app.Config
	win.events = events
	_ = loop(win, Config{Title: "x", Mode: theme.System}, nil, func(c ayra.Context) ayra.Dimensions {
		afterEach = append(afterEach, win.bars)
		return ayra.Dimensions{}
	})

	if win.options != 2 {
		t.Errorf("the bars were configured %d times, want 2: first frame and the change", win.options)
	}
	if afterEach[0].StatusColor != light || afterEach[0].NavigationColor != light {
		t.Errorf("the bars on a light window are %v and %v, want the light background %v",
			afterEach[0].StatusColor, afterEach[0].NavigationColor, light)
	}
	if afterEach[2].StatusColor != dark || afterEach[2].NavigationColor != dark {
		t.Errorf("the bars on a dark window are %v and %v, want the dark background %v",
			afterEach[2].StatusColor, afterEach[2].NavigationColor, dark)
	}
}

// TestAChoiceWinsOverTheSystemWhileItStands is the manual toggle: a person who
// chose light on a dark phone keeps light when the system reports dark again,
// until they hand the decision back.
func TestAChoiceWinsOverTheSystemWhileItStands(t *testing.T) {
	_, seen := run(t, Config{Title: "x", Mode: theme.System}, []event.Event{
		system(app.AppearanceDark), frame(), // 0: follows the system
		frame(), // 1: after the choice
		system(app.AppearanceLight), frame(),
		system(app.AppearanceDark), frame(), // 3: the system moved, the choice stands
		frame(), // 4: after FollowSystem
	}, func(frame int, c ayra.Context) {
		switch frame {
		case 0:
			c.ChangeScheme(theme.Light)
		case 3:
			c.FollowSystem()
		}
	})

	want := []drawn{
		{theme.Dark, theme.System},
		{theme.Light, theme.Fixed},
		{theme.Light, theme.Fixed},
		{theme.Light, theme.Fixed},
		{theme.Dark, theme.System},
	}
	for i, w := range want {
		if seen[i] != w {
			t.Errorf("frame %d was %v, want %v", i, seen[i], w)
		}
	}
}

// TestFollowingTheSystemAgainTakesTheSchemeItIsInNow keeps the system's last
// report across a choice: it is recorded while the window ignores it, so the
// hand-back needs no new event from the platform.
func TestFollowingTheSystemAgainTakesTheSchemeItIsInNow(t *testing.T) {
	state := newSchemes(theme.System, theme.Light)
	state.observe(app.AppearanceDark)
	state.choose(theme.Light)
	if got := state.scheme; got != theme.Light {
		t.Fatalf("a choice of light drew %v", got)
	}
	if !state.setMode(theme.System) || state.scheme != theme.Dark {
		t.Errorf("following the system again drew %v, want dark, the scheme it reported", state.scheme)
	}
}

// TestAFixedWindowIgnoresTheSystem is the behaviour the window had before the
// system could be followed, and it is still the zero value.
func TestAFixedWindowIgnoresTheSystem(t *testing.T) {
	_, seen := run(t, Config{Title: "x", Scheme: theme.Light}, []event.Event{
		system(app.AppearanceDark), frame(),
		system(app.AppearanceLight), frame(),
	}, nil)

	for i, got := range seen {
		if got.scheme != theme.Light || got.mode != theme.Fixed {
			t.Errorf("frame %d of a fixed window was %v, want light and fixed", i, got)
		}
	}
}

// TestAPlatformThatCannotTellKeepsTheSchemeItOpenedWith is Linux with no
// settings portal, or a browser without the media query: unknown is not light.
func TestAPlatformThatCannotTellKeepsTheSchemeItOpenedWith(t *testing.T) {
	_, seen := run(t, Config{Title: "x", Mode: theme.System, Scheme: theme.Dark}, []event.Event{
		system(app.AppearanceUnknown), frame(),
		system(app.AppearanceUnknown), frame(),
	}, nil)

	for i, got := range seen {
		if got.scheme != theme.Dark {
			t.Errorf("frame %d was %v, want the scheme it opened with, dark", i, got.scheme)
		}
	}
}

// TestAThemeOfTheProductGetsTheLocale keeps a locale from being lost by a
// product that builds its own theme.
func TestAThemeOfTheProductGetsTheLocale(t *testing.T) {
	cfg := Config{
		Title:  "x",
		Locale: theme.PortugueseBrazil(),
		Theme:  func(s theme.Scheme) theme.Theme { return theme.New(s) },
	}
	if got := cfg.theme(theme.Dark).Locale.Tag; got != "pt-BR" {
		t.Errorf("the theme of the window carries locale %q, want pt-BR", got)
	}
	if got := (Config{Title: "x"}).theme(theme.Light).Locale; !got.IsZero() {
		t.Errorf("a window with no locale got %q", got.Tag)
	}
}

// TestTheApplicationIsToldWhereItIs plays the stages: it goes to the background
// and comes back, and the window is destroyed.
func TestTheApplicationIsToldWhereItIs(t *testing.T) {
	var told []ayra.Lifecycle
	run(t, Config{Title: "x", OnLifecycle: func(l ayra.Lifecycle) { told = append(told, l) }}, []event.Event{
		frame(),
		app.StageEvent{Stage: app.StageForeground}, // already there: not a notice
		app.StageEvent{Stage: app.StageBackground},
		app.StageEvent{Stage: app.StageBackground}, // repeated: not a notice
		app.StageEvent{Stage: app.StageForeground},
		app.StageEvent{Stage: app.StageBackground},
	}, nil)

	want := []ayra.Lifecycle{ayra.Background, ayra.Foreground, ayra.Background, ayra.Destroyed}
	if len(told) != len(want) {
		t.Fatalf("the application was told %v, want %v", told, want)
	}
	for i := range want {
		if told[i] != want[i] {
			t.Errorf("notice %d was %v, want %v", i, told[i], want[i])
		}
	}
}

// TestAWindowWithNoWatcherStillRuns keeps the callback optional.
func TestAWindowWithNoWatcherStillRuns(t *testing.T) {
	run(t, Config{Title: "x"}, []event.Event{app.StageEvent{Stage: app.StageBackground}, frame()}, nil)
}
