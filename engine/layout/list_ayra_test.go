package layout

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/op"
)

// listContext is the frame a list is laid out in, with no event source: a list
// that is only being measured never reads one.
func listContext(size image.Point) Context {
	return Context{Ops: new(op.Ops), Constraints: Constraints{Max: size}}
}

// visited records which indices a layout actually asked for, and refuses one
// that is not in the list at all.
//
// Out of range is the failure this whole family of tests exists to catch. A
// virtualised list computes which children to ask for from a saved position and
// a count that changed since, and the arithmetic that is one off asks for an
// element the caller does not have -- which in a real screen is an index into a
// slice and a panic in somebody else's code, not a wrong picture.
type visited struct {
	t     *testing.T
	len   int
	size  image.Point
	order []int
}

func (v *visited) element(gtx Context, index int) Dimensions {
	v.t.Helper()
	if index < 0 || index >= v.len {
		v.t.Fatalf("asked for element %d of a list of %d", index, v.len)
	}
	v.order = append(v.order, index)
	return Dimensions{Size: v.size}
}

// lowest and highest are the span of indices a frame touched.
func (v *visited) span() (int, int) {
	if len(v.order) == 0 {
		return -1, -1
	}
	lo, hi := v.order[0], v.order[0]
	for _, i := range v.order {
		lo = min(lo, i)
		hi = max(hi, i)
	}
	return lo, hi
}

// TestListDrawsWhatIntersectsTheViewport is the whole reason this list is
// virtualised: it reports as visible exactly the children that cross the
// viewport, whatever the total.
//
// The count is the load-bearing number -- a scrollbar is drawn from it, and so
// is every "showing x of y" a screen writes. One too many and the bar is short
// of the end it is already at; one too few and it never reaches it.
func TestListDrawsWhatIntersectsTheViewport(t *testing.T) {
	for _, tc := range []struct {
		name     string
		viewport int
		item     int
		len      int
		want     int
	}{
		{"exactly two fit", 20, 10, 10, 2},
		{"two and a sliver", 25, 10, 10, 3},
		{"one short item", 20, 30, 10, 1},
		{"fewer items than room", 100, 10, 3, 3},
		{"one item, one screen", 10, 10, 1, 1},
		{"nothing at all", 10, 10, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtx := listContext(image.Pt(tc.viewport, 10))
			seen := &visited{t: t, len: tc.len, size: image.Pt(tc.item, 10)}

			var l List
			l.Layout(gtx, tc.len, seen.element)

			if got := l.Position.Count; got != tc.want {
				t.Errorf("reported %d visible; want %d", got, tc.want)
			}
			if tc.len == 0 {
				return
			}
			// One extra child at each end is laid out on purpose, so that
			// something off screen can still be reached and scroll the list to
			// itself. Anything beyond that is a list measuring the whole
			// collection, which is what virtualising exists to avoid.
			if got, limit := len(seen.order), tc.want+2; got > limit {
				t.Errorf("laid out %d children for %d visible; want at most %d", got, tc.want, limit)
			}
			lo, hi := seen.span()
			if lo != 0 {
				t.Errorf("started at element %d; want 0", lo)
			}
			if hi >= tc.want+1 && hi >= tc.len {
				t.Errorf("reached element %d of %d", hi, tc.len)
			}
		})
	}
}

// TestListDoesNotMeasureWhatItDoesNotDraw is the same promise stated as a cost:
// a list of a hundred thousand rows must not call the caller a hundred thousand
// times per frame.
func TestListDoesNotMeasureWhatItDoesNotDraw(t *testing.T) {
	const many = 100000
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: many, size: image.Pt(10, 10)}

	var l List
	l.Layout(gtx, many, seen.element)

	if got := len(seen.order); got > 16 {
		t.Errorf("a list of %d measured %d children in one frame", many, got)
	}
	if got, want := l.Position.Count, 10; got != want {
		t.Errorf("reported %d visible; want %d", got, want)
	}
}

