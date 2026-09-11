package widget_test

import (
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// song is an ordinary recording, and film is one long enough to need the third
// field on its clock.
const (
	song = 3*time.Minute + 47*time.Second
	film = 2*time.Hour + 14*time.Minute
)

// TestAPlayerTakesRoomForItsTransport is the first thing a control has to do,
// and the thing a broken one fails silently: a zero size draws nothing, and a
// screen with an invisible transport reads as a layout problem rather than as a
// control that never measured itself.
func TestAPlayerTakesRoomForItsTransport(t *testing.T) {
	c, _ := frame(t, theme.Light, 600)
	var state widget.Player

	dims := widget.PlayerProps{Duration: song, Position: 30 * time.Second}.Layout(c, &state)

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("a transport took %v, want room for its controls", dims.Size)
	}
}

// TestATransportDrawsAtEveryWidth keeps a column that is narrow, ordinary or
// very wide from producing a row with no height or no width.
func TestATransportDrawsAtEveryWidth(t *testing.T) {
	for _, width := range []int{160, 240, 320, 480, 600, 900, 1400} {
		for _, props := range []widget.PlayerProps{
			{Duration: song, Position: 30 * time.Second},
			{Duration: film, Position: time.Hour, ShowSpeed: true},
			{Duration: film, Position: time.Hour, Compact: true},
			{Position: 30 * time.Second},
		} {
			c, _ := frame(t, theme.Light, width)
			var state widget.Player

			dims := props.Layout(c, &state)

			if dims.Size.X <= 0 || dims.Size.Y <= 0 {
				t.Errorf("at %d the transport took %v", width, dims.Size)
			}
		}
	}
}

// TestMediaOfUnknownLengthDraws fixes the state every stream starts in.
//
// The duration is the denominator of the thumb, the clock and the seek, and it
// is zero until somebody reads a header. A transport that divided by it would
// not draw a wrong picture; it would stop the frame.
func TestMediaOfUnknownLengthDraws(t *testing.T) {
	for _, position := range []time.Duration{0, 30 * time.Second, 3 * time.Hour} {
		c, _ := frame(t, theme.Light, 600)
		var state widget.Player

		dims := widget.PlayerProps{Position: position, Playing: true}.Layout(c, &state)

		if dims.Size.X <= 0 || dims.Size.Y <= 0 {
			t.Errorf("a stream at %v took %v", position, dims.Size)
		}
		if _, asked := state.Sought(); asked {
			t.Errorf("a stream at %v asked to seek in media whose length nobody knows", position)
		}
	}
}

// TestATitleAddsALineAndCompactRemovesIt fixes what the two fields do.
func TestATitleAddsALineAndCompactRemovesIt(t *testing.T) {
	measure := func(props widget.PlayerProps) int {
		c, _ := frame(t, theme.Light, 600)
		var state widget.Player
		return props.Layout(c, &state).Size.Y
	}

	bare := measure(widget.PlayerProps{Duration: song})
	titled := measure(widget.PlayerProps{Duration: song, Title: "Cancao"})
	compact := measure(widget.PlayerProps{Duration: song, Title: "Cancao", Compact: true})

	if titled <= bare {
		t.Errorf("a title took no extra height: %d with, %d without", titled, bare)
	}
	if compact != bare {
		t.Errorf("a compact transport with a title is %d tall and one with no title is %d", compact, bare)
	}
}

// TestEverySchemeDraws keeps a palette from producing a transport made entirely
// of the background colour.
func TestEverySchemeDraws(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		t.Run(scheme.String(), func(t *testing.T) {
			c, _ := frame(t, scheme, 600)
			var state widget.Player

			dims := widget.PlayerProps{
				Title:     "Cancao",
				Duration:  film,
				Position:  time.Hour,
				Playing:   true,
				Volume:    0.7,
				Speed:     1.25,
				ShowSpeed: true,
			}.Layout(c, &state)

			if dims.Size.X <= 0 || dims.Size.Y <= 0 {
				t.Errorf("took %v", dims.Size)
			}
		})
	}
}

// TestAFreshPlayerAsksForNothing fixes the state a screen arrives in.
//
// A control that reported an intent before anybody touched it would toggle the
// media on the first frame it was drawn.
func TestAFreshPlayerAsksForNothing(t *testing.T) {
	c, _ := frame(t, theme.Light, 600)
	var state widget.Player

	widget.PlayerProps{Duration: song, Position: time.Minute, Volume: 0.5, ShowSpeed: true}.Layout(c, &state)

	if state.Toggled() {
		t.Error("a fresh transport asked to start the media")
	}
	if state.MuteToggled() {
		t.Error("a fresh transport asked to silence the sound")
	}
	if _, asked := state.Sought(); asked {
		t.Error("a fresh transport asked to seek")
	}
	if _, asked := state.VolumeSet(); asked {
		t.Error("a fresh transport asked to change the volume")
	}
	if _, asked := state.SpeedSet(); asked {
		t.Error("a fresh transport asked to change the rate")
	}
	if _, scrubbing := state.Scrubbing(); scrubbing {
		t.Error("a fresh transport reports a finger on its scrubber")
	}
}

// TestAPlayerDrawnEveryFrameAsksForNothing is the guard against a control that
// reports the same event over and over.
//
// The volume and the scrubber are written from their props on every frame they
// are not held. A change detected against the previous frame rather than
// against what was written would fire on every tick of the media.
func TestAPlayerDrawnEveryFrameAsksForNothing(t *testing.T) {
	var state widget.Player

	for tick := 0; tick < 30; tick++ {
		c, _ := frame(t, theme.Light, 600)

		widget.PlayerProps{
			Duration:  song,
			Position:  time.Duration(tick) * time.Second,
			Playing:   true,
			Volume:    0.5,
			ShowSpeed: true,
		}.Layout(c, &state)

		if state.Toggled() || state.MuteToggled() {
			t.Fatalf("frame %d asked to change the transport with nothing touched", tick)
		}
		if _, asked := state.Sought(); asked {
			t.Fatalf("frame %d asked to seek with nothing touched", tick)
		}
		if _, asked := state.VolumeSet(); asked {
			t.Fatalf("frame %d asked to change the volume with nothing touched", tick)
		}
		if _, asked := state.SpeedSet(); asked {
			t.Fatalf("frame %d asked to change the rate with nothing touched", tick)
		}
	}
}

// TestADisabledPlayerDrawsAndAsksForNothing is the half a test usually forgets:
// a control drawn as unavailable that still reports is worse than one that is
// not drawn as unavailable at all, because the screen says one thing and does
// another.
func TestADisabledPlayerDrawsAndAsksForNothing(t *testing.T) {
	c, _ := frame(t, theme.Light, 600)
	var state widget.Player

	dims := widget.PlayerProps{
		Duration:  song,
		Position:  time.Minute,
		Volume:    0.5,
		ShowSpeed: true,
		Disabled:  true,
	}.Layout(c, &state)

	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("a disabled transport took %v, and it still has to be drawn", dims.Size)
	}
	if state.Toggled() || state.MuteToggled() {
		t.Error("a disabled transport reported a press")
	}
	if _, asked := state.Sought(); asked {
		t.Error("a disabled transport reported a seek")
	}
}
