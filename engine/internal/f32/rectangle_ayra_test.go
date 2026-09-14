package f32_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/internal/f32"
)

// The file under test carries no tests of its own, and the packages that
// import it exercise none of these methods, so a wrong sign here reaches a
// drawn frame before it reaches a failure. The name of this file keeps it out
// of the way of a test file the original may gain later.
//
// Geometry is the one thing on this path that is cheap to check exhaustively:
// every value below is small, every operation is exact in float32, and a
// property that has to hold for every rectangle is stated as one -- driven by
// the corpus at the bottom rather than by the three numbers whoever wrote the
// test happened to think of.

func TestRectOrdersItsCorners(t *testing.T) {
	want := f32.Rectangle{f32.Point{X: 1, Y: 2}, f32.Point{X: 3, Y: 4}}
	for _, args := range [][4]float32{
		{1, 2, 3, 4},
		{3, 2, 1, 4},
		{1, 4, 3, 2},
		{3, 4, 1, 2},
	} {
		if got := f32.Rect(args[0], args[1], args[2], args[3]); got != want {
			t.Errorf("Rect%v = %v, want %v", args, got, want)
		}
	}
}

func TestRectKeepsEachArgumentOnItsOwnAxis(t *testing.T) {
	// Distinct values on every corner, so swapping X with Y is visible.
	got := f32.Rect(1, 2, 3, 5)
	if got.Min.X != 1 || got.Min.Y != 2 || got.Max.X != 3 || got.Max.Y != 5 {
		t.Errorf("Rect(1, 2, 3, 5) = %v, want (1,2)-(3,5)", got)
	}
}

func TestAddAndSubOffsetBothCorners(t *testing.T) {
	r := f32.Rect(1, 2, 3, 5)
	p := f32.Point{X: 10, Y: 20}

	add := r.Add(p)
	if want := f32.Rect(11, 22, 13, 25); add != want {
		t.Errorf("Add(%v) = %v, want %v", p, add, want)
	}

	sub := r.Sub(p)
	if want := f32.Rect(-9, -18, -7, -15); sub != want {
		t.Errorf("Sub(%v) = %v, want %v", p, sub, want)
	}

	if back := add.Sub(p); back != r {
		t.Errorf("Add then Sub = %v, want %v", back, r)
	}
}

func TestSizeReportsWidthThenHeight(t *testing.T) {
	got := f32.Rect(1, 2, 4, 12).Size()
	if want := (f32.Point{X: 3, Y: 10}); got != want {
		t.Errorf("Size() = %v, want %v", got, want)
	}
}

func TestDxAndDyAreTheTwoHalvesOfSize(t *testing.T) {
	r := f32.Rect(1, 2, 4, 12)
	if got, want := r.Dx(), float32(3); got != want {
		t.Errorf("Dx() = %v, want %v", got, want)
	}
	if got, want := r.Dy(), float32(10); got != want {
		t.Errorf("Dy() = %v, want %v", got, want)
	}

	// Unordered corners are not an error here: Dx and Dy subtract, and the
	// negative is what tells a caller the rectangle was never canonicalised.
	backwards := f32.Rectangle{f32.Point{X: 4, Y: 12}, f32.Point{X: 1, Y: 2}}
	if got, want := backwards.Dx(), float32(-3); got != want {
		t.Errorf("Dx() of unordered corners = %v, want %v", got, want)
	}
	if got, want := backwards.Dy(), float32(-10); got != want {
		t.Errorf("Dy() of unordered corners = %v, want %v", got, want)
	}
}

