package widget

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// carouselFrame builds the context a control is drawn into, without a window
// and without a GPU. A frame is a list of operations, and a list can be built
// and measured on a machine with no display -- which is what a build server is.
func carouselFrame(t *testing.T, width int) ayra.Context {
	t.Helper()

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(width, 1000)},
	}
	return ayra.Context{
		Context: gtx,
		Theme:   theme.New(theme.Light),
		Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// blank is a slide that takes the room it was given and draws nothing, for the
// tests that are about the arithmetic rather than about the picture.
func blank(c ayra.Context, index int) ayra.Dimensions {
	return ayra.Dimensions{Size: image.Pt(c.Constraints.Max.X, 40)}
}

// TestTheLastPositionStopsWhereTheSlidesDo fixes the rule that decides how far
// a window of more than one slide may travel.
//
// Counting positions to Count instead would put the last of them past the end,
// where the row is a slide or two and then blank space. That reads as a
// carousel that lost its pictures, not as one that reached its last.
func TestTheLastPositionStopsWhereTheSlidesDo(t *testing.T) {
	for _, test := range []struct {
		count, view, want int
	}{
		{count: 0, view: 1, want: 0},
		{count: 0, view: 5, want: 0},
		{count: 1, view: 1, want: 1},
		{count: 5, view: 1, want: 5},
		{count: 10, view: 3, want: 8},
		{count: 3, view: 3, want: 1},
		{count: 2, view: 5, want: 1},
		{count: -4, view: 1, want: 0},
	} {
		if got := positions(test.count, test.view); got != test.want {
			t.Errorf("%d slides %d at a time gives %d positions, want %d", test.count, test.view, got, test.want)
		}
	}
}

// TestTheWindowNeverRunsPastTheLastSlide is the same rule checked from the
// other end: at every position the window may reach, the slides it asks for
// exist.
//
// The boundary is the one that matters. At the last position the window has to
// end exactly on the last slide -- one short leaves a slide nobody can reach,
// and one long leaves blank space beside the final picture.
func TestTheWindowNeverRunsPastTheLastSlide(t *testing.T) {
	for _, count := range []int{1, 2, 3, 7, 10, 40} {
		for _, view := range []int{1, 2, 3, 5} {
			pages := positions(count, view)

			for first := 0; first < pages; first++ {
				from, howMany := shown(first, view, count, 0)

				if from+howMany > count {
					t.Errorf("%d slides %d at a time, position %d: wants slides %d..%d of %d",
						count, view, first, from, from+howMany-1, count)
				}
				if howMany <= 0 {
					t.Errorf("%d slides %d at a time, position %d: nothing to lay out", count, view, first)
				}
			}

			last := pages - 1
			from, howMany := shown(last, view, count, 0)
			if from+howMany != count {
				t.Errorf("%d slides %d at a time: the last position ends at %d, want %d",
					count, view, from+howMany, count)
			}
		}
	}
}

// TestAStepStopsAtTheEndsWhenItDoesNotLoop keeps a press that has nowhere to go
// from counting past the slides that exist.
func TestAStepStopsAtTheEndsWhenItDoesNotLoop(t *testing.T) {
	for _, test := range []struct {
		first, delta, positions, want int
	}{
		{first: 0, delta: -1, positions: 4, want: 0},
		{first: 0, delta: 1, positions: 4, want: 1},
		{first: 3, delta: 1, positions: 4, want: 3},
		{first: 2, delta: 1, positions: 4, want: 3},
		{first: 3, delta: -1, positions: 4, want: 2},
		{first: 0, delta: 1, positions: 1, want: 0},
		{first: 0, delta: -1, positions: 1, want: 0},
		{first: 0, delta: 1, positions: 0, want: 0},
	} {
		if got := stepTo(test.first, test.delta, test.positions, false); got != test.want {
			t.Errorf("%d stepping %+d over %d positions gave %d, want %d",
				test.first, test.delta, test.positions, got, test.want)
		}
	}
}

// TestAStepWrapsInBothDirections fixes the half of looping that is easy to
// leave out.
//
// Forwards is the one anybody writes: past the end, back to nought. Backwards
// is a signed remainder, and left as one it answers -1 -- a position that does
// not exist, reached by pressing the control that is meant to show the last
// picture.
func TestAStepWrapsInBothDirections(t *testing.T) {
	for _, test := range []struct {
		first, delta, positions, want int
	}{
		{first: 3, delta: 1, positions: 4, want: 0},
		{first: 0, delta: -1, positions: 4, want: 3},
		{first: 0, delta: 1, positions: 4, want: 1},
		{first: 2, delta: -1, positions: 4, want: 1},
		{first: 0, delta: -1, positions: 1, want: 0},
		{first: 0, delta: 1, positions: 1, want: 0},
	} {
		if got := stepTo(test.first, test.delta, test.positions, true); got != test.want {
			t.Errorf("%d stepping %+d over %d looping positions gave %d, want %d",
				test.first, test.delta, test.positions, got, test.want)
		}
	}
}

// TestAWrappedStepIsAlwaysAPositionThatExists is the same fact stated as a
// property, so that a wrap written another way has to answer for every start
// rather than for the two in a table.
func TestAWrappedStepIsAlwaysAPositionThatExists(t *testing.T) {
	for _, pages := range []int{1, 2, 3, 9} {
		for first := 0; first < pages; first++ {
			for _, delta := range []int{-1, 1} {
				got := stepTo(first, delta, pages, true)
				if got < 0 || got >= pages {
					t.Errorf("%d stepping %+d over %d positions left the carousel at %d",
						first, delta, pages, got)
				}
			}
		}
	}
}

// TestAPositionIsKeptInsideTheOnesThereAre fixes what an out-of-range position
// does: it is clamped, not refused and not a panic.
//
// A screen restoring where it was -- from a link, from a saved session, from a
// list that has since got shorter -- would otherwise have to know how many
// slides there are before it could say which one to open on.
func TestAPositionIsKeptInsideTheOnesThereAre(t *testing.T) {
	for _, test := range []struct {
		index, positions, want int
	}{
		{index: -5, positions: 4, want: 0},
		{index: -1, positions: 4, want: 0},
		{index: 0, positions: 4, want: 0},
		{index: 3, positions: 4, want: 3},
		{index: 4, positions: 4, want: 3},
		{index: 99, positions: 4, want: 3},
		{index: 99, positions: 1, want: 0},
		{index: -3, positions: 0, want: 0},
		// Nothing has said how many there are yet, so the position a screen
		// arrived with is kept rather than thrown away.
		{index: 7, positions: 0, want: 7},
	} {
		if got := clampFirst(test.index, test.positions); got != test.want {
			t.Errorf("%d over %d positions became %d, want %d", test.index, test.positions, got, test.want)
		}
	}
}

// TestThereIsNowhereToStepWhenEverythingIsOnScreen keeps a step that cannot
// move from being drawn as one that can.
func TestThereIsNowhereToStepWhenEverythingIsOnScreen(t *testing.T) {
	for _, loop := range []bool{false, true} {
		if canStep(0, 1, 1, loop) || canStep(0, -1, 1, loop) {
			t.Errorf("looping=%v: a carousel with one position offers a step", loop)
		}
		if canStep(0, 1, 0, loop) || canStep(0, -1, 0, loop) {
			t.Errorf("looping=%v: a carousel with no positions offers a step", loop)
		}
	}

	if canStep(0, -1, 4, false) {
		t.Error("the first position offers a step back without looping")
	}
	if canStep(3, 1, 4, false) {
		t.Error("the last position offers a step on without looping")
	}
	if !canStep(0, -1, 4, true) || !canStep(3, 1, 4, true) {
		t.Error("a looping carousel refuses a step at an end")
	}
	if !canStep(0, 1, 4, false) || !canStep(3, -1, 4, false) {
		t.Error("a step towards the middle was refused")
	}
}

// TestAStepWithNowhereToGoIsDrawnUnavailable fixes that the refusal is visible.
//
// A control that is drawn as available and does nothing is worse than one drawn
// as unavailable, because the screen says one thing and does another. Inert and
// disabled look identical in a screenshot and are not the same control.
func TestAStepWithNowhereToGoIsDrawnUnavailable(t *testing.T) {
	props := CarouselProps{Count: 1, Arrows: true}
	pages := positions(props.Count, props.view())

	back := props.arrow(previousStep, canStep(0, -1, pages, props.Loop))
	forward := props.arrow(nextStep, canStep(0, 1, pages, props.Loop))

	if !back.Disabled || !forward.Disabled {
		t.Errorf("a carousel with one position draws its steps as available: back=%v forward=%v",
			back.Disabled, forward.Disabled)
	}

	many := CarouselProps{Count: 5, Arrows: true}
	if step := many.arrow(nextStep, canStep(0, 1, positions(5, 1), false)); step.Disabled {
		t.Error("a carousel with somewhere to go draws its step as unavailable")
	}
	if step := many.arrow(previousStep, canStep(0, -1, positions(5, 1), false)); !step.Disabled {
		t.Error("the first position draws a step back as available")
	}

	off := CarouselProps{Count: 5, Arrows: true, Disabled: true}
	if step := off.arrow(nextStep, true); !step.Disabled {
		t.Error("a disabled carousel draws an available step")
	}
}

// TestOnlyTheWindowIsLaidOutUntilSomethingIsDragged fixes what the carousel
// asks the caller to build.
//
// Laying all of them out and clipping is what turns a hundred images into a
// frame that misses: the ninety-nine off screen get measured, shaped and
// recorded every frame, to be thrown away.
func TestOnlyTheWindowIsLaidOutUntilSomethingIsDragged(t *testing.T) {
	for _, test := range []struct {
		name                  string
		first, view, count    int
		travel                int
		wantFrom, wantHowMany int
	}{
		{name: "at rest", first: 0, view: 1, count: 100, wantFrom: 0, wantHowMany: 1},
		{name: "at rest, in the middle", first: 50, view: 1, count: 100, wantFrom: 50, wantHowMany: 1},
		{name: "at rest, three at a time", first: 2, view: 3, count: 100, wantFrom: 2, wantHowMany: 3},
		{name: "dragged forwards", first: 0, view: 1, count: 100, travel: -30, wantFrom: 0, wantHowMany: 2},
		{name: "dragged backwards", first: 5, view: 1, count: 100, travel: 30, wantFrom: 4, wantHowMany: 2},
		{name: "dragged backwards at the start", first: 0, view: 1, count: 100, travel: 30, wantFrom: 0, wantHowMany: 1},
		{name: "dragged forwards at the end", first: 99, view: 1, count: 100, travel: -30, wantFrom: 99, wantHowMany: 1},
		{name: "nothing to show", first: 0, view: 1, count: 0, wantFrom: 0, wantHowMany: 0},
		{name: "fewer slides than the window", first: 0, view: 5, count: 2, wantFrom: 0, wantHowMany: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			from, howMany := shown(test.first, test.view, test.count, test.travel)

			if from != test.wantFrom || howMany != test.wantHowMany {
				t.Errorf("laid out %d slides from %d, want %d from %d",
					howMany, from, test.wantHowMany, test.wantFrom)
			}
		})
	}
}

