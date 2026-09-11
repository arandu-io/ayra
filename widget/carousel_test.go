package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/layout"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// slides records which slides a carousel asked for and how much room each was
// given, and draws nothing.
//
// What it answers is the size argument, so that a test can put a tall slide
// beside short ones and ask what the window came out as -- which is the only
// way to tell a measured height from one taken out of the constraints.
type slides struct {
	asked       []int
	constraints []layout.Constraints
	heights     []int
}

func (s *slides) layout(c ayra.Context, index int) ayra.Dimensions {
	s.asked = append(s.asked, index)
	s.constraints = append(s.constraints, c.Constraints)

	height := 40
	if index < len(s.heights) {
		height = s.heights[index]
	}
	return ayra.Dimensions{Size: image.Pt(c.Constraints.Max.X, height)}
}

// TestACarouselOverNothingDrawsNothing keeps an empty one from taking room and
// putting controls on the screen that lead nowhere.
func TestACarouselOverNothingDrawsNothing(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}
	drawn := &slides{}

	dims := widget.CarouselProps{Count: 0, Arrows: true, Dots: true}.Layout(c, state, drawn.layout)

	if dims.Size.X != 0 || dims.Size.Y != 0 {
		t.Errorf("a carousel over nothing took %v", dims.Size)
	}
	if len(drawn.asked) != 0 {
		t.Errorf("a carousel over nothing asked for slides %v", drawn.asked)
	}
}

// TestACarouselWithNoSlideFunctionDrawsNothing keeps a screen that has not
// wired itself up yet from bringing down the frame.
func TestACarouselWithNoSlideFunctionDrawsNothing(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}

	dims := widget.CarouselProps{Count: 5, Arrows: true, Dots: true}.Layout(c, state, nil)

	if dims.Size.X != 0 || dims.Size.Y != 0 {
		t.Errorf("a carousel with nothing to draw took %v", dims.Size)
	}
}

// TestOnlyTheSlidesOnScreenAreLaidOut is what keeps a carousel over a hundred
// images from being a frame that misses.
//
// The ninety-nine that are not on screen would be measured, shaped and
// recorded on every frame, to be thrown away.
func TestOnlyTheSlidesOnScreenAreLaidOut(t *testing.T) {
	state := &widget.Carousel{}
	props := widget.CarouselProps{Count: 100, Arrows: true, Dots: true}

	c, _ := frame(t, theme.Light, 400)
	drawn := &slides{}
	props.Layout(c, state, drawn.layout)

	if len(drawn.asked) != 1 {
		t.Fatalf("a window of one asked for %d slides: %v", len(drawn.asked), drawn.asked)
	}

	state.Show(50)
	c, _ = frame(t, theme.Light, 400)
	drawn = &slides{}
	props.Layout(c, state, drawn.layout)

	if len(drawn.asked) != 1 || drawn.asked[0] != 50 {
		t.Errorf("at position 50 the carousel asked for %v", drawn.asked)
	}
}

// TestAWindowOfSeveralStopsOnTheLastSlide fixes the boundary a window wider
// than one slide has: the last position ends exactly on the last picture.
//
// One further and the row is two slides and a blank space, which reads as a
// carousel that has lost something rather than as one that has finished.
func TestAWindowOfSeveralStopsOnTheLastSlide(t *testing.T) {
	state := &widget.Carousel{}
	props := widget.CarouselProps{Count: 10, PerView: 3, Arrows: true}

	c, _ := frame(t, theme.Light, 400)
	props.Layout(c, state, (&slides{}).layout)

	for range 20 {
		state.Next()
	}
	if state.At() != 7 {
		t.Errorf("stepping to the end left the window at %d, want 7", state.At())
	}

	c, _ = frame(t, theme.Light, 400)
	drawn := &slides{}
	props.Layout(c, state, drawn.layout)

	want := []int{7, 8, 9}
	if len(drawn.asked) != len(want) {
		t.Fatalf("the last position asked for %v, want %v", drawn.asked, want)
	}
	for n, index := range want {
		if drawn.asked[n] != index {
			t.Fatalf("the last position asked for %v, want %v", drawn.asked, want)
		}
	}
}

