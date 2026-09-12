package f32

import (
	"math"
	"strconv"
)

// Affine2D is an affine 2D transformation: everything a layout does to a
// coordinate on the way down a tree -- move it, scale it, turn it, slant it --
// held as one value that can be applied in one step.
//
// The transformation is the matrix
//
//	[sx hx ox]
//	[hy sy oy]
//	[ 0  0  1]
//
// and the zero value of Affine2D is the identity, which is worth what it costs
// to arrange: a transformation is a field of many of the types that draw, and
// one that had to be constructed before it was any use would collapse a whole
// subtree onto the origin the first time somebody left it out.
//
// What it costs is that the two diagonal terms are held with one subtracted, so
// that six zeroed fields read back as the matrix which changes nothing, and
// every derivation has to add it back before it can use them. The two fields say
// so in their names: nothing here can read sx out of a field that holds sx-1
// without spelling out which of the two it is asking for.
// [Affine2D.Elems] is that reading undone once and given a name.
//
// The bottom row is not stored. It is [0 0 1] for every transformation this type
// can hold -- that is what makes it affine rather than projective -- so keeping
// it would be two zeros and a one multiplied out several million times a second
// to be told what they already say.
type Affine2D struct {
	sxMinus1, hx, ox float32
	hy, syMinus1, oy float32
}

// NewAffine2D creates a new Affine2D transform from the matrix elements
// in row major order. The rows are: [sx, hx, ox], [hy, sy, oy], [0, 0, 1].
func NewAffine2D(sx, hx, ox, hy, sy, oy float32) Affine2D {
	return Affine2D{
		sxMinus1: sx - 1, hx: hx, ox: ox,
		hy: hy, syMinus1: sy - 1, oy: oy,
	}
}

// AffineId returns an identity transformation matrix that represents no
// transformation when applied.
func AffineId() Affine2D {
	return NewAffine2D(
		1, 0, 0,
		0, 1, 0,
	)
}

// Elems returns the matrix elements of the transform in row-major order. The
// rows are: [sx, hx, ox], [hy, sy, oy], [0, 0, 1].
//
// The six are the transformation's own, so the identity answers 1, 0, 0, 0, 1,
// 0 whether it was built or merely declared.
func (a Affine2D) Elems() (sx, hx, ox, hy, sy, oy float32) {
	return a.sxMinus1 + 1, a.hx, a.ox, a.hy, a.syMinus1 + 1, a.oy
}

// Offset returns the transformation moved by the given amount.
//
// It is the one of the four with no origin to be given, because moving is the
// same measured from anywhere: shifting the frame first and back afterwards
// would add and subtract the same number.
func (a Affine2D) Offset(offset Point) Affine2D {
	return Affine2D{
		a.sxMinus1, a.hx, a.ox + offset.X,
		a.hy, a.syMinus1, a.oy + offset.Y,
	}
}

// Scale returns the transformation scaled by the given factor around the given
// origin.
//
// Scale, rotate and shear all act about zero -- a matrix has no field naming a
// centre -- so reaching an origin means moving that point to zero, acting there,
// and moving it back. Between the two moves the origin is the one point that
// does not move, which is what "around the given origin" promises and what the
// three are tested on.
//
// An origin of zero takes neither move, rather than offsetting by nothing and
// back. The two moves would cancel, but they are not free: measured, they cost
// about half as much again as the scaling itself. A zero origin is what a layout
// passes whenever it scales or turns about the corner it was already handed,
// which is most of the time.
func (a Affine2D) Scale(origin, factor Point) Affine2D {
	if origin == (Point{}) {
		return a.scaleAboutZero(factor)
	}
	return a.Offset(origin.Mul(-1)).scaleAboutZero(factor).Offset(origin)
}

// Rotate returns the transformation turned by the given angle, in radians,
// counter clockwise around the given origin, which is reached the way
// [Affine2D.Scale] reaches it.
func (a Affine2D) Rotate(origin Point, radians float32) Affine2D {
	if origin == (Point{}) {
		return a.rotateAboutZero(radians)
	}
	return a.Offset(origin.Mul(-1)).rotateAboutZero(radians).Offset(origin)
}