// TestListStartsWhereItWasToldTo checks the saved half of the position: a list
// handed a first index and an offset draws from there, and does not quietly
// begin at the top.
func TestListStartsWhereItWasToldTo(t *testing.T) {
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: 50, size: image.Pt(30, 10)}

	var l List
	l.Position.First = 10
	l.Position.Offset = 10
	l.Layout(gtx, 50, seen.element)

	if got, want := l.Position.First, 10; got != want {
		t.Errorf("first visible %d; want %d", got, want)
	}
	if got, want := l.Position.Offset, 10; got != want {
		t.Errorf("offset %d; want %d", got, want)
	}
	// Twenty pixels of the tenth remain, then three more of thirty cover the
	// hundred: four children cross the viewport.
	if got, want := l.Position.Count, 4; got != want {
		t.Errorf("reported %d visible; want %d", got, want)
	}
	if lo, _ := seen.span(); lo < 9 {
		t.Errorf("measured element %d, which is two before the first drawn", lo)
	}
}

// TestListReturnsToThePositionItLeft is the property a saved scroll position
// stands on: going to the end and coming back is the screen that was left.
//
// It has to survive the trip rather than merely be restorable, because the trip
// is where the position is rewritten -- the list clamps at the end, shifts the
// first index as children pass out of view, and folds the offset into it. A
// position that came back one pixel different is a list that creeps every time
// somebody scrolls down and up.
func TestListReturnsToThePositionItLeft(t *testing.T) {
	const len = 200
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: len, size: image.Pt(30, 10)}

	var l List
	l.Layout(gtx, len, seen.element)
	start := l.Position

	l.ScrollBy(len)
	l.Layout(gtx, len, seen.element)
	if l.Position.First == start.First {
		t.Fatalf("scrolling to the end left the first index at %d", start.First)
	}
	if got := l.Position.First + l.Position.Count; got > len {
		t.Errorf("the end runs to element %d of %d", got, len)
	}

	l.ScrollTo(0)
	l.Layout(gtx, len, seen.element)

	back := l.Position
	if back.First != start.First || back.Offset != start.Offset {
		t.Errorf("came back to first %d offset %d; left from first %d offset %d",
			back.First, back.Offset, start.First, start.Offset)
	}
	if back.Count != start.Count || back.OffsetLast != start.OffsetLast {
		t.Errorf("came back to count %d last %d; left from count %d last %d",
			back.Count, back.OffsetLast, start.Count, start.OffsetLast)
	}
}

// TestListLandsOnTheLastElementAtTheEnd checks that scrolling past the end
// stops at the end rather than running off it, and that the end is the end: the
// last element is the last one drawn.
func TestListLandsOnTheLastElementAtTheEnd(t *testing.T) {
	const len = 20
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: len, size: image.Pt(30, 10)}

	var l List
	l.ScrollBy(1000)
	l.Layout(gtx, len, seen.element)

	if got, want := l.Position.First+l.Position.Count, len; got != want {
		t.Errorf("the last drawn element is %d; want %d", got, want)
	}
	if l.Position.OffsetLast > 0 {
		t.Errorf("stopped %d pixels short of the end", l.Position.OffsetLast)
	}
	if _, hi := seen.span(); hi != len-1 {
		t.Errorf("the furthest element measured was %d; want %d", hi, len-1)
	}
}