// TestADragCommitsAtHalfASlide fixes the distance that means "the next one".
//
// Half the slide rather than a number of points, so that the same flick
// advances a window of thumbnails and a window of photographs alike. Short of
// it the window goes back where it was, which is the whole reason somebody
// drags rather than presses.
func TestADragCommitsAtHalfASlide(t *testing.T) {
	const stride = 100

	for _, test := range []struct {
		travel, want int
	}{
		{travel: 0, want: 0},
		{travel: -49, want: 0},
		{travel: -50, want: 1},
		{travel: -100, want: 1},
		{travel: 49, want: 0},
		{travel: 50, want: -1},
		{travel: 100, want: -1},
	} {
		if got := dragged(test.travel, stride); got != test.want {
			t.Errorf("a drag of %d over a slide of %d moved %d positions, want %d",
				test.travel, stride, got, test.want)
		}
	}

	if got := dragged(500, 0); got != 0 {
		t.Errorf("a drag over a slide of no width moved %d positions", got)
	}
}

// TestTheStepsAreDroppedWhenTheyWouldTakeTheWindowsRoom keeps the control from
// becoming two buttons and a sliver.
//
// This is the failure the pager already had: a row wider than its column loses
// what is on the right, and nothing about it looks broken -- the row simply
// ends. Here the window is what gets lost instead, which is the thing the
// carousel exists to show.
func TestTheStepsAreDroppedWhenTheyWouldTakeTheWindowsRoom(t *testing.T) {
	if !arrowsFit(400, 150) {
		t.Error("steps taking under half a wide column were dropped")
	}
	if arrowsFit(200, 150) {
		t.Error("steps taking three quarters of a column were kept")
	}
	if !arrowsFit(300, 150) {
		t.Error("steps taking exactly half a column were dropped")
	}
	if !arrowsFit(0, 150) {
		t.Error("steps were dropped when there was no width to measure against")
	}
}

