package clip

import (
	"image"
	"math"

	"github.com/arandu-io/ayra/engine/f32"
	f32internal "github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
)

// q is how far a curve's control points reach along the tangent, as a
// fraction of the radius, when the curve is standing in for a quarter circle.
//
// A cubic cannot be an arc, only pass for one, and this is the reach at which
// the two meet at the halfway point -- which is where they are otherwise
// furthest apart, and so the only place worth making them agree. Every other
// value still starts and ends exactly where it should and still looks like a
// corner: what changes is how full it comes out, by an amount that is
// invisible at a radius of four and plain at a radius of sixty.
const q = 4 * (math.Sqrt2 - 1) / 3

// iq is the same reach measured from the other end of the curve.
const iq = 1 - q

// Rect is a pixel aligned rectangle, and the area nearly everything is drawn
// into.
//
// It travels as four numbers rather than as four segments: nothing rasterises
// it, nothing caches it, and its edges fall exactly on the pixel grid. The
// same rectangle sent as a path is the same picture with soft sides and an
// entry in a cache, and a screen is mostly rectangles.
type Rect image.Rectangle

// Op returns the area the rectangle covers.
func (r Rect) Op() Op {
	return Op{
		outline: true,
		path:    r.Path(),
	}
}

// Push narrows the area to the rectangle.
func (r Rect) Push(o *op.Ops) Stack {
	return r.Op().Push(o)
}

// Path returns the rectangle as an outline other operations can take.
//
// It records nothing and needs no list to record into: the four numbers are
// the whole of it. What it is for is [Stroke], which cannot widen a shape and
// has to be handed something it can widen.
func (r Rect) Path() PathSpec {
	return PathSpec{
		shape:  ops.Rect,
		bounds: image.Rectangle(r),
	}
}

// UniformRRect returns an RRect with the same radius on all four corners.
//
// It exists so that the common case is not four fields written out, which is
// where a corner gets left at zero -- and three round corners with one sharp
// one reads as a drawing fault rather than as a typing one.
func UniformRRect(rect image.Rectangle, radius int) RRect {
	return RRect{
		Rect: rect,
		SE:   radius,
		SW:   radius,
		NE:   radius,
		NW:   radius,
	}
}

// RRect is a rectangle with its corners rounded off, which is the shape almost
// every surface in a theme is drawn as.
//
// Each corner is given separately, so that a shape can be joined to the one
// beside it -- the ends of a segmented control, a panel flush against an edge
// -- by rounding only the corners that are actually exposed.
//
// A square whose corner radii are half its side is a circle.
type RRect struct {
	Rect image.Rectangle
	// The corner radii.
	SE, SW, NW, NE int
}

// Op returns the area the rounded rectangle covers.
//
// With no radius at all it is the rectangle itself and is sent as one. Zero is
// the ordinary case in a theme, and sending it as a path would mean four
// segments and a cache entry to draw what four numbers already describe.
func (rr RRect) Op(o *op.Ops) Op {
	if rr.SE == 0 && rr.SW == 0 && rr.NW == 0 && rr.NE == 0 {
		return Rect(rr.Rect).Op()
	}
	return Outline{Path: rr.Path(o)}.Op()
}

// Push narrows the area to the rounded rectangle.
func (rr RRect) Push(o *op.Ops) Stack {
	return rr.Op(o).Push(o)
}