// TestListDoesNotHangPastAnEndThatMovedIsTheShrinkCase.
//
// A saved position is a promise about a collection that no longer exists. The
// rows were filtered, a page was deleted, a search narrowed -- and the list is
// laid out again, in the same frame, against a count that is now smaller than
// the index it was sitting on.
//
// Two things have to hold, and the first is not the interesting one: no element
// outside the new collection may be asked for, which the helper enforces
// everywhere. The second is that the position that comes out is a position in
// the new collection -- because it is saved again at the end of the frame, and
// a first index past the end that survives one frame survives every frame after
// it, leaving a list that is permanently blank and cannot be scrolled back into
// view.
func TestListDoesNotHangPastAnEndThatMovedIsTheShrinkCase(t *testing.T) {
	for _, tc := range []struct {
		name  string
		was   int
		is    int
		item  int
		frame int
	}{
		{"many to few", 200, 3, 30, 100},
		{"many to one", 200, 1, 30, 100},
		{"many to none", 200, 0, 30, 100},
		{"still longer than the viewport", 200, 20, 30, 100},
		{"one fewer", 20, 19, 30, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtx := listContext(image.Pt(tc.frame, 10))

			var l List
			before := &visited{t: t, len: tc.was, size: image.Pt(tc.item, 10)}
			l.Layout(gtx, tc.was, before.element)
			l.ScrollBy(float32(tc.was))
			l.Layout(gtx, tc.was, before.element)

			after := &visited{t: t, len: tc.is, size: image.Pt(tc.item, 10)}
			l.Layout(gtx, tc.is, after.element)

			pos := l.Position
			if pos.First < 0 {
				t.Errorf("first index %d is before the start", pos.First)
			}
			if pos.First > tc.is {
				t.Errorf("first index %d is past a collection of %d", pos.First, tc.is)
			}
			if got := pos.First + pos.Count; got > tc.is {
				t.Errorf("drawing through element %d of %d", got, tc.is)
			}
			if tc.is > 0 && pos.Count == 0 {
				t.Errorf("a collection of %d drew nothing at all", tc.is)
			}

			// And it stays put: laying out again from the position just saved
			// changes nothing, because a position that has to settle over
			// several frames is a list that visibly jumps after every filter.
			settled := &visited{t: t, len: tc.is, size: image.Pt(tc.item, 10)}
			l.Layout(gtx, tc.is, settled.element)
			if l.Position != pos {
				t.Errorf("position settled at %+v then moved to %+v", pos, l.Position)
			}
		})
	}
}

// TestListShrinkingToNothingReportsNothing is the smallest case of the one
// above, kept separate because an empty collection is the state a screen is in
// before its first answer arrives and is therefore drawn constantly.
func TestListShrinkingToNothingReportsNothing(t *testing.T) {
	gtx := listContext(image.Pt(100, 10))

	var l List
	seen := &visited{t: t, len: 50, size: image.Pt(30, 10)}
	l.Layout(gtx, 50, seen.element)
	l.ScrollBy(50)
	l.Layout(gtx, 50, seen.element)

	dims := l.Layout(gtx, 0, nil)
	if l.Position.Count != 0 {
		t.Errorf("an empty list reported %d visible", l.Position.Count)
	}
	if l.Position.Length != 0 {
		t.Errorf("an empty list estimated a length of %d", l.Position.Length)
	}
	if want := gtx.Constraints.Min; dims.Size != want {
		t.Errorf("an empty list took %v; want %v", dims.Size, want)
	}
}

// TestListTakesTheRoomItsChildrenNeed covers the two directions and the space
// between children, which is the part that is easy to add once too often: a gap
// belongs between two children and not after the last one.
func TestListTakesTheRoomItsChildrenNeed(t *testing.T) {
	for _, tc := range []struct {
		name string
		axis Axis
		gap  int
		len  int
		item image.Point
		room image.Point
		want int
	}{
		{"two across with a gap", Horizontal, 5, 2, image.Pt(10, 10), image.Pt(100, 20), 25},
		{"three across with a gap", Horizontal, 5, 3, image.Pt(10, 10), image.Pt(100, 20), 40},
		{"one across, no gap after it", Horizontal, 5, 1, image.Pt(10, 10), image.Pt(100, 20), 10},
		{"none at all", Horizontal, 5, 0, image.Pt(10, 10), image.Pt(100, 20), 0},
		{"three down with a gap", Vertical, 10, 3, image.Pt(10, 15), image.Pt(20, 100), 65},
		{"no gap at all", Horizontal, 0, 3, image.Pt(10, 10), image.Pt(100, 20), 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtx := listContext(tc.room)
			l := List{Axis: tc.axis, Gap: tc.gap}
			seen := &visited{t: t, len: tc.len, size: tc.item}

			var element ListElement
			if tc.len > 0 {
				element = seen.element
			}
			dims := l.Layout(gtx, tc.len, element)

			if got := tc.axis.Convert(dims.Size).X; got != tc.want {
				t.Errorf("took %d along the axis; want %d", got, tc.want)
			}
		})
	}
}