func TestStringNamesBothCorners(t *testing.T) {
	if got, want := f32.Rect(1, 2, 3.5, 4).String(), "(1,2)-(3.5,4)"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestEmptyIsTrueWhenAnEdgeHasNoLength(t *testing.T) {
	for _, c := range []struct {
		name string
		r    f32.Rectangle
		want bool
	}{
		{"area", f32.Rect(0, 0, 1, 1), false},
		{"zero value", f32.Rectangle{}, true},
		{"no width", f32.Rect(1, 0, 1, 1), true},
		{"no height", f32.Rect(0, 1, 1, 1), true},
		{"corners the wrong way round", f32.Rectangle{f32.Point{X: 1, Y: 1}, f32.Point{X: 0, Y: 0}}, true},
	} {
		if got := c.r.Empty(); got != c.want {
			t.Errorf("%s: %v.Empty() = %v, want %v", c.name, c.r, got, c.want)
		}
	}
}

func TestIntersectKeepsOnlyTheOverlap(t *testing.T) {
	r, s := f32.Rect(0, 0, 10, 10), f32.Rect(5, 4, 20, 20)
	if got, want := r.Intersect(s), f32.Rect(5, 4, 10, 10); got != want {
		t.Errorf("Intersect = %v, want %v", got, want)
	}
	if got, want := s.Intersect(r), f32.Rect(5, 4, 10, 10); got != want {
		t.Errorf("Intersect the other way round = %v, want %v", got, want)
	}
}

func TestIntersectWithoutOverlapIsTheZeroRectangle(t *testing.T) {
	// Not merely empty: the zero value, so that a caller which goes on to read
	// Min gets the origin rather than whichever corner survived the clamping.
	for _, c := range []struct {
		name string
		r, s f32.Rectangle
	}{
		{"disjoint", f32.Rect(0, 0, 1, 1), f32.Rect(2, 2, 3, 3)},
		{"touching along an edge", f32.Rect(0, 0, 1, 1), f32.Rect(1, 0, 2, 1)},
		{"one of them empty", f32.Rect(0, 0, 1, 1), f32.Rectangle{}},
	} {
		if got := c.r.Intersect(c.s); got != (f32.Rectangle{}) {
			t.Errorf("%s: Intersect = %v, want the zero rectangle", c.name, got)
		}
	}
}

func TestUnionCoversBothRectangles(t *testing.T) {
	r, s := f32.Rect(0, 0, 1, 1), f32.Rect(2, 3, 4, 5)
	if got, want := r.Union(s), f32.Rect(0, 0, 4, 5); got != want {
		t.Errorf("Union = %v, want %v", got, want)
	}
	if got, want := s.Union(r), f32.Rect(0, 0, 4, 5); got != want {
		t.Errorf("Union the other way round = %v, want %v", got, want)
	}
}

func TestUnionWithAnEmptyRectangleIsTheOtherOne(t *testing.T) {
	r := f32.Rect(1, 2, 3, 4)
	if got := r.Union(f32.Rectangle{}); got != r {
		t.Errorf("Union with an empty rectangle = %v, want %v", got, r)
	}
	if got := (f32.Rectangle{}).Union(r); got != r {
		t.Errorf("empty rectangle Union = %v, want %v", got, r)
	}
	if got := (f32.Rectangle{}).Union(f32.Rectangle{}); got != (f32.Rectangle{}) {
		t.Errorf("two empty rectangles = %v, want the zero rectangle", got)
	}
}

func TestCanonOrdersCorners(t *testing.T) {
	want := f32.Rect(1, 2, 3, 4)
	for _, r := range []f32.Rectangle{
		{f32.Point{X: 1, Y: 2}, f32.Point{X: 3, Y: 4}},
		{f32.Point{X: 3, Y: 2}, f32.Point{X: 1, Y: 4}},
		{f32.Point{X: 1, Y: 4}, f32.Point{X: 3, Y: 2}},
		{f32.Point{X: 3, Y: 4}, f32.Point{X: 1, Y: 2}},
	} {
		if got := r.Canon(); got != want {
			t.Errorf("%v.Canon() = %v, want %v", r, got, want)
		}
	}
}

func TestRoundGrowsToTheEnclosingIntegerRectangle(t *testing.T) {
	// Out on every side rather than to the nearest: a rectangle rounded inwards
	// clips the drawing it was measured for, by up to a pixel on each edge.
	for _, c := range []struct {
		r    f32.Rectangle
		want image.Rectangle
	}{
		{f32.Rect(1.2, 2.7, 3.1, 4), image.Rect(1, 2, 4, 4)},
		{f32.Rect(-1.2, -2.7, -0.5, -0.1), image.Rect(-2, -3, 0, 0)},
		{f32.Rect(2, 3, 5, 7), image.Rect(2, 3, 5, 7)},
	} {
		if got := c.r.Round(); got != c.want {
			t.Errorf("%v.Round() = %v, want %v", c.r, got, c.want)
		}
	}
}

func TestFPtAndFRectCarryIntegerCoordinatesAcross(t *testing.T) {
	if got, want := f32.FPt(image.Pt(3, -4)), (f32.Point{X: 3, Y: -4}); got != want {
		t.Errorf("FPt = %v, want %v", got, want)
	}
	if got, want := f32.FRect(image.Rect(1, 2, 3, 4)), f32.Rect(1, 2, 3, 4); got != want {
		t.Errorf("FRect = %v, want %v", got, want)
	}

	// FRect does not canonicalise, which is the difference between it and Rect
	// and the reason both exist.
	unordered := image.Rectangle{Min: image.Pt(3, 4), Max: image.Pt(1, 2)}
	if got, want := f32.FRect(unordered), (f32.Rectangle{f32.Point{X: 3, Y: 4}, f32.Point{X: 1, Y: 2}}); got != want {
		t.Errorf("FRect of unordered corners = %v, want %v", got, want)
	}
}

func TestPtIsTheShorthandForAPoint(t *testing.T) {
	if got, want := f32.Pt(1.5, -2.5), (f32.Point{X: 1.5, Y: -2.5}); got != want {
		t.Errorf("Pt = %v, want %v", got, want)
	}
}

// corpus is the set of coordinates the properties below are stated over.
//
// Small, exact in float32, and mixed in sign, so that a property which holds
// only for the top-left quadrant fails here rather than on a screen where
// something scrolled past the origin.
var corpus = []float32{-3.5, -1, 0, 0.25, 2, 7.75}

// forEachRectangle runs fn over every rectangle the corpus can name, corners in
// the order given rather than canonicalised.
func forEachRectangle(fn func(x0, y0, x1, y1 float32)) {
	for _, x0 := range corpus {
		for _, y0 := range corpus {
			for _, x1 := range corpus {
				for _, y1 := range corpus {
					fn(x0, y0, x1, y1)
				}
			}
		}
	}
}

func TestAddThenSubOfTheSameVectorIsTheOriginalRectangle(t *testing.T) {
	// Offsetting is what a layout does to every rectangle it passes down and
	// what input does in reverse to every position it passes back up. The two
	// have to cancel exactly, or a pointer lands next to what it was over.
	offset := f32.Point{X: 2.5, Y: -1.25}
	forEachRectangle(func(x0, y0, x1, y1 float32) {
		r := f32.Rect(x0, y0, x1, y1)
		if got := r.Add(offset).Sub(offset); got != r {
			t.Fatalf("%v.Add(%v).Sub(%v) = %v, want %v", r, offset, offset, got, r)
		}
		if got := r.Sub(offset).Add(offset); got != r {
			t.Fatalf("%v.Sub(%v).Add(%v) = %v, want %v", r, offset, offset, got, r)
		}
	})
}

func TestOrderedCornersNeverGiveANegativeSize(t *testing.T) {
	// A negative width is not a small rectangle, it is a rectangle inside out,
	// and the two ways of ordering corners have to agree that it cannot happen.
	forEachRectangle(func(x0, y0, x1, y1 float32) {
		for _, r := range []f32.Rectangle{
			f32.Rect(x0, y0, x1, y1),
			f32.Rectangle{f32.Point{X: x0, Y: y0}, f32.Point{X: x1, Y: y1}}.Canon(),
		} {
			if size := r.Size(); size.X < 0 || size.Y < 0 {
				t.Fatalf("%v.Size() = %v, want both sides at or above zero", r, size)
			}
		}
	})
}

func TestRectAndCanonOrderCornersTheSameWay(t *testing.T) {
	forEachRectangle(func(x0, y0, x1, y1 float32) {
		shorthand := f32.Rect(x0, y0, x1, y1)
		longhand := f32.Rectangle{f32.Point{X: x0, Y: y0}, f32.Point{X: x1, Y: y1}}.Canon()
		if shorthand != longhand {
			t.Fatalf("Rect(%v, %v, %v, %v) = %v, Canon of the same corners = %v",
				x0, y0, x1, y1, shorthand, longhand)
		}
	})
}

func TestIntersectionIsContainedInBothOperands(t *testing.T) {
	sample := f32.Rect(-1, -1, 3, 2)
	forEachRectangle(func(x0, y0, x1, y1 float32) {
		r := f32.Rect(x0, y0, x1, y1)
		got := r.Intersect(sample)
		if got.Empty() {
			return
		}
		if got.Union(r) != r.Canon() || got.Union(sample) != sample {
			t.Fatalf("%v.Intersect(%v) = %v, which is not inside both", r, sample, got)
		}
	})
}

func TestUnionContainsBothOperands(t *testing.T) {
	sample := f32.Rect(-1, -1, 3, 2)
	forEachRectangle(func(x0, y0, x1, y1 float32) {
		r := f32.Rect(x0, y0, x1, y1)
		got := r.Union(sample)
		if !r.Empty() && got.Union(r) != got {
			t.Fatalf("%v.Union(%v) = %v, which does not contain the first", r, sample, got)
		}
		if got.Union(sample) != got {
			t.Fatalf("%v.Union(%v) = %v, which does not contain the second", r, sample, got)
		}
	})
}

func TestRoundEnclosesTheRectangleItWasGiven(t *testing.T) {
	forEachRectangle(func(x0, y0, x1, y1 float32) {
		r := f32.Rect(x0, y0, x1, y1)
		rounded := f32.FRect(r.Round())
		if rounded.Min.X > r.Min.X || rounded.Min.Y > r.Min.Y {
			t.Fatalf("%v.Round() = %v, which starts inside it", r, rounded)
		}
		if rounded.Max.X < r.Max.X || rounded.Max.Y < r.Max.Y {
			t.Fatalf("%v.Round() = %v, which ends inside it", r, rounded)
		}
	})
}

func TestIntegerRectanglesSurviveTheRoundTrip(t *testing.T) {
	// FRect widens and Round narrows, and every coordinate the layout hands
	// this package started as an integer number of pixels.
	for _, r := range []image.Rectangle{
		image.Rect(0, 0, 0, 0),
		image.Rect(1, 2, 3, 4),
		image.Rect(-7, -9, -1, -2),
		image.Rect(-4, 3, 5, 11),
	} {
		if got := f32.FRect(r).Round(); got != r {
			t.Errorf("FRect(%v).Round() = %v, want %v", r, got, r)
		}
	}
}

func TestTheIdentityTransformLeavesAPointWhereItIs(t *testing.T) {
	// Three spellings of the same transform, because all three are reachable
	// from here: the zero value, the constructor, and the named identity. A
	// transform that moved a point by a fraction would show up as text drawn
	// half a pixel off its baseline and nowhere else.
	transforms := []struct {
		name string
		a    f32.Affine2D
	}{
		{"zero value", f32.Affine2D{}},
		{"AffineId", f32.AffineId()},
		{"NewAffine2D", f32.NewAffine2D(1, 0, 0, 0, 1, 0)},
	}
	for _, tr := range transforms {
		for _, x := range corpus {
			for _, y := range corpus {
				p := f32.Pt(x, y)
				if got := tr.a.Transform(p); got != p {
					t.Fatalf("%s: Transform(%v) = %v, want the point unmoved", tr.name, p, got)
				}
			}
		}
	}
}

func TestNewAffine2DKeepsTheElementsItWasGiven(t *testing.T) {
	a := f32.NewAffine2D(2, 0.5, 3, -0.25, 4, -5)
	sx, hx, ox, hy, sy, oy := a.Elems()
	want := [6]float32{2, 0.5, 3, -0.25, 4, -5}
	if got := [6]float32{sx, hx, ox, hy, sy, oy}; got != want {
		t.Errorf("Elems() = %v, want %v", got, want)
	}
}