// TestTheWindowIsAsTallAsItsTallestSlide fixes where the height comes from.
//
// Taken from the constraints instead it is nought in a row -- the window
// vanishes and the screen reads as a layout fault -- and the whole page in a
// column, which draws the carousel down the length of the screen. Both have
// been written in this package.
func TestTheWindowIsAsTallAsItsTallestSlide(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}
	drawn := &slides{heights: []int{20, 60, 40}}

	dims := widget.CarouselProps{Count: 3, PerView: 3}.Layout(c, state, drawn.layout)

	if dims.Size.Y != 60 {
		t.Errorf("a window over slides of 20, 60 and 40 came out %d tall, want 60", dims.Size.Y)
	}
}

// TestEverySlideIsGivenTheSameWidth fixes what the caller is handed to draw
// into: a fixed width, so that a slide fills its share rather than shrinking to
// whatever it happens to contain.
func TestEverySlideIsGivenTheSameWidth(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}
	drawn := &slides{}

	widget.CarouselProps{Count: 9, PerView: 3}.Layout(c, state, drawn.layout)

	if len(drawn.constraints) != 3 {
		t.Fatalf("three at a time laid out %d slides", len(drawn.constraints))
	}
	for n, cs := range drawn.constraints {
		if cs.Min.X != cs.Max.X {
			t.Errorf("slide %d was given a width between %d and %d", n, cs.Min.X, cs.Max.X)
		}
		if cs.Max.X <= 0 {
			t.Errorf("slide %d was given no width", n)
		}
		if cs.Max.X != drawn.constraints[0].Max.X {
			t.Errorf("slide %d was given %d of width and the first was given %d", n, cs.Max.X, drawn.constraints[0].Max.X)
		}
	}
	if total := drawn.constraints[0].Max.X * 3; total > 400 {
		t.Errorf("three slides of %d take %d of a 400 wide carousel", drawn.constraints[0].Max.X, total)
	}
}

// TestTheStepsNeverTakeTheWindowsRoom is the failure the pager already had, in
// the place a carousel has it.
//
// A row wider than its column loses what is on the right, and nothing about it
// looks broken -- the row simply ends. Here what would be lost is the picture.
func TestTheStepsNeverTakeTheWindowsRoom(t *testing.T) {
	measure := func(width int) int {
		c, _ := frame(t, theme.Light, width)
		drawn := &slides{}
		widget.CarouselProps{Count: 5, Arrows: true}.Layout(c, &widget.Carousel{}, drawn.layout)

		if len(drawn.constraints) != 1 {
			t.Fatalf("at %d wide the carousel laid out %d slides", width, len(drawn.constraints))
		}
		return drawn.constraints[0].Max.X
	}

	wide := measure(400)
	if wide <= 0 || wide >= 400 {
		t.Errorf("a wide carousel gave its window %d of 400: the steps drew or took everything", wide)
	}

	narrow := measure(120)
	if narrow <= 0 {
		t.Errorf("a narrow carousel gave its window %d: the steps squeezed the picture out", narrow)
	}
	if narrow != 120 {
		t.Errorf("a narrow carousel gave its window %d of 120; with no room for steps the window takes the row", narrow)
	}
}

// TestACarouselWithOnePositionDrawsNoDots keeps a mark that says nothing off
// the screen: one dot under a picture reads as a mark on the picture.
func TestACarouselWithOnePositionDrawsNoDots(t *testing.T) {
	height := func(count, perView int, dots bool) int {
		c, _ := frame(t, theme.Light, 400)
		props := widget.CarouselProps{Count: count, PerView: perView, Dots: dots}
		return props.Layout(c, &widget.Carousel{}, (&slides{}).layout).Size.Y
	}

	if with, without := height(1, 1, true), height(1, 1, false); with != without {
		t.Errorf("a carousel over one slide took %d with dots and %d without", with, without)
	}
	if with, without := height(3, 3, true), height(3, 3, false); with != without {
		t.Errorf("a carousel showing everything took %d with dots and %d without", with, without)
	}
	if with, without := height(5, 1, true), height(5, 1, false); with <= without {
		t.Errorf("a carousel with five positions took %d with dots and %d without", with, without)
	}
}

