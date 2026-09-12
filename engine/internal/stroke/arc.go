package stroke

import (
	"math"

	"github.com/arandu-io/ayra/engine/internal/f32"
)

// segmentsPerCircle is how many pieces a whole turn is drawn in, and so how
// fine an arc of less than a whole turn comes out.
//
// Sixteen quadratics round a circle is under a tenth of a pixel out on the
// largest round corner this draws, and a join or a cap is a good deal smaller
// than that. The number is fixed rather than taken from the radius because every
// arc here is drawn at a stroke's half-width: there is no case where one is big
// enough for the difference to show.
const segmentsPerCircle = 16

// ArcTransform returns a transform that steps round an arc, and how many steps
// the arc takes.
//
// The arc is named by the point it starts from, the two foci of the ellipse it
// runs on, and a signed sweep in radians -- two foci rather than a centre and
// two radii, because that is what a caller offsetting a path has: the pen, and
// the point it is turning about.
//
// Applying the transform to the start lands halfway through the first step;
// applying it again lands at the end of that step, which is where the next one
// begins. A caller drawing quadratics wants exactly that pair, an end and a
// point on the way, and gets both from one multiplication each.
//
// One transform is built and reused rather than a sine and a cosine taken per
// step. The steps are equal by construction, so the turn between any two of them
// is the same matrix.
func ArcTransform(p, f1, f2 f32.Point, angle float32) (transform f32.Affine2D, segments int) {
	const anglePerSegment = 2 * math.Pi / segmentsPerCircle

	steps := angle / anglePerSegment
	if steps < 0 {
		steps = -steps
	}
	segments = int(math.Ceil(float64(steps)))
	if segments <= 0 {
		// A sweep of nothing is still one step. Zero would leave the caller's
		// loop with nothing to emit, and the join or cap it was drawing would go
		// missing rather than come out small.
		segments = 1
	}

	rx, ry, tilt := ellipse(p, f1, f2)

	// The turn is taken in a frame where the ellipse is the unit circle, because
	// a rotation is only a rotation there: move the centre to the origin, turn
	// the ellipse's long axis onto the X axis, and divide out the two radii.
	// Then turn, then undo all three.
	centre := f32.Point{
		X: 0.5 * (f1.X + f2.X),
		Y: 0.5 * (f1.Y + f2.Y),
	}

	into := f32.AffineId()
	into = into.Offset(f32.Point{}.Sub(centre))
	into = into.Rotate(f32.Point{}, float32(-tilt))
	into = into.Scale(f32.Point{}, f32.Point{
		X: float32(1 / rx),
		Y: float32(1 / ry),
	})

	// Half a step, so that two applications make one piece and the first lands
	// on the point the piece's control point is derived from.
	step := angle / float32(segments)
	turn := f32.AffineId().Rotate(f32.Point{}, 0.5*step)

	return into.Invert().Mul(turn).Mul(into), segments
}

// ellipse answers the two radii of the ellipse through p with the given foci,
// and how far its long axis is turned from the X axis.
func ellipse(p, f1, f2 f32.Point) (rx, ry, tilt float64) {
	if f1 == f2 {
		// One focus is a circle, and a circle has no long axis to be turned.
		radius := dist(f1, p)
		return radius, radius, 0
	}

	// The sum of the distances to the two foci is the long diameter, and the
	// short one follows from how far apart the foci are.
	long := 0.5 * (dist(f1, p) + dist(f2, p))
	focal := dist(f1, f2) * 0.5
	short := math.Sqrt(long*long - focal*focal)

	switch {
	case long > short:
		rx, ry = long, short
	default:
		rx, ry = short, long
	}

	if f1.X == f2.X {
		// Foci one above the other: the long axis stands upright, and the
		// general case below would be dividing by a horizontal spread of zero.
		tilt = math.Pi / 2
		if f1.Y < f2.Y {
			tilt = -tilt
		}
		return rx, ry, tilt
	}

	spread := float64(f1.X-f2.X) * 0.5
	if spread < 0 {
		spread = -spread
	}
	return rx, ry, math.Acos(spread / focal)
}

// dist is how far apart two points are, worked out in the wider type: these
// feed a division by a radius, and a radius that came out a little short puts
// every point of the arc off the curve it is meant to be on.
func dist(p1, p2 f32.Point) float64 {
	var (
		x1 = float64(p1.X)
		y1 = float64(p1.Y)
		x2 = float64(p2.X)
		y2 = float64(p2.Y)
		dx = x2 - x1
		dy = y2 - y1
	)
	return math.Hypot(dx, dy)
}
