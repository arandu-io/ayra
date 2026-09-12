package layout

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
)

// insetContext is the frame every inset case below is laid out in.
//
// One pixel per dp, so that an inset written in dp and a constraint written in
// pixels are the same numbers and a wrong answer is arithmetic rather than a
// conversion.
func insetContext(cs Constraints) Context {
	return Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{},
		Constraints: cs,
	}
}

// TestInsetTakesExactlyWhatItAsksFor is the whole promise of an inset: the
// child is offered the room that is left, and what comes back is what the child
// took plus the room the inset kept.
//
// Both halves are asserted together because either one alone passes while the
// pair is wrong: an inset that shrank the constraints and forgot to add the
// space back reports a control smaller than it drew, and everything placed
// after it lands on top of it.
func TestInsetTakesExactlyWhatItAsksFor(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Inset
		cs   Constraints
	}{
		{"uniform", UniformInset(10), Constraints{Max: image.Pt(100, 100)}},
		{"asymmetric", Inset{Top: 1, Right: 2, Bottom: 3, Left: 4}, Constraints{Max: image.Pt(100, 100)}},
		{"none", Inset{}, Constraints{Max: image.Pt(100, 100)}},
		{"with min", UniformInset(5), Constraints{Min: image.Pt(20, 20), Max: image.Pt(100, 100)}},
		{"one side", Inset{Left: 30}, Constraints{Max: image.Pt(100, 100)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtx := insetContext(tc.cs)
			before := gtx.Constraints

			horizontal := gtx.Dp(tc.in.Left) + gtx.Dp(tc.in.Right)
			vertical := gtx.Dp(tc.in.Top) + gtx.Dp(tc.in.Bottom)

			var inner Constraints
			const childBaseline = 3
			child := image.Pt(7, 9)
			dims := tc.in.Layout(gtx, func(gtx Context) Dimensions {
				inner = gtx.Constraints
				return Dimensions{Size: child, Baseline: childBaseline}
			})

			wantMax := before.Max.Sub(image.Pt(horizontal, vertical))
			if inner.Max != wantMax {
				t.Errorf("child offered max %v; want %v", inner.Max, wantMax)
			}
			if inner.Min != before.Min {
				t.Errorf("child offered min %v; want %v", inner.Min, before.Min)
			}
			if want := child.Add(image.Pt(horizontal, vertical)); dims.Size != want {
				t.Errorf("reported size %v; want %v", dims.Size, want)
			}
			if want := childBaseline + gtx.Dp(tc.in.Bottom); dims.Baseline != want {
				t.Errorf("reported baseline %d; want %d", dims.Baseline, want)
			}
			if gtx.Constraints != before {
				t.Errorf("caller constraints became %v; want %v", gtx.Constraints, before)
			}
		})
	}
}

// TestInsetNeverOffersNegativeRoom covers the case the arithmetic makes easy to
// get wrong: an inset wider than the space it was given.
//
// Subtraction alone lands below zero, and a negative maximum is not a small
// control -- it is a constraint no size satisfies, and the child that reads it
// draws off the screen or divides by it. The collapsed axis gives up the inset
// entirely rather than keeping half of it, so that the reported size is the
// child's own and not the child's plus padding that was never drawn.
func TestInsetNeverOffersNegativeRoom(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Inset
		cs   Constraints
	}{
		{"wider than max", UniformInset(200), Constraints{Max: image.Pt(100, 100)}},
		{"exactly the max", UniformInset(50), Constraints{Max: image.Pt(100, 100)}},
		{"wider than a fixed size", UniformInset(40), Exact(image.Pt(50, 50))},
		{"no room at all", UniformInset(1), Constraints{}},
		{"horizontal only", Inset{Left: 80, Right: 80}, Constraints{Max: image.Pt(100, 100)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtx := insetContext(tc.cs)

			var inner Constraints
			dims := tc.in.Layout(gtx, func(gtx Context) Dimensions {
				inner = gtx.Constraints
				return Dimensions{Size: gtx.Constraints.Min}
			})

			if inner.Max.X < 0 || inner.Max.Y < 0 {
				t.Errorf("child offered negative max %v", inner.Max)
			}
			if inner.Min.X < 0 || inner.Min.Y < 0 {
				t.Errorf("child offered negative min %v", inner.Min)
			}
			if inner.Min.X > inner.Max.X || inner.Min.Y > inner.Max.Y {
				t.Errorf("child offered min %v above max %v", inner.Min, inner.Max)
			}
			if dims.Size.X < 0 || dims.Size.Y < 0 {
				t.Errorf("reported negative size %v", dims.Size)
			}
			if inner.Max.X == 0 && dims.Size.X != 0 {
				t.Errorf("collapsed axis still reported width %d; want 0", dims.Size.X)
			}
			if inner.Max.Y == 0 && dims.Size.Y != 0 {
				t.Errorf("collapsed axis still reported height %d; want 0", dims.Size.Y)
			}
		})
	}
}

