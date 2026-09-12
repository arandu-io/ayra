package stroke

import (
	"math"

	"github.com/arandu-io/ayra/engine/internal/f32"
)

// strokeTolerance is how far the work here may be out, in both directions it can
// be out in: how far a flattened piece may stray from the curve it stands for,
// and how far apart two runs may end and still be treated as meeting.
//
// One number for both, because they are the same question asked at the two ends
// of the work. The flattening leaves points that are a fraction away from where
// exact arithmetic would have put them; the joining has to accept that fraction,
// or every seam the flattening produced is a seam it refuses to close. A join
// tolerance tighter than the flattening error is a hairline crack at each one.
//
// The value is a trade: smaller draws curves that stay curves under a lens and
// costs pieces, and pieces are vertices on every frame.
const strokeTolerance = 0.01

// segment is one path segment with what the offsetting needs cached alongside
// it: where it runs, and which way each of its two ends faces.
//
// The facings are worked out in a pass of their own, before any offsetting,
// because a join needs the facing of both segments that meet at it -- and the
// second one has not been reached yet at the moment the first is being offset.
type segment struct {
	from, to f32.Point
	ctrl     f32.Point

	// fromNormal and toNormal stand at right angles to the segment at each of
	// its ends, already the half-width long.
	fromNormal, toNormal f32.Point
}

// stroke returns the outline that draws these segments as a stroke.
func (qs StrokeQuads) stroke(style StrokeStyle) StrokeQuads {
	var out StrokeQuads
	half := 0.5 * style.Width

	for _, contour := range qs.contours() {
		rhs, lhs := contour.sides(half)
		if lhs == nil {
			// An open contour comes back as one loop, already closed around
			// both of its caps.
			out = out.append(rhs)
			continue
		}

		// A closed contour comes back as two rings, and a ring is a shape with a
		// hole. The fill has to cancel in the middle or the hole is painted, and
		// what cancels it is running the inner ring against the outer one. Which
		// of the two is inner follows the direction the path was walked in, not
		// the order the two sides were built in -- read from the sides, a square
		// described clockwise and the same square described the other way would
		// come out as a frame and as a filled block.
		if contour.counterClockwise() {
			out = out.append(rhs)
			out = out.append(lhs.reverse())
		} else {
			out = out.append(lhs)
			out = out.append(rhs.reverse())
		}
	}

	return out
}

// sides walks a contour down both of its flanks, half the stroke's width out.
//
// A closed contour comes back as its two rings, for the caller to wind against
// each other. An open one comes back as a single closed loop -- the two flanks,
// the far one walked back, with a cap at each end -- and lhs is nil to say which
// of the two answers this is.
func (qs StrokeQuads) sides(half float32) (rhs, lhs StrokeQuads) {
	closed := qs[0].Quad.From == qs[len(qs)-1].Quad.To

	segments := make([]segment, 0, len(qs))
	for _, q := range qs {
		segments = append(segments, segment{
			from:       q.Quad.From,
			to:         q.Quad.To,
			ctrl:       q.Quad.Ctrl,
			fromNormal: normalAt(q.Quad.From, q.Quad.Ctrl, q.Quad.To, 0, half),
			toNormal:   normalAt(q.Quad.From, q.Quad.Ctrl, q.Quad.To, 1, half),
		})
	}

	for i, seg := range segments {
		rhs = rhs.append(flatten(seg.from, seg.ctrl, seg.to, +half, strokeTolerance))
		lhs = lhs.append(flatten(seg.from, seg.ctrl, seg.to, -half, strokeTolerance))

		var next segment
		switch {
		case i+1 < len(segments):
			next = segments[i+1]
		case closed:
			// The last segment of a closed contour turns into the first, and
			// that corner is a corner like any other. Left unjoined it is the
			// one notch in an otherwise clean ring, and it lands wherever the
			// path happened to start.
			next = segments[0]
		default:
			continue
		}

		// Equal facings mean the path did not turn here, and there is no wedge
		// to fill. Testing the facings rather than the angle between them is
		// what keeps a straight line described in two pieces from being rounded
		// off in the middle.
		if seg.toNormal != next.fromNormal {
			roundJoin(&rhs, &lhs, seg.to, seg.toNormal, next.fromNormal)
		}
	}

	if closed {
		rhs.close()
		lhs.close()
		return rhs, lhs
	}

	// Open, so the two flanks are one loop: walk the far one back the way it
	// came, and cap each end. The caps go on either side of the far flank
	// because a cap is drawn from where the pen is, and after the first one the
	// pen is at the far flank's start.
	first, last := segments[0], segments[len(segments)-1]

	lhs = lhs.reverse()
	roundCap(&rhs, last.to)
	rhs = rhs.append(lhs)
	roundCap(&rhs, first.from)
	rhs.close()

	return rhs, nil
}

// roundJoin fills the wedge two segments leave between them where they meet at
// an angle.
//
// Only the outer side is given an arc. On the inner side the two flanks already
// overlap -- the corner is buried in ink that is drawn anyway -- and an arc
// there would be swept back over covered ground. A straight run to the inner
// offset point is what closes the outline without adding to it.
//
// Which side is outer is the direction of the turn and nothing else: a path
// bending one way puts its outer side on the other.
func roundJoin(rhs, lhs *StrokeQuads, pivot, incoming, outgoing f32.Point) {
	right := pivot.Add(outgoing)
	left := pivot.Sub(outgoing)
	angle := angleBetween(incoming, outgoing)

	if angle <= 0 {
		// Bending right, or turning back on itself.
		centre := pivot.Sub(lhs.pen())
		lhs.arc(centre, centre, float32(angle))
		// The arc lands where the arithmetic puts it, which is a fraction from
		// where the next flank begins. This is the fraction.
		lhs.lineTo(left)
		rhs.lineTo(right)
		return
	}

	// Bending left.
	centre := pivot.Sub(rhs.pen())
	rhs.arc(centre, centre, float32(angle))
	rhs.lineTo(right)
	lhs.lineTo(left)
}

