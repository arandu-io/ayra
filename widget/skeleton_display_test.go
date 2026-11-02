package widget_test

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// A skeleton said "something is coming" and stood perfectly still, where Kyse's
// pulses. These tests draw it at two moments and with the system asking for less
// movement.

func skeletonAt(t *testing.T, now time.Time, reduce bool) (pixel [4]uint8, asked bool) {
	t.Helper()
	th := theme.New(theme.Light)
	p := newPanel()
	p.size = image.Pt(120, 40)
	p.moving(th, now, reduce, func(c ayra.Context) ayra.Dimensions {
		c.Fill(th.Colours.Background)
		return widget.SkeletonProps{Height: 30}.Layout(c)
	})
	_, asked = p.router.WakeupTime()
	got := p.raster(t).RGBAAt(60, 15)
	return [4]uint8{got.R, got.G, got.B, got.A}, asked
}

// TestASkeletonPulsesBetweenTwoFrames draws it at the start of its cycle and half
// way through it, and finds the colour changed, and the next frame asked for.
func TestASkeletonPulsesBetweenTwoFrames(t *testing.T) {
	start, asked := skeletonAt(t, time.Unix(0, 0), false)
	if !asked {
		t.Error("a pulsing skeleton did not ask for the next frame, so it pulses once and stops")
	}
	half, _ := skeletonAt(t, time.Unix(0, 0).Add(time.Second), false)

	if start == half {
		t.Errorf("the skeleton is %v at the start of its pulse and %v half way: it does not change", start, half)
	}
	// It fades toward the ground, which on a light page is lighter than the
	// muted colour it is made of.
	if half[0] <= start[0] {
		t.Errorf("at half strength the skeleton is %v, no lighter than %v at full", half, start)
	}
}

// TestASkeletonStandsStillWhenTheSystemAsksForLessMovement draws it at the same
// two moments with the ask set: the same colour, and no frame requested.
func TestASkeletonStandsStillWhenTheSystemAsksForLessMovement(t *testing.T) {
	a, asked := skeletonAt(t, time.Unix(0, 0), true)
	b, _ := skeletonAt(t, time.Unix(0, 0).Add(time.Second), true)

	if a != b {
		t.Errorf("with less movement asked for, the skeleton is %v and then %v", a, b)
	}
	if asked {
		t.Error("with less movement asked for, the skeleton still asked for the next frame")
	}
}

// TestASkeletonDrawnWithNoClockIsAtFullStrength keeps the pictures the library
// keeps: a frame with no clock is the start of the cycle, at full strength, and
// asks for nothing.
func TestASkeletonDrawnWithNoClockIsAtFullStrength(t *testing.T) {
	zero, asked := skeletonAt(t, time.Time{}, false)
	start, _ := skeletonAt(t, time.Unix(0, 0), false)

	if zero != start {
		t.Errorf("with no clock the skeleton is %v, want the full strength of %v", zero, start)
	}
	if asked {
		t.Error("a frame with no clock asked for another")
	}
}

// TestAnIndeterminateBarStandsStillWhenTheSystemAsksForLessMovement is Kyse's
// prefers-reduced-motion for the bar: the track filled, still, and no more frames.
func TestAnIndeterminateBarStandsStillWhenTheSystemAsksForLessMovement(t *testing.T) {
	th := theme.New(theme.Light)
	p := newPanel()
	p.size = image.Pt(300, 20)
	p.moving(th, time.Unix(0, 0), true, func(c ayra.Context) ayra.Dimensions {
		return widget.ProgressProps{Indeterminate: true}.Layout(c)
	})
	if _, asked := p.router.WakeupTime(); asked {
		t.Error("the bar asked for the next frame with less movement asked for")
	}
	picture := p.raster(t)
	for _, x := range []int{12, 150, 288} {
		if got := picture.RGBAAt(x, 4); int(got.R) > int(th.Colours.Muted.R)/2 {
			t.Errorf("the track at %d is %v, want it filled with the bar's own colour", x, got)
		}
	}
}
