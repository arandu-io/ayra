package stroke

import (
	"math"

	"github.com/arandu-io/ayra/engine/internal/f32"
)

// normalAt returns the vector of the given length standing at right angles to a
// segment at one of its ends, at 0 or at 1.
//
// It is the segment's own direction there, turned a quarter turn. A positive
// length lands on the right-hand side of the direction of travel and a negative
// one on the left, which is how the two flanks of a stroke are asked for: the
// same call, the half-width and its negative.
//
// A segment of no length has no direction and gets no normal. Scaling the zero
// vector to a length is a division by zero, and a NaN reaching the vertex buffer
// is a shape that vanishes rather than a shape that is merely wrong.
func normalAt(from, ctrl, to f32.Point, at, length float32) f32.Point {
	var direction f32.Point

	switch at {
	case 0:
		direction = ctrl.Sub(from)
	case 1:
		direction = to.Sub(ctrl)
	default:
		panic("a segment has a normal at its ends and nowhere else")
	}

	if direction.X == 0 && direction.Y == 0 {
		return f32.Point{}
	}

	return scaleTo(quarterTurn(direction), length)
}

// quarterTurn turns a vector a quarter turn.
//
// Which way round that is depends on which way the axes run, and here Y grows
// downwards: this sends the Y axis onto the X axis. It is the one place the
// handedness of the coordinate system is written down, and every side, join and
// cap in this package inherits its sense of "outward" from it -- so the cost of
// having it wrong is not a wrong corner, it is both flanks of every stroke drawn
// on the same side of the path.
func quarterTurn(p f32.Point) f32.Point {
	return f32.Pt(+p.Y, -p.X)
}

// scaleTo returns p pointing the same way and length units long. A negative
// length points it the other way, at that many units.
//
// The exact cases below are not a shortcut for speed. A vector that is already
// the length asked for has to come back bit for bit unchanged, because these are
// added to path points and the results are then compared for equality: a join is
// drawn precisely when the facing of one segment differs from the facing of the
// next. A value that made a needless round trip through a division comes back a
// unit in the last place away from itself, and that is enough to put a corner in
// the middle of a straight line.
func scaleTo(p f32.Point, length float32) f32.Point {
	if (p.X == 0 && p.Y == 0) || length == 0 {
		return f32.Point{}
	}

	onAxis := (p.X == 0 && (p.Y == length || p.Y == -length)) ||
		(p.Y == 0 && (p.X == length || p.X == -length))
	if onAxis {
		return pointedAlong(p, length)
	}

	have := math.Hypot(float64(p.X), float64(p.Y))
	want := float64(length)
	if math.Abs(have-want) < 1e-10 {
		// Already the right length, near enough that scaling it would only move
		// it. The comparison is against the signed length, so a vector that is
		// the right length the wrong way round falls through and is scaled by a
		// negative factor, which turns it.
		return pointedAlong(p, length)
	}

	scale := float32(want / have)
	return f32.Point{X: p.X * scale, Y: p.Y * scale}
}

// pointedAlong gives p the sign of length, leaving how long it is alone.
func pointedAlong(p f32.Point, length float32) f32.Point {
	if math.Signbit(float64(length)) {
		return f32.Point{X: -p.X, Y: -p.Y}
	}
	return f32.Point{X: p.X, Y: p.Y}
}

// lengthOf is how long a vector is.
func lengthOf(p f32.Point) float32 {
	return float32(math.Hypot(float64(p.X), float64(p.Y)))
}

// angleBetween is the turn from one vector to the other in radians, signed by
// which way it turns.
//
// Taken the short way round, and that is the whole of why it is a function. The
// plain difference of two angles is a full turn out whenever the two straddle
// the half turn, where the underlying angle flips from one sign to the other. A
// join reading that difference sweeps the long way about the corner and fills
// the wedge it was there to trim -- and only for corners at that one bearing,
// which is why it draws correctly until it is pointed the wrong way.
func angleBetween(from, to f32.Point) float64 {
	turn := math.Atan2(float64(to.Y), float64(to.X)) -
		math.Atan2(float64(from.Y), float64(from.X))
	return math.Remainder(turn, 2*math.Pi)
}