// TestUniformInsetIsTheSameOnFourSides fixes what the name promises. It is one
// line of code and it has one way to be wrong -- a side left out -- which is
// invisible on any screen whose content happens to be centred anyway.
func TestUniformInsetIsTheSameOnFourSides(t *testing.T) {
	in := UniformInset(12)
	if want := (Inset{Top: 12, Right: 12, Bottom: 12, Left: 12}); in != want {
		t.Errorf("UniformInset(12) = %+v; want %+v", in, want)
	}
}

// TestInsetScalesWithTheDisplay checks that an inset is a length and not a
// pixel count: the same Inset on a denser display keeps more room.
func TestInsetScalesWithTheDisplay(t *testing.T) {
	gtx := Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 2},
		Constraints: Constraints{Max: image.Pt(100, 100)},
	}
	var inner Constraints
	UniformInset(10).Layout(gtx, func(gtx Context) Dimensions {
		inner = gtx.Constraints
		return Dimensions{}
	})
	if want := image.Pt(60, 60); inner.Max != want {
		t.Errorf("child offered max %v at two pixels per dp; want %v", inner.Max, want)
	}
}

// TestConstrainClampsBothWays fixes the one operation every layout in the
// package leans on.
func TestConstrainClampsBothWays(t *testing.T) {
	cs := Constraints{Min: image.Pt(10, 20), Max: image.Pt(50, 60)}
	for _, tc := range []struct{ in, want image.Point }{
		{image.Pt(0, 0), image.Pt(10, 20)},
		{image.Pt(30, 30), image.Pt(30, 30)},
		{image.Pt(100, 100), image.Pt(50, 60)},
		{image.Pt(-5, 100), image.Pt(10, 60)},
		{image.Pt(100, -5), image.Pt(50, 20)},
	} {
		if got := cs.Constrain(tc.in); got != tc.want {
			t.Errorf("Constrain(%v) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

// TestConstrainIsIdempotent says a constrained size is already a legal one.
// Anything else means a single pass through a layout is not enough, and no
// caller runs two.
func TestConstrainIsIdempotent(t *testing.T) {
	cs := Constraints{Min: image.Pt(10, 20), Max: image.Pt(50, 60)}
	for _, p := range []image.Point{{}, {X: 100, Y: 5}, {X: 25, Y: 25}, {X: -40, Y: 400}} {
		once := cs.Constrain(p)
		if twice := cs.Constrain(once); twice != once {
			t.Errorf("Constrain(%v) settled at %v then moved to %v", p, once, twice)
		}
	}
}

// TestExactLeavesOneChoice is what a fixed size means: minimum and maximum are
// the same, so every size constrained by it is that size.
func TestExactLeavesOneChoice(t *testing.T) {
	size := image.Pt(30, 40)
	cs := Exact(size)
	if cs.Min != size || cs.Max != size {
		t.Errorf("Exact(%v) = %+v", size, cs)
	}
	for _, p := range []image.Point{{}, {X: 1000, Y: 1000}, {X: -1, Y: 7}} {
		if got := cs.Constrain(p); got != size {
			t.Errorf("Constrain(%v) under a fixed size = %v; want %v", p, got, size)
		}
	}
}

// TestAddMinAndSubMaxStayInRange fixes the two adjustments a parent makes when
// it has already spent part of the room on itself.
//
// The invariants are the same for both: no side goes below zero, and the
// minimum never ends up above the maximum. A minimum above a maximum is a
// constraint with no size in it, and a child asked for one returns whichever of
// the two it read first.
func TestAddMinAndSubMaxStayInRange(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     Constraints
		subMax image.Point
		addMin image.Point
		want   Constraints
	}{
		{
			name:   "shrink the max",
			in:     Constraints{Max: image.Pt(100, 100)},
			subMax: image.Pt(25, 25),
			want:   Constraints{Max: image.Pt(75, 75)},
		},
		{
			name:   "shrink the max under the min",
			in:     Constraints{Min: image.Pt(50, 50), Max: image.Pt(100, 100)},
			subMax: image.Pt(75, 75),
			want:   Constraints{Min: image.Pt(25, 25), Max: image.Pt(25, 25)},
		},
		{
			name:   "shrink the max past nothing",
			in:     Constraints{Min: image.Pt(50, 50), Max: image.Pt(100, 100)},
			subMax: image.Pt(125, 125),
			want:   Constraints{},
		},
		{
			name:   "grow the min",
			in:     Constraints{Max: image.Pt(100, 100)},
			addMin: image.Pt(25, 25),
			want:   Constraints{Min: image.Pt(25, 25), Max: image.Pt(100, 100)},
		},
		{
			name:   "grow the min past the max",
			in:     Constraints{Max: image.Pt(100, 100)},
			addMin: image.Pt(125, 125),
			want:   Constraints{Min: image.Pt(100, 100), Max: image.Pt(100, 100)},
		},
		{
			name:   "shrink the min past nothing",
			in:     Constraints{Min: image.Pt(50, 50), Max: image.Pt(100, 100)},
			addMin: image.Pt(-125, -125),
			want:   Constraints{Max: image.Pt(100, 100)},
		},
		{
			name:   "both, in order",
			in:     Constraints{Max: image.Pt(100, 100)},
			subMax: image.Pt(40, 10),
			addMin: image.Pt(80, 80),
			want:   Constraints{Min: image.Pt(60, 80), Max: image.Pt(60, 90)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			if tc.subMax != (image.Point{}) {
				got = got.SubMax(tc.subMax)
			}
			if tc.addMin != (image.Point{}) {
				got = got.AddMin(tc.addMin)
			}
			if got != tc.want {
				t.Errorf("got %+v; want %+v", got, tc.want)
			}
			if got.Min.X < 0 || got.Min.Y < 0 || got.Max.X < 0 || got.Max.Y < 0 {
				t.Errorf("a side went negative: %+v", got)
			}
			if got.Min.X > got.Max.X || got.Min.Y > got.Max.Y {
				t.Errorf("min above max: %+v", got)
			}
		})
	}
}

// TestDirectionClearsOnlyTheAxisItPlacesAlong is what lets a direction place
// anything at all.
//
// A child held to the room's own minimum fills the room, and a child that fills
// the room has nowhere to be placed inside it. The edges clear one axis only:
// a control pinned to the top still spans the width, which is what "top" means
// on a screen and not "top left".
func TestDirectionClearsOnlyTheAxisItPlacesAlong(t *testing.T) {
	room := image.Pt(100, 100)
	for _, tc := range []struct {
		dir  Direction
		want image.Point
	}{
		{N, image.Pt(100, 0)},
		{S, image.Pt(100, 0)},
		{E, image.Pt(0, 100)},
		{W, image.Pt(0, 100)},
		{NW, image.Point{}},
		{NE, image.Point{}},
		{SE, image.Point{}},
		{SW, image.Point{}},
		{Center, image.Point{}},
	} {
		t.Run(tc.dir.String(), func(t *testing.T) {
			gtx := Context{Ops: new(op.Ops), Constraints: Exact(room)}
			var min image.Point
			dims := tc.dir.Layout(gtx, func(gtx Context) Dimensions {
				min = gtx.Constraints.Min
				return Dimensions{}
			})
			if min != tc.want {
				t.Errorf("child offered min %v; want %v", min, tc.want)
			}
			if dims.Size != room {
				t.Errorf("took %v of the room; want %v", dims.Size, room)
			}
		})
	}
}

// TestDirectionPlacesAtTheNamedCorner fixes the arithmetic of the nine
// positions. Each one is a corner, an edge or the middle, and the failure they
// all share is a swapped axis -- which looks right on a square.
func TestDirectionPlacesAtTheNamedCorner(t *testing.T) {
	widget := image.Pt(20, 10)
	bounds := image.Pt(100, 50)
	for _, tc := range []struct {
		dir  Direction
		want image.Point
	}{
		{NW, image.Pt(0, 0)},
		{N, image.Pt(40, 0)},
		{NE, image.Pt(80, 0)},
		{E, image.Pt(80, 20)},
		{SE, image.Pt(80, 40)},
		{S, image.Pt(40, 40)},
		{SW, image.Pt(0, 40)},
		{W, image.Pt(0, 20)},
		{Center, image.Pt(40, 20)},
	} {
		t.Run(tc.dir.String(), func(t *testing.T) {
			if got := tc.dir.Position(widget, bounds); got != tc.want {
				t.Errorf("Position = %v; want %v", got, tc.want)
			}
		})
	}
}

// TestDirectionGrowsToTheMinimumItWasGiven checks the other half of placing: a
// small child inside a large room reports the room, because that is the space
// it was placed in and the space the parent has to skip.
func TestDirectionGrowsToTheMinimumItWasGiven(t *testing.T) {
	gtx := Context{Ops: new(op.Ops), Constraints: Exact(image.Pt(100, 50))}
	dims := Center.Layout(gtx, func(gtx Context) Dimensions {
		return Dimensions{Size: image.Pt(10, 10), Baseline: 2}
	})
	if want := image.Pt(100, 50); dims.Size != want {
		t.Errorf("size %v; want %v", dims.Size, want)
	}
	// The child sits 20 pixels down in a 50 pixel room, so its baseline is 20
	// further from the bottom than it was in its own box.
	if want := 2 + 50 - 10 - 20; dims.Baseline != want {
		t.Errorf("baseline %d; want %d", dims.Baseline, want)
	}
}

// TestSpacerTakesItsSizeFromTheConstraints says a spacer is a size and not a
// demand: a gap larger than the room left is the room left.
func TestSpacerTakesItsSizeFromTheConstraints(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Spacer
		cs   Constraints
		want image.Point
	}{
		{"fits", Spacer{Width: 10, Height: 4}, Constraints{Max: image.Pt(100, 100)}, image.Pt(10, 4)},
		{"clipped", Spacer{Width: 400, Height: 400}, Constraints{Max: image.Pt(100, 100)}, image.Pt(100, 100)},
		{"stretched to the min", Spacer{}, Constraints{Min: image.Pt(5, 5), Max: image.Pt(100, 100)}, image.Pt(5, 5)},
		{"nothing at all", Spacer{}, Constraints{Max: image.Pt(100, 100)}, image.Point{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gtx := insetContext(tc.cs)
			if got := tc.in.Layout(gtx).Size; got != tc.want {
				t.Errorf("size %v; want %v", got, tc.want)
			}
		})
	}
}

// TestAxisConvertIsItsOwnInverse is why one piece of layout arithmetic serves
// both directions: a vertical layout is a horizontal one read through a swap,
// and a swap applied twice is the point that went in.
func TestAxisConvertIsItsOwnInverse(t *testing.T) {
	for _, a := range []Axis{Horizontal, Vertical} {
		for _, p := range []image.Point{{}, {X: 3, Y: 7}, {X: -1, Y: 100}} {
			if got := a.Convert(a.Convert(p)); got != p {
				t.Errorf("%v: Convert twice on %v gave %v", a, p, got)
			}
			f := f32.Pt(float32(p.X), float32(p.Y))
			if got := a.FConvert(a.FConvert(f)); got != f {
				t.Errorf("%v: FConvert twice on %v gave %v", a, f, got)
			}
		}
	}
	if got := Vertical.Convert(image.Pt(3, 7)); got != image.Pt(7, 3) {
		t.Errorf("Vertical.Convert did not swap: %v", got)
	}
	if got := Horizontal.Convert(image.Pt(3, 7)); got != image.Pt(3, 7) {
		t.Errorf("Horizontal.Convert did not leave the point alone: %v", got)
	}
}

// TestAxisReadsTheConstraintOfItsOwnDirection fixes the pair of readers every
// multi-child layout in the package measures with. They differ only by which
// field they take, which is exactly the mistake that compiles.
func TestAxisReadsTheConstraintOfItsOwnDirection(t *testing.T) {
	cs := Constraints{Min: image.Pt(1, 2), Max: image.Pt(10, 20)}

	if lo, hi := Horizontal.mainConstraint(cs); lo != 1 || hi != 10 {
		t.Errorf("horizontal main = (%d, %d); want (1, 10)", lo, hi)
	}
	if lo, hi := Horizontal.crossConstraint(cs); lo != 2 || hi != 20 {
		t.Errorf("horizontal cross = (%d, %d); want (2, 20)", lo, hi)
	}
	if lo, hi := Vertical.mainConstraint(cs); lo != 2 || hi != 20 {
		t.Errorf("vertical main = (%d, %d); want (2, 20)", lo, hi)
	}
	if lo, hi := Vertical.crossConstraint(cs); lo != 1 || hi != 10 {
		t.Errorf("vertical cross = (%d, %d); want (1, 10)", lo, hi)
	}
}

// TestAxisConstraintsRoundTrip closes the loop: constraints taken apart along
// an axis and put back together along the same axis are the ones that went in.
func TestAxisConstraintsRoundTrip(t *testing.T) {
	cs := Constraints{Min: image.Pt(1, 2), Max: image.Pt(10, 20)}
	for _, a := range []Axis{Horizontal, Vertical} {
		mainMin, mainMax := a.mainConstraint(cs)
		crossMin, crossMax := a.crossConstraint(cs)
		if got := a.constraints(mainMin, mainMax, crossMin, crossMax); got != cs {
			t.Errorf("%v: round trip gave %+v; want %+v", a, got, cs)
		}
	}
}

// TestFPtKeepsTheCoordinates guards the conversion the drawing side reads
// positions through.
func TestFPtKeepsTheCoordinates(t *testing.T) {
	if got := FPt(image.Pt(3, -4)); got != (f32.Point{X: 3, Y: -4}) {
		t.Errorf("FPt = %v", got)
	}
}

// TestNamesAreTheOnesWritten fixes the strings, which are what a failing test
// elsewhere in the product prints when it names a direction or an axis.
func TestNamesAreTheOnesWritten(t *testing.T) {
	directions := map[Direction]string{
		NW: "NW", N: "N", NE: "NE", E: "E", SE: "SE", S: "S", SW: "SW", W: "W", Center: "Center",
	}
	for dir, want := range directions {
		if got := dir.String(); got != want {
			t.Errorf("Direction(%d).String() = %q; want %q", dir, got, want)
		}
	}
	alignments := map[Alignment]string{Start: "Start", End: "End", Middle: "Middle", Baseline: "Baseline"}
	for a, want := range alignments {
		if got := a.String(); got != want {
			t.Errorf("Alignment(%d).String() = %q; want %q", a, got, want)
		}
	}
	axes := map[Axis]string{Horizontal: "Horizontal", Vertical: "Vertical"}
	for a, want := range axes {
		if got := a.String(); got != want {
			t.Errorf("Axis(%d).String() = %q; want %q", a, got, want)
		}
	}
}

// TestNamesRefuseAValueThatIsNotOne says what happens to a value outside the
// set: it stops, rather than printing a number that reads like a name.
//
// These are closed sets with no zero-value escape hatch, so a value outside
// them came from arithmetic on a constant or from a cast, and both are bugs at
// the place that made them rather than at the place that printed them.
func TestNamesRefuseAValueThatIsNotOne(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func()
	}{
		{"direction", func() { _ = Direction(100).String() }},
		{"alignment", func() { _ = Alignment(100).String() }},
		{"axis", func() { _ = Axis(100).String() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("naming a value outside the set returned instead of stopping")
				}
			}()
			tc.call()
		})
	}
}
