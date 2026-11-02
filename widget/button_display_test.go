package widget_test

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/icons"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// Found on 02/11/2026 by drawing the display widgets under the Alpr.sh theme at
// the width of a phone. Each test below fails on the code that was there.

// TestAButtonSmallerThanAFingerAnswersToAFinger taps the band a finger covers
// around a small button. The button is 29 points tall at the Small size and 24
// at the smallest, which is what a finger lands badly on; the area that answers
// has to be 44 points each way, and what the button draws and reports must not
// grow with it, or every screen laid out around it moves.
func TestAButtonSmallerThanAFingerAnswersToAFinger(t *testing.T) {
	for _, size := range []widget.Size{widget.ExtraSmall, widget.Small, widget.Medium, widget.Icon} {
		t.Run(size.String(), func(t *testing.T) {
			var state widget.Button
			var drawn image.Point
			clicked := 0
			screen := func(c ayra.Context) ayra.Dimensions {
				if state.Clicked(c) {
					clicked++
				}
				return layout.Inset{Top: 100, Left: 100}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					props := widget.ButtonProps{Label: "Ok", Size: size}
					if size == widget.Icon {
						props = widget.ButtonProps{Label: "Excluir", Icon: icons.Trash(), IconOnly: true, Size: size}
					}
					dims := props.Layout(c.With(gtx), &state)
					drawn = dims.Size
					return dims
				})
			}

			p := newPanel()
			p.draw(screen)
			if drawn.Y >= 44 {
				t.Skipf("a %s button is %d tall: already a finger's size", size, drawn.Y)
			}

			middle := 100 + drawn.X/2
			below := 100 + drawn.Y
			// Just beyond the drawing, inside what a finger covers.
			for name, at := range map[string]image.Point{
				"below": {middle, below + 4},
				"above": {middle, 100 - 4},
			} {
				before := clicked
				tap(p, screen, at.X, at.Y)
				if clicked != before+1 {
					t.Errorf("a tap %s the %s button, %d points from it, did not press it", name, size, 4)
				}
			}

			// Beyond what a finger covers, nothing is pressed.
			before := clicked
			tap(p, screen, middle, below+(44-drawn.Y)/2+12)
			if clicked != before {
				t.Errorf("a tap far below the %s button pressed it", size)
			}
		})
	}
}

// TestAFocusedButtonShowsItsRing moves the keyboard focus onto a button and
// looks at what is drawn just outside it. The button took the focus and drew
// nothing different, so a person using a keyboard, a switch or a screen reader's
// cursor could not tell which button Return would press.
func TestAFocusedButtonShowsItsRing(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		t.Run(scheme.String(), func(t *testing.T) {
			var state widget.Button
			var drawn image.Point
			screen := func(c ayra.Context) ayra.Dimensions {
				return layout.Inset{Top: 40, Left: 40}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					dims := widget.ButtonProps{Label: "Salvar", Size: widget.Large}.Layout(c.With(gtx), &state)
					drawn = dims.Size
					return dims
				})
			}

			p := newPanel()
			p.at(scheme, time.Time{}, screen)
			resting := p.raster(t)

			p.router.MoveFocus(key.FocusForward)
			p.at(scheme, time.Time{}, screen)
			p.at(scheme, time.Time{}, screen)
			focused := p.raster(t)

			// Two points above the top edge, in the middle: outside the button,
			// inside the three the ring takes.
			at := image.Pt(40+drawn.X/2, 40-2)
			if resting.RGBAAt(at.X, at.Y) != (color.RGBA{}) {
				t.Fatalf("the resting button has something drawn outside it at %v: %v", at, resting.RGBAAt(at.X, at.Y))
			}
			if focused.RGBAAt(at.X, at.Y).A == 0 {
				t.Errorf("the focused button has no ring outside it at %v", at)
			}
		})
	}
}

// TestALargeButtonIsSetInTheBodySize keeps the large button to the label of
// the others, as Kyse's is: it differs in the room it takes, not in the type.
// Its label was set at the heading size, 20 points, and the button was 48 tall.
func TestALargeButtonIsSetInTheBodySize(t *testing.T) {
	size := func(s widget.Size) image.Point {
		c, _ := frame(t, theme.Light, 400)
		var state widget.Button
		return widget.ButtonProps{Label: "Salvar", Size: s}.Layout(c, &state).Size
	}
	large, medium := size(widget.Large), size(widget.Medium)

	if large.Y > 44 {
		t.Errorf("a large button is %d tall, want at most the 44 of a finger", large.Y)
	}
	if large.Y <= medium.Y {
		t.Errorf("a large button (%d) is not taller than the ordinary one (%d)", large.Y, medium.Y)
	}
}