// pointAt is the point on a quadratic at t.
//
//	B(t) = (1-t)² P0 + 2(1-t)t P1 + t² P2
func pointAt(from, ctrl, to f32.Point, t float32) f32.Point {
	rest := 1 - t

	out := from.Mul(rest * rest)
	out = out.Add(ctrl.Mul(2 * rest * t))
	out = out.Add(to.Mul(t * t))
	return out
}

// between is the point t of the way from p to q.
func between(p, q f32.Point, t float32) f32.Point {
	return f32.Pt(
		(1-t)*p.X+t*q.X,
		(1-t)*p.Y+t*q.Y,
	)
}

// splitAt cuts a quadratic in two at t.
//
// Both halves are the curve they were cut from, read over their own stretch of
// t: sampling either is sampling the original. That is what makes flattening by
// repeated cutting safe -- however many times a curve is divided, the pieces
// never drift off it, so the error is only ever the last straight piece and
// never accumulates along the chain.
func splitAt(from, ctrl, to f32.Point, t float32) (first, second QuadSegment) {
	middle := pointAt(from, ctrl, to, t)

	return QuadSegment{From: from, Ctrl: between(from, ctrl, t), To: middle},
		QuadSegment{From: middle, Ctrl: between(ctrl, to, t), To: to}
}

// flatten breaks a curve into straight pieces, each pushed out by the given
// distance, and returns them as the flat segments the rest of this package
// carries.
//
// The pieces are cut to the bend of the curve rather than by halving it a fixed
// number of times: a piece is flat enough when the area its control point spans,
// measured against the distance it spans it over, is under the tolerance. Where
// the curve is nearly straight that is one piece, and where it turns hard it is
// many -- which is where the pieces are worth paying for.
func flatten(from, ctrl, to f32.Point, distance, flatness float32) StrokeQuads {
	var out StrokeQuads
	tolerance := float64(flatness)

	// The loop condition looks redundant against the break below and is not: a
	// step that comes out NaN satisfies neither comparison, and leaving on the
	// condition is what stops a path carrying one bad coordinate from spinning
	// here forever.
	step := float32(0)
	for step < 1 {
		spread := float64((to.X-from.X)*(ctrl.Y-from.Y) - (to.Y-from.Y)*(ctrl.X-from.X))
		over := math.Hypot(float64(ctrl.X-from.X), float64(ctrl.Y-from.Y))
		if spread*over == 0.0 {
			// Straight, or a control point sitting on an end. There is nothing
			// left to divide, and the division below would be by zero.
			break
		}

		spread /= over
		step = 2.0 * float32(math.Sqrt(tolerance/3.0/math.Abs(spread)))
		if step >= 1.0 {
			// What is left is flat enough whole.
			break
		}

		cut, rest := splitAt(from, ctrl, to, step)
		from, ctrl, to = rest.From, rest.Ctrl, rest.To
		out.addOffsetLine(cut.From, cut.Ctrl, cut.To, distance)
	}

	out.addOffsetLine(from, ctrl, to, distance)
	return out
}

// addOffsetLine appends one piece, pushed out by distance along the normal at
// each of its ends.
//
// Where a run already exists the piece starts from where that run ended, rather
// than from where this piece's own start computes to. The two are the same point
// in arithmetic and not always the same in floating point, and a run whose pieces
// each began where they were calculated to begin rather than where the last one
// finished is a run with a crack at every seam.
func (qs *StrokeQuads) addOffsetLine(from, ctrl, to f32.Point, distance float32) {
	if len(*qs) == 0 {
		from = from.Add(normalAt(from, ctrl, to, 0, distance))
	} else {
		from = (*qs)[len(*qs)-1].Quad.To
	}

	to = to.Add(normalAt(from, ctrl, to, 1, distance))

	*qs = append(*qs, StrokeQuad{
		Quad: QuadSegment{
			From: from,
			Ctrl: from.Add(to).Mul(0.5),
			To:   to,
		},
	})
}