// TestListCountsTheGapAgainstTheViewport fixes that the space between children
// is room a child cannot be in. Counted only in the total and not in the fill,
// a list reports one more visible child than a person can see.
func TestListCountsTheGapAgainstTheViewport(t *testing.T) {
	gtx := listContext(image.Pt(30, 20))
	l := List{Gap: 5}
	seen := &visited{t: t, len: 5, size: image.Pt(10, 10)}

	l.Layout(gtx, 5, seen.element)

	// Ten, then twenty five, then forty: the third child crosses the thirty
	// pixel viewport and the fourth begins outside it.
	if got, want := l.Position.Count, 3; got != want {
		t.Errorf("reported %d visible; want %d", got, want)
	}
	if got, want := l.Position.First, 0; got != want {
		t.Errorf("first visible %d; want %d", got, want)
	}
	if got, want := l.Position.OffsetLast, -10; got != want {
		t.Errorf("trailing offset %d; want %d", got, want)
	}
}

// TestListEstimatesTheWholeFromWhatItSaw is what a scrollbar's size comes from.
//
// The list never measures the collection, so the total is an estimate: the
// average of the children it did lay out, times the count, plus the gaps. It is
// exact for children of one size, which is the common case, and it is the only
// number available for any other.
func TestListEstimatesTheWholeFromWhatItSaw(t *testing.T) {
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: 50, size: image.Pt(20, 10)}

	l := List{Gap: 4}
	l.Layout(gtx, 50, seen.element)

	if got, want := l.Position.Length, 50*20+49*4; got != want {
		t.Errorf("estimated %d for the whole; want %d", got, want)
	}
}

// TestListHeldAtTheEndFollowsIt is the behaviour a log or a conversation is
// built on: new content appended below is scrolled to, until somebody scrolls
// away, and then it is not.
func TestListHeldAtTheEndFollowsIt(t *testing.T) {
	gtx := Context{Ops: new(op.Ops), Constraints: Exact(image.Pt(20, 10))}
	l := List{ScrollToEnd: true}

	l.Layout(gtx, 1, func(gtx Context, index int) Dimensions {
		return Dimensions{Size: image.Pt(10, 10)}
	})
	// One child of ten in a viewport of twenty, pinned to the far end: the
	// content begins ten pixels before the leading edge.
	if got, want := l.Position.Offset, -10; got != want {
		t.Errorf("offset %d; want %d", got, want)
	}
	if l.Position.BeforeEnd {
		t.Error("a list pinned to the end reported itself away from it")
	}
}

// TestListRefusesAPositionOutsideTheCollection covers the two ways a saved
// position arrives impossible: before the start, and past the end. Both are
// values a caller can set, so neither is a state the list may assume away.
func TestListRefusesAPositionOutsideTheCollection(t *testing.T) {
	const len = 3
	gtx := Context{Ops: new(op.Ops), Constraints: Exact(image.Pt(20, 10))}

	for _, first := range []int{-1, -1000, len + 1, len + 1000} {
		var l List
		seen := &visited{t: t, len: len, size: image.Point{}}
		l.Position.First = first
		l.Layout(gtx, len, seen.element)

		if l.Position.First < 0 || l.Position.First > len {
			t.Errorf("a first index of %d became %d", first, l.Position.First)
		}
	}
}

