package clip_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
)

// TestAnEllipseFillsItsMiddleAndNotItsCorners is the cheapest statement that
// a round shape came out round.
//
// The outline is four curves, and a mistake in the arithmetic that places
// their control points produces a shape that is still closed, still centred
// and still the right size -- a lens, a lozenge, a square with soft edges.
// Each of those fills the middle. What tells them apart is the corner of the
// bounding box, which belongs to the box and never to the ellipse.
func TestAnEllipseFillsItsMiddleAndNotItsCorners(t *testing.T) {
	bounds := image.Rect(20, 20, 80, 80)

	pixels := picture(t, 100, func(o *op.Ops) {
		paint.FillShape(o, ink, clip.Ellipse(bounds).Op(o))
	})

	for _, at := range []struct {
		point image.Point
		want  bool
		what  string
	}{
		{point: image.Pt(50, 50), want: true, what: "the centre"},
		{point: image.Pt(50, 22), want: true, what: "the top of the outline"},
		{point: image.Pt(22, 50), want: true, what: "the left of the outline"},
		{point: image.Pt(25, 25), want: false, what: "the corner of the bounding box"},
		{point: image.Pt(75, 75), want: false, what: "the opposite corner"},
		{point: image.Pt(5, 5), want: false, what: "outside the bounding box"},
	} {
		if got := painted(pixels, at.point.X, at.point.Y); got != at.want {
			t.Errorf("%s at %v is painted=%v, want %v", at.what, at.point, got, at.want)
		}
	}

	if reached := inked(pixels); !reached.In(bounds) {
		t.Errorf("the ellipse reached %v, which is outside its bounds %v", reached, bounds)
	}
}

// TestAZeroSizedEllipseDrawsNothing fixes a size no ellipse can be made from.
//
// A point has no width to stretch the outline across, and the ratio that
// would do the stretching is a division by nothing. It reaches the shape from
// ordinary layout -- a control measured before it has been given any room --
// so the frame it happens on has to draw nothing rather than refuse.
func TestAZeroSizedEllipseDrawsNothing(t *testing.T) {
	point := image.Pt(1, 2)

	pixels := picture(t, 100, func(o *op.Ops) {
		paint.FillShape(o, ink, clip.Ellipse{Min: point, Max: point}.Op(o))
	})

	if reached := inked(pixels); !reached.Empty() {
		t.Errorf("an ellipse with no size reached %v, want nothing", reached)
	}
}

// TestARoundedRectangleCutsItsCorners fixes that the radius does something.
//
// A rounded rectangle whose corners came out square is the failure that looks
// like a theme setting rather than like a drawing fault, and it is what a
// caller sees whenever the radius is lost between the shape and the outline.
// The middle of each side has to stay, or the corners were not cut but the
// whole shape shrunk.
func TestARoundedRectangleCutsItsCorners(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)

	pixels := picture(t, 100, func(o *op.Ops) {
		paint.FillShape(o, ink, clip.UniformRRect(bounds, 30).Op(o))
	})

	for _, at := range []struct {
		point image.Point
		want  bool
		what  string
	}{
		{point: image.Pt(50, 50), want: true, what: "the centre"},
		{point: image.Pt(50, 1), want: true, what: "the middle of the top edge"},
		{point: image.Pt(1, 50), want: true, what: "the middle of the left edge"},
		{point: image.Pt(2, 2), want: false, what: "the north west corner"},
		{point: image.Pt(97, 2), want: false, what: "the north east corner"},
		{point: image.Pt(2, 97), want: false, what: "the south west corner"},
		{point: image.Pt(97, 97), want: false, what: "the south east corner"},
	} {
		if got := painted(pixels, at.point.X, at.point.Y); got != at.want {
			t.Errorf("%s at %v is painted=%v, want %v", at.what, at.point, got, at.want)
		}
	}
}

// TestARectangleWithNoRadiusKeepsItsCorners is the other half of the same
// statement.
//
// Zero is the ordinary case -- most surfaces in a theme have no radius at all
// -- and a rounded rectangle that rounds a little anyway would soften every
// edge in the product by an amount nobody asked for and nobody can name.
func TestARectangleWithNoRadiusKeepsItsCorners(t *testing.T) {
	bounds := image.Rect(10, 10, 90, 90)

	pixels := picture(t, 100, func(o *op.Ops) {
		paint.FillShape(o, ink, clip.UniformRRect(bounds, 0).Op(o))
	})

	if reached := inked(pixels); reached != bounds {
		t.Fatalf("the ink reached %v, want the whole rectangle %v", reached, bounds)
	}
	for _, corner := range []image.Point{
		image.Pt(10, 10), image.Pt(89, 10), image.Pt(10, 89), image.Pt(89, 89),
	} {
		if !painted(pixels, corner.X, corner.Y) {
			t.Errorf("the corner at %v was cut, want it kept", corner)
		}
	}
}

// TestAStrokeDrawsTheEdgeAndLeavesTheMiddle is what separates a border from a
// fill.
//
// A stroke asks for the path itself, widened; an outline asks for everything
// inside it. They are one field apart, and the frame where the wrong one is
// chosen is a frame where a control's border is a solid block over its own
// label -- which reads as a painting order problem and is not one.
func TestAStrokeDrawsTheEdgeAndLeavesTheMiddle(t *testing.T) {
	bounds := image.Rect(20, 20, 80, 80)

	pixels := picture(t, 100, func(o *op.Ops) {
		edge := clip.Stroke{Path: clip.Rect(bounds).Path(), Width: 4}
		paint.FillShape(o, ink, edge.Op())
	})

	for _, at := range []struct {
		point image.Point
		want  bool
		what  string
	}{
		{point: image.Pt(50, 20), want: true, what: "the top edge"},
		{point: image.Pt(20, 50), want: true, what: "the left edge"},
		{point: image.Pt(79, 50), want: true, what: "the right edge"},
		{point: image.Pt(50, 79), want: true, what: "the bottom edge"},
		{point: image.Pt(50, 50), want: false, what: "the middle"},
		{point: image.Pt(50, 30), want: false, what: "just inside the top edge"},
		{point: image.Pt(50, 10), want: false, what: "well outside the top edge"},
	} {
		if got := painted(pixels, at.point.X, at.point.Y); got != at.want {
			t.Errorf("%s at %v is painted=%v, want %v", at.what, at.point, got, at.want)
		}
	}
}

// TestAStrokeIsCentredOnItsPath fixes which side of the line the width goes.
//
// Half the width falls outside the path and half inside, so the area claimed
// has to grow. Claimed at the path's own bounds, the outer half of every
// border is clipped away -- and since the inner half survives, the result is a
// border that is simply thinner than asked for, at every width, everywhere.
func TestAStrokeIsCentredOnItsPath(t *testing.T) {
	bounds := image.Rect(20, 20, 80, 80)

	pixels := picture(t, 100, func(o *op.Ops) {
		edge := clip.Stroke{Path: clip.Rect(bounds).Path(), Width: 8}
		paint.FillShape(o, ink, edge.Op())
	})

	if !painted(pixels, 50, 17) {
		t.Error("the outer half of the stroke was clipped away above the path")
	}
	if !painted(pixels, 50, 22) {
		t.Error("the inner half of the stroke is missing below the path")
	}
	if painted(pixels, 50, 50) {
		t.Error("the stroke filled the middle, want only its edge")
	}
}
