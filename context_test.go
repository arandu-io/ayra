package ayra_test

import (
	"testing"

	"github.com/arandu-io/ayra"
)

// TestRedrawAsksForAFrame fixes the call that makes late work visible.
//
// Work that finishes while nobody is touching the device -- an answer from the
// server, a timer -- changes what should be on screen and draws nothing. The
// failure is a screen that is correct in memory and stale in front of a
// person, until they happen to click.
func TestRedrawAsksForAFrame(t *testing.T) {
	asked := 0
	c := ayra.Context{Invalidate: func() { asked++ }}

	c.Redraw()

	if asked != 1 {
		t.Errorf("Redraw asked for %d frames, want 1", asked)
	}
}

// TestRedrawOutsideAWindowDoesNothing keeps a control from having to ask
// whether it may redraw.
//
// A context built for a test or for a tool that renders one frame has no
// window behind it, and a control that called through would panic there and
// nowhere else -- which is the kind of failure that only appears in the test
// suite of whoever depends on this.
func TestRedrawOutsideAWindowDoesNothing(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("redrawing outside a window panicked: %v", r)
		}
	}()

	ayra.Context{}.Redraw()
}

// TestWithKeepsWhatTravels fixes which half of the context a narrower one
// carries.
//
// The frame changes and the rest does not: a layout handing a child less room
// must not hand it a different palette, a second shaper, or a context that can
// no longer ask for a frame.
func TestWithKeepsWhatTravels(t *testing.T) {
	asked := 0
	c := ayra.Context{Invalidate: func() { asked++ }}

	narrower := c.With(c.Context)
	narrower.Redraw()

	if asked != 1 {
		t.Error("a narrower context lost the ability to ask for a frame")
	}
	if narrower.Theme != c.Theme {
		t.Error("a narrower context was given a different theme")
	}
	if narrower.Shaper != c.Shaper {
		t.Error("a narrower context was given a different shaper")
	}
}