// TestScrollToPutsAnElementAtTheLeadingEdge fixes what "scroll to item n" means:
// the element starts where the list starts, with nothing of it cut off.
func TestScrollToPutsAnElementAtTheLeadingEdge(t *testing.T) {
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: 50, size: image.Pt(30, 10)}

	var l List
	l.ScrollTo(12)
	l.Layout(gtx, 50, seen.element)

	if got, want := l.Position.First, 12; got != want {
		t.Errorf("first visible %d; want %d", got, want)
	}
	if got := l.Position.Offset; got != 0 {
		t.Errorf("the element starts %d pixels off the edge; want 0", got)
	}
	if !l.Position.BeforeEnd {
		t.Error("scrolling to an element in the middle reported the list at its end")
	}
}

// TestScrollByMovesByElements checks the whole-number half of scrolling by
// count, which is what a page up, a page down and a keyboard both go through.
func TestScrollByMovesByElements(t *testing.T) {
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: 200, size: image.Pt(30, 10)}

	var l List
	l.Layout(gtx, 200, seen.element)

	l.ScrollBy(10)
	l.Layout(gtx, 200, seen.element)
	if got, want := l.Position.First, 10; got != want {
		t.Errorf("after ten forward, first visible %d; want %d", got, want)
	}

	l.ScrollBy(-4)
	l.Layout(gtx, 200, seen.element)
	if got, want := l.Position.First, 6; got != want {
		t.Errorf("after four back, first visible %d; want %d", got, want)
	}
}

// TestScrollByMovesByPartOfAnElement covers the fractional half, which is what
// a scrollbar dragged by hand produces: a position between two elements.
func TestScrollByMovesByPartOfAnElement(t *testing.T) {
	gtx := listContext(image.Pt(100, 10))
	seen := &visited{t: t, len: 200, size: image.Pt(30, 10)}

	var l List
	l.Layout(gtx, 200, seen.element)

	l.ScrollBy(2.5)
	l.Layout(gtx, 200, seen.element)

	// Half of a thirty pixel element is fifteen, which is inside the third one
	// and not past it.
	if got, want := l.Position.First, 2; got != want {
		t.Errorf("first visible %d; want %d", got, want)
	}
	if got, want := l.Position.Offset, 15; got != want {
		t.Errorf("offset %d; want %d", got, want)
	}
}

// TestListStartsIdle guards the state a list reports before anything has
// touched it, which is what a screen asks to decide whether it may take the
// pointer for something else.
func TestListStartsIdle(t *testing.T) {
	var l List
	if l.Dragging() {
		t.Error("a list nobody touched reported itself dragged")
	}
}

// TestListMovesWhatArrivesFromThePointer is the one case with a real event
// source behind it, and it is here because everything above measures a list
// that nothing is scrolling.
//
// The frame boundary matters: filters are declared while drawing and events are
// delivered against the ones declared last frame, so a list laid out once has
// not listened yet. Two frames is the smallest script that moves anything.
func TestListMovesWhatArrivesFromThePointer(t *testing.T) {
	var router input.Router
	gtx := Context{
		Ops:         new(op.Ops),
		Constraints: Constraints{Max: image.Pt(20, 10)},
		Source:      router.Source(),
	}
	seen := &visited{t: t, len: 3, size: image.Pt(10, 10)}

	var l List
	l.Layout(gtx, 3, seen.element)
	router.Frame(gtx.Ops)
	router.Queue(
		pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Kind: pointer.Press, Position: f32.Pt(0, 0)},
		pointer.Event{Source: pointer.Mouse, Kind: pointer.Scroll, Scroll: f32.Pt(10, 0)},
		pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Kind: pointer.Release, Position: f32.Pt(15, 0)},
	)
	gtx.Ops.Reset()
	l.Layout(gtx, 3, seen.element)

	if got, want := l.Position.First, 1; got != want {
		t.Errorf("after a wheel of one element, first visible %d; want %d", got, want)
	}
	if got, want := l.Position.Count, 2; got != want {
		t.Errorf("reported %d visible; want %d", got, want)
	}
	if got := l.Position.Offset; got != 0 {
		t.Errorf("offset %d; want 0", got)
	}
}