// Shear returns the transformation slanted by the given angles, in radians,
// around the given origin, which is reached the way [Affine2D.Scale] reaches it.
func (a Affine2D) Shear(origin Point, radiansX, radiansY float32) Affine2D {
	if origin == (Point{}) {
		return a.shearAboutZero(radiansX, radiansY)
	}
	return a.Offset(origin.Mul(-1)).shearAboutZero(radiansX, radiansY).Offset(origin)
}

// Transform returns the point p with this transformation applied.
func (a Affine2D) Transform(p Point) Point {
	sx, hx, ox, hy, sy, oy := a.Elems()
	return Point{
		X: p.X*sx + p.Y*hx + ox,
		Y: p.X*hy + p.Y*sy + oy,
	}
}

// Mul returns the transformation that applies b and then a.
//
// The order is the one the reading gives: a.Mul(b) is a*b, and a point meets b
// first. It is also the order the methods above build in -- each of them
// pre-multiplies, so a chain of calls composes right to left, and the last one
// written is the outermost.
func (a Affine2D) Mul(b Affine2D) (r Affine2D) {
	asx, asy := a.sxMinus1+1, a.syMinus1+1
	bsx, bsy := b.sxMinus1+1, b.syMinus1+1
	r.sxMinus1 = asx*bsx + a.hx*b.hy - 1
	r.hx = asx*b.hx + a.hx*bsy
	r.ox = asx*b.ox + a.hx*b.oy + a.ox
	r.hy = a.hy*bsx + asy*b.hy
	r.syMinus1 = a.hy*b.hx + asy*bsy - 1
	r.oy = a.hy*b.ox + asy*b.oy + a.oy
	return r
}

// Invert returns the transformation that undoes this one.
//
// A matrix close to singular -- one that flattens the plane onto a line, which
// is what a scale by zero on one axis does -- has a determinant close to zero,
// and dividing by it makes the error large or infinite. Nothing here rejects
// that: the caller that produced the degenerate scale is the only one that knows
// what it meant by it.
func (a Affine2D) Invert() Affine2D {
	if a.isOffsetOnly() {
		return Affine2D{0, 0, -a.ox, 0, 0, -a.oy}
	}

	sx, sy := a.sxMinus1+1, a.syMinus1+1
	det := sx*sy - a.hx*a.hy
	isx, isy := sy/det, sx/det
	ihx, ihy := -a.hx/det, -a.hy/det
	return Affine2D{
		isx - 1, ihx, -isx*a.ox - ihx*a.oy,
		ihy, isy - 1, -ihy*a.ox - isy*a.oy,
	}
}

// Split separates the transformation into the part that is pure offset and the
// part that scales, shears and rotates.
//
// Putting the offset back gives the transformation it came from, exactly: the
// offset is carried in two fields of its own and moving it out is a copy, not a
// calculation. That is what lets a caller treat a plain move as the cheap thing
// it is without having to prove the two halves still add up.
func (a Affine2D) Split() (srs Affine2D, offset Point) {
	return Affine2D{
		sxMinus1: a.sxMinus1, hx: a.hx, ox: 0,
		hy: a.hy, syMinus1: a.syMinus1, oy: 0,
	}, Point{X: a.ox, Y: a.oy}
}

