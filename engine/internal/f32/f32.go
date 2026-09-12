// Package f32 is the drawing path's half of the public f32 package.
//
// It re-exports the public point and the affine transform under this name, and
// adds the float32 rectangle that only the inside of the engine works in.
//
// The rectangle is here rather than beside the point because it is not part of
// what an application writes. A screen is handed room in whole pixels and
// answers in whole pixels; between those two the engine clips, strokes and
// transforms, and there the corners stop landing on pixel boundaries. FPt and
// FRect are the way in, Round is the way out, and everything fractional stays
// between them -- which is what keeps a coordinate with a fraction in it out of
// the API a screen is written against.
package f32

import (
	"image"
	"math"

	"github.com/arandu-io/ayra/engine/f32"
)

// Point is the public Point under this package's name.
//
// It is an alias and not a defined type of its own, so a point crosses between
// the two packages without a conversion. A defined type would make them
// incompatible, and every value passed from here into the public package would
// have to be converted -- once per point, per frame, to say nothing.
//
// Every literal of it in this file names its fields, and that is the rule here
// rather than a preference. The alias resolves to a struct declared in another
// package, so the positional form commits this file to a field order it does
// not own: a field added on that side, or X and Y exchanged, leaves this side
// compiling and meaning something else. Both fields are float32, so neither the
// compiler nor a test of the result would catch the exchange -- the coordinates
// would simply be the wrong way round, and a picture drawn from them is
// mirrored about the diagonal. Naming the fields is what makes the literal say
// which coordinate is which, and it is also the form go vet accepts for a
// struct that comes from another package.
type Point = f32.Point

// Pt is shorthand for Point{X: x, Y: y}.
var Pt = f32.Pt

// Affine2D is the public affine transform under this package's name, aliased
// for the same reason as Point: a transform is applied to every point of every
// path in a frame, and a conversion on that path would be paid per point.
type Affine2D = f32.Affine2D

// NewAffine2D builds a transform from the matrix elements in row major order.
// The rows are [sx, hx, ox], [hy, sy, oy], [0, 0, 1].
var NewAffine2D = f32.NewAffine2D

// AffineId returns the transform that moves nothing.
var AffineId = f32.AffineId

// A Rectangle contains the points (X, Y) where Min.X <= X < Max.X,
// Min.Y <= Y < Max.Y.
//
// Half open on the Max side, like the integer rectangle it is converted to and
// from: two rectangles that share an edge then cover that edge once between
// them, rather than both claiming it and drawing the seam twice.
//
// The corners are not kept ordered. A rectangle whose Max is above or left of
// its Min is inside out, Empty reports it as empty, and Canon is what orders
// one. Nothing in this file orders corners behind the caller's back except
// Rect and Canon, which say so.
type Rectangle struct {
	Min, Max Point
}

// Rect is a shorthand for Rectangle{Point{x0, y0}, Point{x1, y1}} with the
// corners ordered: x0 and x1 are exchanged if necessary, and so are y0 and y1,
// so that the result is never inside out.
//
// The two axes are ordered independently. Exchanging the pair as a whole would
// turn a rectangle that is backwards on one axis only into one that is
// backwards on the other.
func Rect(x0, y0, x1, y1 float32) Rectangle {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	return Rectangle{Point{X: x0, Y: y0}, Point{X: x1, Y: y1}}
}

// String returns a string representation of r.
func (r Rectangle) String() string {
	return r.Min.String() + "-" + r.Max.String()
}

// Size returns r's width and height, as the two components of a point.
//
// It is a Point and not a pair because that is what it is added to and
// subtracted from: an offset and a size are the same two numbers here, and a
// separate type for each would be a conversion at every corner.
func (r Rectangle) Size() Point {
	return Point{X: r.Dx(), Y: r.Dy()}
}

// Dx returns r's width, which is negative if r is inside out.
func (r Rectangle) Dx() float32 {
	return r.Max.X - r.Min.X
}

// Dy returns r's height, which is negative if r is inside out.
func (r Rectangle) Dy() float32 {
	return r.Max.Y - r.Min.Y
}