// TestADotRowMarksPositionsRatherThanSlides keeps a carousel over forty
// pictures from drawing forty marks, which is a texture rather than an
// indicator.
func TestADotRowMarksPositionsRatherThanSlides(t *testing.T) {
	props := CarouselProps{Count: 100, Dots: true}
	pages := positions(props.Count, props.view())

	marks := props.dotPages(50, pages)
	if len(marks) >= 20 {
		t.Errorf("a carousel over %d positions drew %d marks", pages, len(marks))
	}
	if marks[0] != 1 {
		t.Errorf("the row does not begin at the first position: %v", marks)
	}
	if marks[len(marks)-1] != pages {
		t.Errorf("the row does not end at the last position: %v", marks)
	}

	short := CarouselProps{Count: 3, Dots: true}.dotPages(0, 3)
	if len(short) != 3 {
		t.Errorf("three positions drew %d marks: %v", len(short), short)
	}
	for _, page := range short {
		if page == 0 {
			t.Errorf("three positions collapsed into a gap: %v", short)
		}
	}
}

// TestTheRunOfMarksIsCentredOnTheWindow fixes which positions the row keeps
// when it cannot keep them all.
//
// The pager counts from one and a position counts from nought, and the run it
// keeps is centred on the number it is handed. Handed the position unmoved, the
// row keeps a run sitting one place behind the window: the mark that is lit
// drifts towards the left edge of the group instead of staying in the middle
// of it, and the position just ahead -- the one somebody is about to reach --
// is the one dropped.
func TestTheRunOfMarksIsCentredOnTheWindow(t *testing.T) {
	props := CarouselProps{Count: 100, Dots: true}
	pages := positions(props.Count, props.view())

	const first = 50
	marks := props.dotPages(first, pages)

	drawn := map[int]bool{}
	for _, page := range marks {
		drawn[page] = true
	}

	// The window is on position 50, which the pager calls page 51, and a run of
	// two either side of it is pages 49 to 53.
	for page := first - 1; page <= first+3; page++ {
		if !drawn[page] {
			t.Errorf("page %d is not marked, and it is inside the run around the window: %v", page, marks)
		}
	}
	if drawn[first-2] {
		t.Errorf("page %d is marked, so the run sits behind the window rather than around it: %v", first-2, marks)
	}
}