// roundCap closes the end of an open contour with a half turn about the point
// the path itself ended at.
//
// Half a turn and no more: the pen is already a half-width out to one side, so
// sweeping it about the path's own end lands it a half-width out the other side,
// which is exactly where the far flank begins.
func roundCap(qs *StrokeQuads, pivot f32.Point) {
	centre := pivot.Sub(qs.pen())
	qs.arc(centre, centre, math.Pi)
}

// contours divides a run of segments into the sub-paths it holds.
//
// A change of contour number is a break, and a return to a number already seen
// is a break too -- the numbering says where one shape stops, not which shape a
// segment belongs to.
func (qs StrokeQuads) contours() []StrokeQuads {
	var out []StrokeQuads
	var current uint32

	for i, q := range qs {
		if i == 0 || q.Contour != current {
			current = q.Contour
			out = append(out, StrokeQuads{})
		}
		last := len(out) - 1
		out[last] = append(out[last], q)
	}

	return out
}

// counterClockwise reports which way round the path was walked.
//
// The sum is twice the area the path encloses, signed by the winding, and only
// the sign is read -- the magnitude is thrown away, so the factor of two is not
// worth an operation to remove. It is what decides which ring of a closed stroke
// is the hole.
func (qs StrokeQuads) counterClockwise() bool {
	var area float32

	for _, contour := range qs.contours() {
		for i := 1; i < len(contour); i++ {
			to := contour[i].Quad.To
			previous := contour[i-1].Quad.To
			area += (to.X - previous.X) * (to.Y + previous.Y)
		}
	}

	return area <= 0.0
}

// pen is where the run currently ends, and where whatever is added next begins.
func (qs *StrokeQuads) pen() f32.Point {
	return (*qs)[len(*qs)-1].Quad.To
}

// lineTo extends the run with a straight segment to pt.
func (qs *StrokeQuads) lineTo(pt f32.Point) {
	from := qs.pen()
	*qs = append(*qs, StrokeQuad{
		Quad: QuadSegment{
			From: from,
			Ctrl: from.Add(pt).Mul(0.5),
			To:   pt,
		},
	})
}

// arc extends the run with an elliptical arc, sweeping angle radians about the
// two foci -- which are given relative to the pen, because every caller here has
// the offset and not the absolute point.
//
// It is emitted as curves rather than sampled into lines. A join and a cap are
// the two places on a stroke where a polygon shows as facets, being the only
// places the outline turns sharply over a short distance.
func (qs *StrokeQuads) arc(f1, f2 f32.Point, angle float32) {
	pen := qs.pen()
	transform, segments := ArcTransform(pen, f1.Add(pen), f2.Add(pen), angle)

	for range segments {
		from := qs.pen()
		// One step of the transform is half a piece, two is the whole of it. The
		// halfway point is what the control point is derived from: pushed out
		// from it far enough that the curve, which passes at half the distance,
		// passes through it.
		middle := transform.Transform(from)
		to := transform.Transform(middle)
		ctrl := middle.Mul(2).Sub(from.Add(to).Mul(.5))

		*qs = append(*qs, StrokeQuad{
			Quad: QuadSegment{From: from, Ctrl: ctrl, To: to},
		})
	}
}

// close adds the segment from the run's end back to its start, unless they are
// already the same point.
func (qs *StrokeQuads) close() {
	from := (*qs)[len(*qs)-1].Quad.To
	to := (*qs)[0].Quad.From

	if from == to {
		return
	}

	*qs = append(*qs, StrokeQuad{
		Quad: QuadSegment{
			From: from,
			Ctrl: from.Add(to).Mul(0.5),
			To:   to,
		},
	})
}

// reverse walks the run the other way.
//
// Each segment's control point stays where it is. A quadratic read backwards
// bends about the same point; only its ends change places.
func (qs StrokeQuads) reverse() StrokeQuads {
	if len(qs) == 0 {
		return nil
	}

	out := make(StrokeQuads, 0, len(qs))
	for i := len(qs) - 1; i >= 0; i-- {
		q := qs[i]
		q.Quad.From, q.Quad.To = q.Quad.To, q.Quad.From
		out = append(out, q)
	}

	return out
}

// append joins another run onto the end of this one, bridging a gap left by
// rounding.
//
// Two runs meant to meet can end a fraction apart, each having arrived by a
// different chain of arithmetic at what is the same point on paper. The outline
// is about to be filled, so a fraction of daylight is a hole, and a bridging
// segment costs one segment.
//
// A wider gap is left alone, and that is the reason for the tolerance rather
// than a plain inequality: past it the distance is not rounding, it is a cap, a
// join or a shape that ended, and bridging it would draw a line nobody asked
// for.
func (qs StrokeQuads) append(ps StrokeQuads) StrokeQuads {
	switch {
	case len(ps) == 0:
		return qs
	case len(qs) == 0:
		return ps
	}

	from := qs[len(qs)-1].Quad.To
	to := ps[0].Quad.From
	if from != to && lengthOf(from.Sub(to)) < strokeTolerance {
		qs = append(qs, StrokeQuad{
			Quad: QuadSegment{
				From: from,
				Ctrl: from.Add(to).Mul(0.5),
				To:   to,
			},
		})
	}

	return append(qs, ps...)
}