// TestListStopsAtTheEndsItWasGiven checks the boundary declared to the pointer
// rather than applied to the answer: a list already at its start refuses a
// wheel that would take it further back, so that whatever is behind it -- a page
// under a list that has run out -- gets the event instead.
func TestListStopsAtTheEndsItWasGiven(t *testing.T) {
	var router input.Router
	gtx := Context{
		Ops:         new(op.Ops),
		Constraints: Constraints{Max: image.Pt(20, 10)},
		Source:      router.Source(),
	}
	seen := &visited{t: t, len: 3, size: image.Pt(10, 10)}

	var l List
	l.Layout(gtx, 3, seen.element)
	router.Frame(gtx.Ops)
	router.Queue(scrollBackwards()...)
	gtx.Ops.Reset()
	l.Layout(gtx, 3, seen.element)

	if got := l.Position.First; got != 0 {
		t.Errorf("scrolling back from the start moved to element %d", got)
	}
	if got := l.Position.Offset; got < 0 {
		t.Errorf("scrolling back from the start left the offset at %d", got)
	}
}

// scrollBackwards is a wheel turned towards the start of the list.
func scrollBackwards() []event.Event {
	return []event.Event{
		pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Kind: pointer.Press, Position: f32.Pt(0, 0)},
		pointer.Event{Source: pointer.Mouse, Kind: pointer.Scroll, Scroll: f32.Pt(-50, 0)},
		pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Kind: pointer.Release, Position: f32.Pt(0, 0)},
	}
}

// TestListAlignsAcrossTheAxis fixes the cross axis placement, which is the one
// thing a list does to a child that is narrower than the others.
func TestListAlignsAcrossTheAxis(t *testing.T) {
	for _, alignment := range []Alignment{Start, End, Middle} {
		gtx := listContext(image.Pt(100, 40))
		l := List{Alignment: alignment}
		dims := l.Layout(gtx, 2, func(gtx Context, index int) Dimensions {
			if index == 0 {
				return Dimensions{Size: image.Pt(10, 40)}
			}
			return Dimensions{Size: image.Pt(10, 10)}
		})
		// Whatever the alignment, the list is as tall as its tallest child:
		// alignment moves a child inside the row and never changes the row.
		if got, want := dims.Size.Y, 40; got != want {
			t.Errorf("%v: took %d across; want %d", alignment, got, want)
		}
	}
}

// TestListFillsTheMinimumItWasGiven says a list is a box like any other: given
// a fixed size it reports that size, even with less content than fills it.
func TestListFillsTheMinimumItWasGiven(t *testing.T) {
	gtx := Context{Ops: new(op.Ops), Constraints: Exact(image.Pt(100, 40))}
	var l List
	dims := l.Layout(gtx, 1, func(gtx Context, index int) Dimensions {
		return Dimensions{Size: image.Pt(10, 10)}
	})
	if want := image.Pt(100, 40); dims.Size != want {
		t.Errorf("took %v; want %v", dims.Size, want)
	}
}

// TestListNeverReportsMoreThanTheViewport guards the other side of the same
// box: a list of far more content than fits reports the viewport, not the
// content. Reporting the content makes every parent lay out around a list as
// tall as the collection.
func TestListNeverReportsMoreThanTheViewport(t *testing.T) {
	room := image.Pt(100, 40)
	gtx := listContext(room)
	seen := &visited{t: t, len: 1000, size: image.Pt(30, 40)}

	var l List
	dims := l.Layout(gtx, 1000, seen.element)

	if dims.Size.X > room.X || dims.Size.Y > room.Y {
		t.Errorf("took %v out of %v", dims.Size, room)
	}
}