// TestTheMarkedDotIsTheOneTheWindowIsOn fixes the one place in this control
// where a count from one and a count from nought meet.
//
// Compared without moving one of them, the mark before the window lights on
// every position but the first -- and on the last position nothing lights at
// all, which reads as a carousel that has come off its own track.
func TestTheMarkedDotIsTheOneTheWindowIsOn(t *testing.T) {
	th := theme.New(theme.Light)
	props := CarouselProps{Count: 5, Dots: true}
	pages := positions(props.Count, props.view())

	for first := 0; first < pages; first++ {
		lit := 0
		for _, page := range props.dotPages(first, pages) {
			if page == 0 {
				continue
			}
			if props.dotInk(th, page, first) == th.Colours.Foreground {
				lit++
				if page-1 != first {
					t.Errorf("at position %d the mark for position %d is lit", first, page-1)
				}
			}
		}
		if lit != 1 {
			t.Errorf("at position %d, %d marks are lit; exactly one says where the window is", first, lit)
		}
	}
}

// TestTheDotsAreRebuiltWhenTheNumberOfPositionsChanges keeps a press from
// landing on a slide nobody pointed at.
//
// A dot is identified by where it is in the row. Kept across a change, the
// button that was position seven of forty answers for position seven of three.
func TestTheDotsAreRebuiltWhenTheNumberOfPositionsChanges(t *testing.T) {
	state := &Carousel{}

	CarouselProps{Count: 40, Dots: true}.Layout(carouselFrame(t, 400), state, blank)
	if len(state.dots) != 40 {
		t.Fatalf("a carousel over 40 positions holds %d dots", len(state.dots))
	}

	CarouselProps{Count: 3, Dots: true}.Layout(carouselFrame(t, 400), state, blank)
	if len(state.dots) != 3 {
		t.Errorf("after the list got shorter the carousel holds %d dots, want 3", len(state.dots))
	}
	if state.first > 2 {
		t.Errorf("the window stayed at position %d of a carousel with three", state.first)
	}
}

