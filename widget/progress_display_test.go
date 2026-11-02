package widget_test

import (
	"image"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// Found on 02/11/2026 by drawing the display widgets under the Alpr.sh theme at
// the width of a phone. Each test below fails on the code that was there.

// TestAnIndeterminateBarMoves draws the bar half way through its trip and
// finds it in the middle of the track. It drew an empty track at every moment,
// which is what a bar at nothing looks like.
func TestAnIndeterminateBarMoves(t *testing.T) {
	var p = newPanel()
	p.size = image.Pt(300, 20)
	screen := func(c ayra.Context) ayra.Dimensions { return widget.ProgressProps{Indeterminate: true}.Layout(c) }

	th := theme.New(theme.Light)
	track := th.Colours.Muted

	// 700 ms into a 1400 ms trip: the third of the track that travels is in the
	// middle of it.
	p.at(theme.Light, time.Unix(0, 0).Add(700*time.Millisecond), screen)
	if _, asked := p.router.WakeupTime(); !asked {
		t.Error("the bar did not ask for the next frame, so it moves once and stops")
	}
	middle := p.raster(t).RGBAAt(150, 4)
	if middle.R > track.R/2 {
		t.Errorf("the middle of the track is %v half way through the trip, want the bar's own dark", middle)
	}
	edge := p.raster(t).RGBAAt(8, 4)
	if int(edge.R) < int(track.R)-6 {
		t.Errorf("the start of the track is %v, want the track's own colour: the bar is a third wide", edge)
	}

	// A bar that is not indeterminate asks for nothing: a still screen stays
	// still.
	still := newPanel()
	still.at(theme.Light, time.Unix(0, 0).Add(700*time.Millisecond), func(c ayra.Context) ayra.Dimensions {
		return widget.ProgressProps{Fraction: 0.5}.Layout(c)
	})
	if _, asked := still.router.WakeupTime(); asked {
		t.Error("a determinate bar asked for another frame")
	}
}