// TestSteppingClampsWithoutLoopingAndWrapsWithIt is the pair of rules a person
// pressing the same control twenty times depends on.
func TestSteppingClampsWithoutLoopingAndWrapsWithIt(t *testing.T) {
	lay := func(state *widget.Carousel, loop bool) {
		c, _ := frame(t, theme.Light, 400)
		widget.CarouselProps{Count: 3, Loop: loop, Arrows: true}.Layout(c, state, (&slides{}).layout)
	}

	clamped := &widget.Carousel{}
	lay(clamped, false)
	for range 10 {
		clamped.Next()
	}
	if clamped.At() != 2 {
		t.Errorf("ten steps on over three slides reached %d, want 2", clamped.At())
	}
	for range 10 {
		clamped.Previous()
	}
	if clamped.At() != 0 {
		t.Errorf("ten steps back over three slides reached %d, want 0", clamped.At())
	}

	wrapped := &widget.Carousel{}
	lay(wrapped, true)
	wrapped.Previous()
	if wrapped.At() != 2 {
		t.Errorf("a step back from the first looping slide reached %d, want 2", wrapped.At())
	}
	wrapped.Next()
	if wrapped.At() != 0 {
		t.Errorf("a step on from the last looping slide reached %d, want 0", wrapped.At())
	}
}

// TestAMoveIsReportedOnceAndConsumed keeps the caller from doing the work
// again on every frame for as long as nobody moves.
func TestAMoveIsReportedOnceAndConsumed(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}
	props := widget.CarouselProps{Count: 5, Arrows: true}
	props.Layout(c, state, (&slides{}).layout)

	if state.Changed() {
		t.Error("a carousel nobody has touched reported a move")
	}

	state.Next()
	if !state.Changed() {
		t.Fatal("a step reported no move")
	}
	if state.Changed() {
		t.Error("the same move was reported twice")
	}

	for range 3 {
		c, _ := frame(t, theme.Light, 400)
		props.Layout(c, state, (&slides{}).layout)
		if state.Changed() {
			t.Fatal("a carousel standing still reported a move on a later frame")
		}
	}

	// A step at the end goes nowhere, and going nowhere is not a move: reported,
	// it would have the caller load again the slide it is already showing.
	state.Show(4)
	state.Changed()
	state.Next()
	if state.Changed() {
		t.Error("a step with nowhere to go was reported as a move")
	}
}

// TestShowClampsRatherThanRefusing fixes what a position restored from
// somewhere else does when it no longer exists.
func TestShowClampsRatherThanRefusing(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}
	props := widget.CarouselProps{Count: 4, PerView: 2}
	props.Layout(c, state, (&slides{}).layout)

	state.Show(-7)
	if state.At() != 0 {
		t.Errorf("a negative position became %d, want 0", state.At())
	}

	state.Show(99)
	if state.At() != 2 {
		t.Errorf("a position past the end became %d, want 2", state.At())
	}

	// Written before anything says how many there are, and then brought inside
	// them by the frame that does.
	fresh := &widget.Carousel{}
	fresh.Show(99)
	c, _ = frame(t, theme.Light, 400)
	props.Layout(c, fresh, (&slides{}).layout)
	if fresh.At() != 2 {
		t.Errorf("a position written before the first frame settled at %d, want 2", fresh.At())
	}
}

// TestShowDoesNotReportItsOwnWrite keeps a screen from answering itself.
//
// A screen that wrote the position it was opened with and was then told the
// position changed would act on that, and what it does is another write.
func TestShowDoesNotReportItsOwnWrite(t *testing.T) {
	c, _ := frame(t, theme.Light, 400)
	state := &widget.Carousel{}
	widget.CarouselProps{Count: 5}.Layout(c, state, (&slides{}).layout)

	state.Show(3)

	if state.Changed() {
		t.Error("a screen writing the position it arrived with was told the position changed")
	}
}

// TestNothingAtTheEdgesPanics walks the shapes a carousel is given by a screen
// whose list has just arrived, just emptied, or never had more than one thing
// in it.
func TestNothingAtTheEdgesPanics(t *testing.T) {
	for _, count := range []int{-1, 0, 1, 2, 7} {
		for _, perView := range []int{-1, 0, 1, 3, 9} {
			for _, loop := range []bool{false, true} {
				for _, disabled := range []bool{false, true} {
					for _, width := range []int{0, 1, 40, 400} {
						c, _ := frame(t, theme.Light, width)
						state := &widget.Carousel{}
						state.Show(-4)
						state.Next()
						state.Previous()

						widget.CarouselProps{
							Count:    count,
							PerView:  perView,
							Loop:     loop,
							Dots:     true,
							Arrows:   true,
							Disabled: disabled,
						}.Layout(c, state, (&slides{}).layout)

						if state.At() < 0 {
							t.Fatalf("count=%d perView=%d width=%d left the window at %d",
								count, perView, width, state.At())
						}
					}
				}
			}
		}
	}
}
