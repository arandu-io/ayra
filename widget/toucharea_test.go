package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/layout"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// TestTouchAreaGrowsAnyPressableToTheSizeOfAFinger puts a control the size of an
// unlabelled checkbox, 18 points, and of a slider's track, 20 tall, behind the
// shared helper. A finger lands within 44 points of the middle of each and the
// control has to answer, while the room it reports, which is what a screen lays
// out around, is the 18 or the 20 it drew.
func TestTouchAreaGrowsAnyPressableToTheSizeOfAFinger(t *testing.T) {
	for name, side := range map[string]image.Point{
		"checkbox": {18, 18},
		"slider":   {200, 20},
		"stepper":  {28, 28},
	} {
		t.Run(name, func(t *testing.T) {
			var state widget.Clickable
			var drawn image.Point
			clicked := 0
			screen := func(c ayra.Context) ayra.Dimensions {
				if state.Clicked(c) {
					clicked++
				}
				return layout.Inset{Top: 100, Left: 100}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					dims := state.Reach(c.With(gtx), func(c ayra.Context) ayra.Dimensions {
						return ayra.Dimensions{Size: side}
					})
					drawn = dims.Size
					return dims
				})
			}

			p := newPanel()
			p.draw(screen)
			if drawn != side {
				t.Fatalf("the control reports %v, want the %v it drew: the area that answers must not change the layout", drawn, side)
			}

			// 20 points off the middle of the short side, which is inside 44.
			middle := image.Pt(100+side.X/2, 100+side.Y/2)
			for name, at := range map[string]image.Point{
				"above": {middle.X, middle.Y - 20},
				"below": {middle.X, middle.Y + 20},
			} {
				before := clicked
				tap(p, screen, at.X, at.Y)
				if clicked != before+1 {
					t.Errorf("a tap %s the %v control, 20 points from its middle, did not press it", name, side)
				}
			}

			before := clicked
			tap(p, screen, middle.X, middle.Y+30)
			if clicked != before {
				t.Errorf("a tap 30 points from the middle of the %v control pressed it: the area is more than 44", side)
			}
		})
	}
}
