package shell

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/theme"
)

// schemes decides which palette the window draws, from three things that can
// disagree: the scheme the application opened with, the one the operating
// system is in, and a choice a person made on a toggle.
//
// The rule is short. A window in [theme.System] mode draws the system's scheme
// whenever it is known; one in [theme.Fixed] mode draws the scheme it was
// given, and a choice made with ChangeScheme puts it there. The system's scheme
// is always recorded, in either mode, so that going back to following it needs
// no new event from the platform.
//
// It is a value with no window in it, and that is deliberate: the whole policy
// is exercised by calling it.
type schemes struct {
	// mode is where the scheme comes from now.
	mode theme.Mode
	// scheme is what is drawn.
	scheme theme.Scheme
	// system is what the platform last said, and unknown until it does.
	system app.Appearance
}

// newSchemes starts in the mode and the scheme the configuration asked for.
func newSchemes(mode theme.Mode, scheme theme.Scheme) *schemes {
	return &schemes{mode: mode, scheme: scheme}
}

// observe records the system's scheme and answers whether the one drawn
// changed with it.
func (s *schemes) observe(appearance app.Appearance) bool {
	s.system = appearance
	return s.follow()
}

// choose is a person's choice of one scheme: the window stops following the
// system and draws that one. It answers whether the drawn scheme changed.
func (s *schemes) choose(scheme theme.Scheme) bool {
	s.mode = theme.Fixed
	if s.scheme == scheme {
		return false
	}
	s.scheme = scheme
	return true
}

// setMode changes where the scheme comes from. Going to the system takes its
// scheme at once, if it is known. It answers whether the drawn scheme changed.
func (s *schemes) setMode(mode theme.Mode) bool {
	s.mode = mode
	return s.follow()
}

// follow draws the system's scheme when the mode asks for it and the platform
// said which it is. Unknown changes nothing: guessing light on a dark phone is
// worse than staying in the scheme the application opened in.
func (s *schemes) follow() bool {
	if s.mode != theme.System {
		return false
	}
	var scheme theme.Scheme
	switch s.system {
	case app.AppearanceLight:
		scheme = theme.Light
	case app.AppearanceDark:
		scheme = theme.Dark
	default:
		return false
	}
	if s.scheme == scheme {
		return false
	}
	s.scheme = scheme
	return true
}

// stages tells the application where it is, once per change.
type stages struct {
	notify func(ayra.Lifecycle)
	// at is the stage the application was last told.
	at ayra.Lifecycle
	// done is set once Destroyed was told, after which nothing else is.
	done bool
}

func newStages(notify func(ayra.Lifecycle)) *stages {
	return &stages{notify: notify, at: ayra.Foreground}
}

// observe passes on a stage the platform reported, unless it is the one the
// application is already in. A platform that says "foreground" at start-up, or
// twice, is not a return from anywhere.
func (s *stages) observe(stage app.Stage) {
	next := ayra.Foreground
	if stage == app.StageBackground {
		next = ayra.Background
	}
	s.tell(next)
}

// destroyed passes on the end of the window.
func (s *stages) destroyed() { s.tell(ayra.Destroyed) }

func (s *stages) tell(next ayra.Lifecycle) {
	if s.done || next == s.at {
		return
	}
	s.at = next
	s.done = next == ayra.Destroyed
	if s.notify != nil {
		s.notify(next)
	}
}
