package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// TestCarouselStepsAreArrowsAndLeaveTheSlidesRoom: the steps were the words
// "Previous" and "Next", on a Portuguese screen, and took 150 points of the 358
// a phone has.
func TestCarouselStepsAreArrowsAndLeaveTheSlidesRoom(t *testing.T) {
	p := phone(300)
	var state widget.Carousel
	got := 0
	p.draw(func(c ayra.Context) ayra.Dimensions {
		return widget.CarouselProps{Count: 4, Arrows: true, PreviousLabel: "Anterior", NextLabel: "Próxima"}.
			Layout(c, &state, func(c ayra.Context, _ int) ayra.Dimensions {
				got = c.Constraints.Max.X
				return widget.TextProps{Content: "Slide"}.Layout(c)
			})
	})
	if got < 250 {
		t.Errorf("a slide has %d points between the steps of a 390-wide window, want at least 250", got)
	}
}
