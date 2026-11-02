package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/widget"
)

// TestAPressableRowAnswersAFingerOffItsEdge: a row of a sidebar or of a menu is
// 37 points, and the finger that lands a few points above it is not a miss. The
// room it reports, which is what the screen is laid out around, stays what it
// drew.
func TestAPressableRowAnswersAFingerOffItsEdge(t *testing.T) {
	p := phone(300)
	var row widget.Button
	pressed := 0
	screen := func(c ayra.Context) ayra.Dimensions {
		if row.Clicked(c) {
			pressed++
		}
		return layout.Inset{Top: 100, Left: 20}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = 200
			return widget.ItemProps{Title: "Câmeras", Pressable: true}.Layout(c.With(gtx), &row, nil)
		})
	}
	room := p.draw(screen).Size.Y - 100
	p.draw(screen)
	if room >= 44 {
		t.Fatalf("the row is %d points high, which is no test of a smaller one", room)
	}

	// Two points above the top of the row, and two below its foot.
	for _, y := range []int{100 - 2, 100 + room + 2} {
		before := pressed
		p.tap(image.Pt(60, y), screen)
		p.draw(screen)
		if pressed != before+1 {
			t.Errorf("a press at %d, 2 points off the row (100 to %d), did not reach it", y, 100+room)
		}
	}
}
