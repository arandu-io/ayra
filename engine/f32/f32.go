// Package f32 is the coordinate arithmetic everything drawn is measured in:
// two dimensional points and affine transforms, in float32.
//
// The origin is the top left corner, with x extending right and y extending
// down.
//
// Nothing here is rounded on the way through. A position that has been scaled,
// turned or slanted lands between whole pixels, and rounding at each step would
// accumulate down a tree of nested layouts into a drift somebody can see. The
// crossing to the integer grid a display actually addresses happens once, at the
// end, and it is spelt [Point.Round].
package f32

import (
	"image"
	"math"
	"strconv"
)

// A Point is a two dimensional point.
//
// It is not [image.Point]. That one holds two ints, this one holds two float32,
// and the two spellings are close enough that the wrong one reaches for the
// right variable name. Nothing catches it: the conversion between them is
// written by hand either way, and an integer point that took a scaled or
// rotated position simply loses the fraction and draws half a pixel off.
//
// Which of the two a value is decides what may be done with it. Only this one
// can be handed to [Affine2D.Transform], and only this one can hold what a
// transform gives back.
type Point struct {
	X, Y float32
}

// Pt is shorthand for Point{X: x, Y: y}.
func Pt(x, y float32) Point {
	return Point{X: x, Y: y}
}

// Add returns the point p moved by p2.
func (p Point) Add(p2 Point) Point {
	return Point{X: p.X + p2.X, Y: p.Y + p2.Y}
}

// Sub returns the vector from p2 to p.
func (p Point) Sub(p2 Point) Point {
	return Point{X: p.X - p2.X, Y: p.Y - p2.Y}
}

// Mul returns p scaled by s, both coordinates by the same factor.
//
// Scaling the two axes by different amounts is a transform rather than an
// arithmetic step, and it is [Affine2D.Scale].
func (p Point) Mul(s float32) Point {
	return Point{X: p.X * s, Y: p.Y * s}
}

// Div returns p divided by s.
//
// A zero divisor gives the infinities rather than a panic, which is what the
// division itself does. A caller whose scale can reach zero has to decide what
// that means before it gets here: there is no sensible point to answer with, and
// a guard would only be this package choosing one on the caller's behalf.
func (p Point) Div(s float32) Point {
	return Point{X: p.X / s, Y: p.Y / s}
}

// Round returns the integer point closest to p, which is the crossing from this
// package to the grid a display is addressed by.
//
// A half goes away from zero on both sides. Rounding every half up instead would
// shift a shape by a whole pixel as it crossed an axis, so a control centred on
// the origin would be drawn off centre on one side and not the other.
func (p Point) Round() image.Point {
	return image.Point{
		X: int(math.Round(float64(p.X))),
		Y: int(math.Round(float64(p.Y))),
	}
}

// String returns p as "(x,y)".
//
// The coordinates are printed at the width they are stored at, so the text names
// the same number the value holds. Widened to float64 first, a float32 gains the
// digits it never carried -- 0.1 prints as 0.10000000149011612 -- and a
// coordinate copied out of a log no longer matches the one in memory.
func (p Point) String() string {
	return "(" + strconv.FormatFloat(float64(p.X), 'f', -1, 32) +
		"," + strconv.FormatFloat(float64(p.Y), 'f', -1, 32) + ")"
}