// Path returns the outline of the rounded rectangle.
//
// It is walked clockwise from the top left, alternating a straight side with
// the corner that ends it, and it closes where it began. The order matters to
// nothing but the reading of it; what matters is that it closes, because an
// outline is filled by counting crossings.
func (rr RRect) Path(o *op.Ops) PathSpec {
	var p Path
	p.Begin(o)

	se, sw, nw, ne := float32(rr.SE), float32(rr.SW), float32(rr.NW), float32(rr.NE)
	rrf := f32internal.FRect(rr.Rect)
	w, n, e, s := rrf.Min.X, rrf.Min.Y, rrf.Max.X, rrf.Max.Y

	p.MoveTo(f32.Point{X: w + nw, Y: n})
	p.LineTo(f32.Point{X: e - ne, Y: n}) // N
	p.CubeTo(                            // NE
		f32.Point{X: e - ne*iq, Y: n},
		f32.Point{X: e, Y: n + ne*iq},
		f32.Point{X: e, Y: n + ne})
	p.LineTo(f32.Point{X: e, Y: s - se}) // E
	p.CubeTo(                            // SE
		f32.Point{X: e, Y: s - se*iq},
		f32.Point{X: e - se*iq, Y: s},
		f32.Point{X: e - se, Y: s})
	p.LineTo(f32.Point{X: w + sw, Y: s}) // S
	p.CubeTo(                            // SW
		f32.Point{X: w + sw*iq, Y: s},
		f32.Point{X: w, Y: s - sw*iq},
		f32.Point{X: w, Y: s - sw})
	p.LineTo(f32.Point{X: w, Y: n + nw}) // W
	p.CubeTo(                            // NW
		f32.Point{X: w, Y: n + nw*iq},
		f32.Point{X: w + nw*iq, Y: n},
		f32.Point{X: w + nw, Y: n})

	return p.End()
}

// Ellipse is the largest axis aligned ellipse that fits in a rectangle. A
// square gives a circle, which is what an avatar and a status dot are.
type Ellipse image.Rectangle

// Op returns the area inside the ellipse.
func (e Ellipse) Op(o *op.Ops) Op {
	return Outline{Path: e.Path(o)}.Op()
}

// Push narrows the area to the inside of the ellipse.
func (e Ellipse) Push(o *op.Ops) Stack {
	return e.Op(o).Push(o)
}

// Path returns the outline of the ellipse.
//
// It is four curves, each a quarter, starting at the top and running
// clockwise. A side of nothing draws nothing at all: the outline is a circle
// stretched in one direction, the stretch is the ratio of the two sides, and a
// side of nothing makes that ratio an infinity. A control measured before it
// has been given any room arrives here every time a screen is laid out.
func (e Ellipse) Path(o *op.Ops) PathSpec {
	bounds := image.Rectangle(e)
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return PathSpec{shape: ops.Rect}
	}

	var p Path
	p.Begin(o)

	bf := f32internal.FRect(bounds)
	center := bf.Max.Add(bf.Min).Mul(.5)
	diam := bf.Dx()
	r := diam * .5
	// Modelled as a circle scaled in the Y direction, rather than as an
	// ellipse worked out in both: one radius and one reach, applied twice,
	// instead of two of each that have to agree.
	scale := bf.Dy() / diam

	curve := r * q
	top := f32.Point{X: center.X, Y: center.Y - r*scale}

	p.MoveTo(top)
	p.CubeTo(
		f32.Point{X: center.X + curve, Y: center.Y - r*scale},
		f32.Point{X: center.X + r, Y: center.Y - curve*scale},
		f32.Point{X: center.X + r, Y: center.Y},
	)
	p.CubeTo(
		f32.Point{X: center.X + r, Y: center.Y + curve*scale},
		f32.Point{X: center.X + curve, Y: center.Y + r*scale},
		f32.Point{X: center.X, Y: center.Y + r*scale},
	)
	p.CubeTo(
		f32.Point{X: center.X - curve, Y: center.Y + r*scale},
		f32.Point{X: center.X - r, Y: center.Y + curve*scale},
		f32.Point{X: center.X - r, Y: center.Y},
	)
	p.CubeTo(
		f32.Point{X: center.X - r, Y: center.Y - curve*scale},
		f32.Point{X: center.X - curve, Y: center.Y - r*scale},
		top,
	)

	// Told apart from an arbitrary outline because a renderer can do better
	// with a shape it recognises, and because a pointer test against a circle
	// is arithmetic rather than a crossing count.
	ellipse := p.End()
	ellipse.shape = ops.Ellipse
	return ellipse
}