// SplitCubic approximates a cubic Bézier with a chain of quadratic ones,
// appended to quads and handed back.
//
// The storage comes in and goes out because this runs over every path of every
// frame. A slice allocated per curve is a frame's worth of garbage to say
// something the caller already had room for.
func SplitCubic(from, ctrl0, ctrl1, to f32.Point, quads []QuadSegment) []QuadSegment {
	// How close the approximation has to be is taken from how big the curve is.
	// An absolute tolerance would divide a small curve far past anything that
	// can be seen, and leave a large one visibly faceted.
	hull := f32.Rectangle{
		Min: from,
		Max: ctrl0,
	}.Canon().Union(f32.Rectangle{
		Min: ctrl1,
		Max: to,
	}.Canon())

	longest := hull.Dx()
	if height := hull.Dy(); height > longest {
		longest = height
	}
	limit := longest * 0.001

	reduceCubic(&quads, 0, limit*limit, from, ctrl0, ctrl1, to)
	return quads
}

// reduceCubic appends the quadratics standing in for a cubic, halving it until
// each piece is close enough, and answers how many halvings that took.
//
// A quadratic stands in for a cubic by losing its t³ term. Anchored at the
// start, the cubic reads
//
//	P(t) = from + 3t(ctrl0-from) + 3t²(ctrl1-2ctrl0+from) + t³(to-3ctrl1+3ctrl0-from)
//
// and the quadratic sharing that start has control point (3ctrl0-from)/2. Read
// backwards from the other end it gives a second one, with control point
// (3ctrl1-to)/2. Neither is right on its own, each being accurate at the end it
// was anchored to, so the control point taken is the midpoint of the two: it
// spreads the error over the piece instead of piling it against one end.
//
// The worst gap between the cubic and that quadratic comes to sqrt(3)/36 times
// the length of (3ctrl0-from) - (3ctrl1-to). The test below squares both sides
// of the comparison, which keeps a square root out of a function that calls
// itself.
func reduceCubic(quads *[]QuadSegment, splits int, limitSq float32, from, ctrl0, ctrl1, to f32.Point) int {
	const maxSplits = 32

	fromEnd := ctrl0.Mul(3).Sub(from)
	toEnd := ctrl1.Mul(3).Sub(to)
	ctrl := fromEnd.Add(toEnd).Mul(1.0 / 4.0)

	if splits >= maxSplits {
		// A curve still not converged this far down is one whose control points
		// are arranged so that halving barely moves the error -- a cusp, or ends
		// that nearly coincide. The ceiling is what makes this terminate at all.
		*quads = append(*quads, QuadSegment{From: from, Ctrl: ctrl, To: to})
		return splits
	}

	gap := fromEnd.Sub(toEnd)
	gapSq := (gap.X*gap.X + gap.Y*gap.Y) * 3 / (36 * 36)
	if gapSq <= limitSq {
		*quads = append(*quads, QuadSegment{From: from, Ctrl: ctrl, To: to})
		return splits
	}

	// Halve the cubic and approximate each half. The points below are the
	// successive midpoints that a cut at one half walks through, and the last of
	// them is the point on the curve the two halves meet at.
	const half = float32(0.5)
	a := from.Add(ctrl0.Sub(from).Mul(half))
	b := ctrl0.Add(ctrl1.Sub(ctrl0).Mul(half))
	c := ctrl1.Add(to.Sub(ctrl1).Mul(half))
	ab := a.Add(b.Sub(a).Mul(half))
	bc := b.Add(c.Sub(b).Mul(half))
	middle := ab.Add(bc.Sub(ab).Mul(half))

	splits++
	splits = reduceCubic(quads, splits, limitSq, from, a, ab, middle)
	splits = reduceCubic(quads, splits, limitSq, middle, bc, c, to)
	return splits
}