// Intersect returns the intersection of r and s, and the zero Rectangle if
// they do not overlap.
//
// The zero value rather than whatever the clamping left behind, because a
// caller that goes on to read Min from an empty result would otherwise get a
// corner that looks like a position on screen and is not one.
func (r Rectangle) Intersect(s Rectangle) Rectangle {
	if r.Min.X < s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y < s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X > s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y > s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	if r.Empty() {
		return Rectangle{}
	}
	return r
}

// Union returns the smallest rectangle containing both r and s.
//
// An empty operand is dropped rather than stretched over. A rectangle with no
// area still has a position, and letting the zero one take part would pull
// every union it appears in back to the origin -- which is how a bounding box
// grows to the top left corner of the window and clips nothing.
func (r Rectangle) Union(s Rectangle) Rectangle {
	if r.Empty() {
		return s
	}
	if s.Empty() {
		return r
	}
	if r.Min.X > s.Min.X {
		r.Min.X = s.Min.X
	}
	if r.Min.Y > s.Min.Y {
		r.Min.Y = s.Min.Y
	}
	if r.Max.X < s.Max.X {
		r.Max.X = s.Max.X
	}
	if r.Max.Y < s.Max.Y {
		r.Max.Y = s.Max.Y
	}
	return r
}

// Canon returns the canonical version of r, where Min is to the upper left of
// Max. It is what turns a rectangle built from two arbitrary corners -- a drag,
// a transform that flipped an axis -- into one the rest of this file accepts.
func (r Rectangle) Canon() Rectangle {
	if r.Max.X < r.Min.X {
		r.Min.X, r.Max.X = r.Max.X, r.Min.X
	}
	if r.Max.Y < r.Min.Y {
		r.Min.Y, r.Max.Y = r.Max.Y, r.Min.Y
	}
	return r
}

// Empty reports whether r represents the empty area.
//
// An inside out rectangle is empty too, and deliberately: it encloses no point
// that satisfies Min <= X < Max, so treating it as covering the area between
// its corners would be treating a negative width as a positive one.
func (r Rectangle) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

// Add offsets r with the vector p.
func (r Rectangle) Add(p Point) Rectangle {
	return Rectangle{
		Point{X: r.Min.X + p.X, Y: r.Min.Y + p.Y},
		Point{X: r.Max.X + p.X, Y: r.Max.Y + p.Y},
	}
}

// Sub offsets r with the vector -p.
//
// It is the exact inverse of Add for the same vector: offsetting is what a
// layout does on the way down and what input undoes on the way back up, and a
// pair that did not cancel would land a press next to what it was aimed at.
func (r Rectangle) Sub(p Point) Rectangle {
	return Rectangle{
		Point{X: r.Min.X - p.X, Y: r.Min.Y - p.Y},
		Point{X: r.Max.X - p.X, Y: r.Max.Y - p.Y},
	}
}

// Round returns the smallest integer rectangle that contains r.
//
// Outwards on all four sides, not to the nearest. The result is what something
// is drawn or clipped into, and one rounded to the nearest would be up to half
// a pixel short on each edge -- which is a clipped edge, not a rounding error.
func (r Rectangle) Round() image.Rectangle {
	return image.Rectangle{
		Min: image.Point{
			X: floor(r.Min.X),
			Y: floor(r.Min.Y),
		},
		Max: image.Point{
			X: ceil(r.Max.X),
			Y: ceil(r.Max.Y),
		},
	}
}

// FRect converts an integer rectangle into the fractional one this package
// works in. It is the way into that space, and Round is the way out.
//
// The corners are carried across as they are. Unlike Rect it does not order
// them, because an integer rectangle that is inside out is inside out for a
// reason the caller knows and this conversion does not.
func FRect(r image.Rectangle) Rectangle {
	return Rectangle{
		Min: FPt(r.Min), Max: FPt(r.Max),
	}
}

// FPt converts an integer point into a fractional one.
func FPt(p image.Point) Point {
	return Point{
		X: float32(p.X), Y: float32(p.Y),
	}
}

// ceil is the smallest integer at or above v.
func ceil(v float32) int {
	return int(math.Ceil(float64(v)))
}

// floor is the largest integer at or below v.
func floor(v float32) int {
	return int(math.Floor(float64(v)))
}