// String returns the transformation as its two rows, "[[sx hx ox] [hy sy oy]]".
//
// The matrix is printed, not the stored form, so a value read while debugging is
// the transformation rather than the identity-subtracted encoding of it -- which
// would be off by one on the diagonal exactly where somebody is checking whether
// a scale took.
func (a Affine2D) String() string {
	sx, hx, ox, hy, sy, oy := a.Elems()

	// Room for the ordinary case, so that appending does not have to grow the
	// slice: six numbers at six significant digits, each with a point and a sign
	// to spare, plus the brackets and the spaces between them. A number that
	// comes out in exponent form is longer than that and the slice grows, which
	// is why this is an estimate and not a bound.
	const precision = 6
	const perNumber = precision + 2
	s := make([]byte, 0, 6*perNumber+len("[[ ] [ ]]"))

	s = append(s, '[', '[')
	s = strconv.AppendFloat(s, float64(sx), 'g', precision, 32)
	s = append(s, ' ')
	s = strconv.AppendFloat(s, float64(hx), 'g', precision, 32)
	s = append(s, ' ')
	s = strconv.AppendFloat(s, float64(ox), 'g', precision, 32)
	s = append(s, ']', ' ', '[')
	s = strconv.AppendFloat(s, float64(hy), 'g', precision, 32)
	s = append(s, ' ')
	s = strconv.AppendFloat(s, float64(sy), 'g', precision, 32)
	s = append(s, ' ')
	s = strconv.AppendFloat(s, float64(oy), 'g', precision, 32)
	s = append(s, ']', ']')

	return string(s)
}

// isOffsetOnly reports whether the transformation does nothing but move a point.
//
// It is the case [Affine2D.Invert] answers by negating the offset rather than
// inverting a matrix. The two give the same answer here, to the bit -- the
// determinant of such a matrix is exactly one, and dividing by one loses
// nothing. What the short way saves is the work: four divisions by a number
// already known, and six products of terms that are zero, on much the commonest
// transformation a layout builds, which is a stack of offsets and nothing else.
func (a Affine2D) isOffsetOnly() bool {
	return a.sxMinus1 == 0 && a.hx == 0 && a.hy == 0 && a.syMinus1 == 0
}

// scaleAboutZero multiplies each row of the matrix by that axis' factor, which
// is the scaling matrix applied before this transformation.
func (a Affine2D) scaleAboutZero(factor Point) Affine2D {
	sx, sy := a.sxMinus1+1, a.syMinus1+1
	return Affine2D{
		sx*factor.X - 1, a.hx * factor.X, a.ox * factor.X,
		a.hy * factor.Y, sy*factor.Y - 1, a.oy * factor.Y,
	}
}

// rotateAboutZero applies the rotation matrix before this transformation.
//
// The angle is turned into its sine and cosine once, in float64, and narrowed
// after: computing them at float32 width costs the accuracy of the result at
// every angle, and a rotation is the operation whose error a caller is most
// likely to see, as a shape that no longer closes where it started.
func (a Affine2D) rotateAboutZero(radians float32) Affine2D {
	sin64, cos64 := math.Sincos(float64(radians))
	sin, cos := float32(sin64), float32(cos64)

	sx, sy := a.sxMinus1+1, a.syMinus1+1
	return Affine2D{
		sx*cos - a.hy*sin - 1, a.hx*cos - sy*sin, a.ox*cos - a.oy*sin,
		sx*sin + a.hy*cos, a.hx*sin + sy*cos - 1, a.ox*sin + a.oy*cos,
	}
}

// shearAboutZero applies the shearing matrix before this transformation.
//
// Each angle becomes the tangent that says how far one axis leans per unit of
// the other, so a quarter of a right angle leans by exactly one and a zero angle
// leans by nothing.
//
// Every term of a row, the offset included, takes its contribution from the same
// row of the matrix being sheared. Building the offset out of the row it is
// already in instead is wrong in a way that is invisible for as long as the two
// coordinates of the origin are equal -- which is what an origin written as a
// square corner looks like, and what it looked like here.
func (a Affine2D) shearAboutZero(radiansX, radiansY float32) Affine2D {
	tx := float32(math.Tan(float64(radiansX)))
	ty := float32(math.Tan(float64(radiansY)))

	sx, sy := a.sxMinus1+1, a.syMinus1+1
	return Affine2D{
		sx + a.hy*tx - 1, a.hx + sy*tx, a.ox + a.oy*tx,
		sx*ty + a.hy, a.hx*ty + sy - 1, a.ox*ty + a.oy,
	}
}
