package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// A field is painted across the room it is given, and the person taps what is
// painted. The tap has to reach the editor behind it: an editor that is
// focused is what receives the characters the on-screen keyboard sends, and one
// that is not focused receives none, so the keyboard comes up, accepts every
// key and shows nothing.
//
// That was the sign-in of the Android app. The field sat in a column whose
// minimum width was zero, which is every column a screen centres, and the
// editor measured only its text: one point wide while empty. The border was
// drawn across the whole width, the tap landed on the border, and the editor's
// own pointer area was beside it.
//
// Measuring the field (TestAFieldFillsTheRoomItIsGiven) cannot see this,
// because the field reports the painted width. These tests tap it.

// tap presses and lifts a finger at x, y, and draws the frame that reads it.
func tap(p *panel, screen func(ayra.Context) ayra.Dimensions, x, y int) {
	at := f32.Pt(float32(x), float32(y))
	p.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: at},
	)
	p.draw(screen)
}

// typeText is what the platform input method sends for the keys it pressed. It
// reaches the editor that has the focus and no other.
func typeText(p *panel, screen func(ayra.Context) ayra.Dimensions, s string) {
	p.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: s})
	p.draw(screen)
}

// TestATapAnywhereOnAFieldFocusesItWhateverTheMinimumWidth taps across the
// field drawn with no minimum width, and the same field drawn filling a
// minimum, and types after each tap.
func TestATapAnywhereOnAFieldFocusesItWhateverTheMinimumWidth(t *testing.T) {
	for _, minimum := range []int{0, 200, 400} {
		// Left, middle and right of the painted field, on the line the text is
		// drawn on.
		for _, x := range []int{20, 200, 380} {
			p := newPanel()
			var state widget.Input
			screen := func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Min.X = minimum
				// A column that centres its children, which is how a sign-in
				// is laid out, hands each child a minimum of nothing.
				return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.InputProps{Placeholder: "voce@empresa.com.br", Kind: widget.Email}.Layout(c.With(gtx), &state)
					}),
				)
			}

			p.draw(screen)
			p.draw(screen)
			tap(p, screen, x, 24)
			typeText(p, screen, "ção")

			if got := state.Text(); got != "ção" {
				t.Errorf("minimum %d, tap at x=%d: the field holds %q after typing, want %q", minimum, x, got, "ção")
			}
		}
	}
}

// TestATapOnTheFieldsPaddingFocusesIt covers the strip between the border and
// the text: it is part of the field to a finger, and the field is only a few
// millimetres tall.
func TestATapOnTheFieldsPaddingFocusesIt(t *testing.T) {
	var state widget.Input
	screen := func(c ayra.Context) ayra.Dimensions {
		return widget.InputProps{Placeholder: "Nome"}.Layout(c, &state)
	}

	size := newPanel().draw(screen).Size
	for _, at := range []image.Point{
		{X: 2, Y: 2},
		{X: size.X / 2, Y: 2},
		{X: size.X - 2, Y: size.Y - 2},
		{X: size.X / 2, Y: size.Y - 2},
	} {
		p := newPanel()
		state = widget.Input{}
		p.draw(screen)
		p.draw(screen)
		tap(p, screen, at.X, at.Y)
		typeText(p, screen, "a")

		if got := state.Text(); got != "a" {
			t.Errorf("tap at %v of a %v field: it holds %q after typing, want %q", at, size, got, "a")
		}
	}
}