// TestAPressOnAStepIsTakenOnTheFrameItIsMade keeps the wiring honest: the
// control that is drawn is the one that answers.
func TestAPressOnAStepIsTakenOnTheFrameItIsMade(t *testing.T) {
	state := &Carousel{}
	props := CarouselProps{Count: 5, Arrows: true}

	props.Layout(carouselFrame(t, 400), state, blank)

	state.next.click.Click()
	props.Layout(carouselFrame(t, 400), state, blank)
	if state.At() != 1 {
		t.Errorf("a press on the step forward left the window at %d, want 1", state.At())
	}
	if !state.Changed() {
		t.Error("a press on the step forward reported no move")
	}

	state.previous.click.Click()
	props.Layout(carouselFrame(t, 400), state, blank)
	if state.At() != 0 {
		t.Errorf("a press on the step back left the window at %d, want 0", state.At())
	}
}

// TestAPressOnAStepWithNowhereToGoMovesNothing is the half that is usually
// forgotten: the control is drawn as unavailable, and it also has to behave
// that way.
func TestAPressOnAStepWithNowhereToGoMovesNothing(t *testing.T) {
	state := &Carousel{}
	props := CarouselProps{Count: 3, Arrows: true}

	props.Layout(carouselFrame(t, 400), state, blank)
	state.Show(2)

	state.next.click.Click()
	props.Layout(carouselFrame(t, 400), state, blank)

	if state.At() != 2 {
		t.Errorf("the last position stepped on to %d", state.At())
	}
	if state.Changed() {
		t.Error("a press that moved nothing was reported as a move")
	}
}

// TestADisabledCarouselDoesNotAnswerAPress keeps an unavailable control from
// doing the thing it is drawn as unable to do.
func TestADisabledCarouselDoesNotAnswerAPress(t *testing.T) {
	state := &Carousel{}
	props := CarouselProps{Count: 5, Arrows: true, Dots: true, Disabled: true}

	props.Layout(carouselFrame(t, 400), state, blank)

	state.next.click.Click()
	if len(state.dots) > 1 {
		state.dots[3].click.Click()
	}
	props.Layout(carouselFrame(t, 400), state, blank)

	if state.At() != 0 {
		t.Errorf("a disabled carousel moved to %d", state.At())
	}
	if state.Changed() {
		t.Error("a disabled carousel reported a move")
	}
}

// TestAPressOnADotGoesToThatPosition fixes what the marks are for: they are
// navigation, not decoration.
func TestAPressOnADotGoesToThatPosition(t *testing.T) {
	state := &Carousel{}
	props := CarouselProps{Count: 5, Dots: true}

	props.Layout(carouselFrame(t, 400), state, blank)

	state.dots[3].click.Click()
	props.Layout(carouselFrame(t, 400), state, blank)

	if state.At() != 3 {
		t.Errorf("a press on the fourth mark left the window at %d, want 3", state.At())
	}
	if !state.Changed() {
		t.Error("a press on a mark reported no move")
	}
}

// TestTheWindowRefusesRoomItCannotDivide keeps the arithmetic from producing a
// stride every slide is drawn on top of.
func TestTheWindowRefusesRoomItCannotDivide(t *testing.T) {
	state := &Carousel{}
	props := CarouselProps{Count: 10, PerView: 8}

	dims := props.Layout(carouselFrame(t, 4), state, blank)

	if dims.Size.X != 0 || dims.Size.Y != 0 {
		t.Errorf("eight slides in four pixels took %v", dims.Size)
	}
}
