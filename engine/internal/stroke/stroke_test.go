package stroke

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/internal/scene"
)

// The tests are inside the package because most of what can be arithmetically
// wrong here is unexported: a normal that points the wrong way, an angle that
// wrapped, a curvature whose sign flipped. None of those is visible from the
// exported entry point until it has already become a drawn pixel, and a pixel
// is the most expensive place to discover a sign.

// tolerance is what two positions may differ by and still count as the same
// point. It is far below a device pixel and far above the error of a float32
// chain a few dozen operations long.
const tolerance = 1e-4

func TestAQuarterTurnSendsTheYAxisOntoTheXAxis(t *testing.T) {
	// The turn is clockwise on screen, where Y grows downwards. Naming it by
	// the axes it maps rather than by a direction is the point: "clockwise"
	// means the opposite thing on paper, and the opposite thing is the defect
	// that draws every join on the wrong side of its segment.
	if got, want := quarterTurn(f32.Pt(0, 1)), f32.Pt(1, 0); got != want {
		t.Errorf("quarterTurn(%v) = %v, want %v", f32.Pt(0, 1), got, want)
	}
	if got, want := quarterTurn(f32.Pt(1, 0)), f32.Pt(0, -1); got != want {
		t.Errorf("quarterTurn(%v) = %v, want %v", f32.Pt(1, 0), got, want)
	}

	// Four turns come back, which no single-axis check can catch: a rotation
	// that mirrors instead of turning passes one of the two cases above.
	p := f32.Pt(3, -7)
	if got := quarterTurn(quarterTurn(quarterTurn(quarterTurn(p)))); got != p {
		t.Errorf("four quarter turns of %v = %v, want %v", p, got, p)
	}
}

func TestScalingToLengthKeepsDirectionAndFlipsOnANegativeLength(t *testing.T) {
	for _, test := range []struct {
		length float32
		in     f32.Point
		want   f32.Point
	}{
		// Nothing to point at, and nowhere to reach.
		{10, f32.Pt(0, 0), f32.Pt(0, 0)},
		{-10, f32.Pt(0, 0), f32.Pt(0, 0)},
		{0, f32.Pt(3, 4), f32.Pt(0, 0)},

		// Along X: grown, kept and shrunk, against both signs of length and
		// both directions of the vector.
		{+20, f32.Pt(+30, 0), f32.Pt(+20, 0)},
		{+20, f32.Pt(+20, 0), f32.Pt(+20, 0)},
		{+20, f32.Pt(+10, 0), f32.Pt(+20, 0)},
		{+20, f32.Pt(-10, 0), f32.Pt(-20, 0)},
		{+20, f32.Pt(-20, 0), f32.Pt(-20, 0)},
		{+20, f32.Pt(-30, 0), f32.Pt(-20, 0)},
		{-20, f32.Pt(+30, 0), f32.Pt(-20, 0)},
		{-20, f32.Pt(+20, 0), f32.Pt(-20, 0)},
		{-20, f32.Pt(+10, 0), f32.Pt(-20, 0)},
		{-20, f32.Pt(-10, 0), f32.Pt(+20, 0)},
		{-20, f32.Pt(-20, 0), f32.Pt(+20, 0)},
		{-20, f32.Pt(-30, 0), f32.Pt(+20, 0)},

		// And the same along Y, which the exact case tests separately: a
		// condition written for one axis and not copied to the other is a
		// defect that shows on vertical edges only.
		{+20, f32.Pt(0, +30), f32.Pt(0, +20)},
		{+20, f32.Pt(0, +20), f32.Pt(0, +20)},
		{+20, f32.Pt(0, +10), f32.Pt(0, +20)},
		{+20, f32.Pt(0, -10), f32.Pt(0, -20)},
		{+20, f32.Pt(0, -20), f32.Pt(0, -20)},
		{+20, f32.Pt(0, -30), f32.Pt(0, -20)},
		{-20, f32.Pt(0, +30), f32.Pt(0, -20)},
		{-20, f32.Pt(0, +20), f32.Pt(0, -20)},
		{-20, f32.Pt(0, +10), f32.Pt(0, -20)},
		{-20, f32.Pt(0, -10), f32.Pt(0, +20)},
		{-20, f32.Pt(0, -20), f32.Pt(0, +20)},
		{-20, f32.Pt(0, -30), f32.Pt(0, +20)},

		// Off axis, where the answer goes through a division. The two values a
		// unit in the last place apart are not a slip: which one comes out
		// depends on how far the input had to be scaled, and pinning them is
		// what says the scaling is still being done the same way. A rewrite
		// that reached the same answer by a different route would show here
		// first, and on screen much later.
		{+20, f32.Pt(+90, +90), f32.Pt(+14.142137, +14.142137)},
		{+20, f32.Pt(+30, +30), f32.Pt(+14.142136, +14.142136)},
		{+20, f32.Pt(+10, +10), f32.Pt(+14.142136, +14.142136)},
		{+20, f32.Pt(-30, -30), f32.Pt(-14.142136, -14.142136)},
		{+20, f32.Pt(-90, -90), f32.Pt(-14.142137, -14.142137)},
		{+20, f32.Pt(+90, -90), f32.Pt(+14.142137, -14.142137)},
		{+20, f32.Pt(-30, +30), f32.Pt(-14.142136, +14.142136)},
		{-20, f32.Pt(+90, +90), f32.Pt(-14.142137, -14.142137)},
		{-20, f32.Pt(+30, +30), f32.Pt(-14.142136, -14.142136)},
		{-20, f32.Pt(-10, -10), f32.Pt(+14.142136, +14.142136)},
		{-20, f32.Pt(+30, -30), f32.Pt(-14.142136, +14.142136)},
		{-20, f32.Pt(-90, +90), f32.Pt(+14.142137, -14.142137)},

		// Already exactly the length asked for, off axis: back bit for bit as
		// it went in, or turned round if the length was negative.
		{5, f32.Pt(3, 4), f32.Pt(3, 4)},
		{5, f32.Pt(3, -4), f32.Pt(3, -4)},
		{5, f32.Pt(-3, -4), f32.Pt(-3, -4)},
		{5, f32.Pt(-3, 4), f32.Pt(-3, 4)},
		{-5, f32.Pt(3, 4), f32.Pt(-3, -4)},
		{-5, f32.Pt(3, -4), f32.Pt(-3, 4)},
		{-5, f32.Pt(-3, -4), f32.Pt(3, 4)},
		{-5, f32.Pt(-3, 4), f32.Pt(3, -4)},

		// Exactly the length asked for, on axis, which is the other exact case.
		{-7, f32.Pt(-7, 0), f32.Pt(7, 0)},
		{-7, f32.Pt(0, -7), f32.Pt(0, 7)},

		// Plain scaling.
		{10, f32.Pt(3, 4), f32.Pt(6, 8)},
		{2.5, f32.Pt(3, 4), f32.Pt(1.5, 2)},
	} {
		if got := scaleTo(test.in, test.length); got != test.want {
			t.Errorf("scaleTo(%v, %v) = %v, want %v", test.in, test.length, got, test.want)
		}
	}

	// The length is honoured for a direction that is not an exact multiple of
	// an axis, where the answer cannot be written down as a literal.
	got := scaleTo(f32.Pt(1, 1), 4)
	if diff := math.Abs(float64(lengthOf(got)) - 4); diff > tolerance {
		t.Errorf("scaleTo(%v, 4) has length %v, want 4", f32.Pt(1, 1), lengthOf(got))
	}
	if got.X <= 0 || got.Y <= 0 {
		t.Errorf("scaleTo(%v, 4) = %v, want it to keep pointing into the same quadrant", f32.Pt(1, 1), got)
	}
}

func TestTheNormalOfASegmentPointsToItsRightHandSide(t *testing.T) {
	// A segment running along positive X. Its right-hand side on screen is
	// upwards, which is negative Y -- and a positive offset has to land there,
	// because the whole of the outer path is built from positive offsets.
	from, ctrl, to := f32.Pt(0, 0), f32.Pt(5, 0), f32.Pt(10, 0)

	if got, want := normalAt(from, ctrl, to, 0, 2), f32.Pt(0, -2); got != want {
		t.Errorf("normal at the start = %v, want %v", got, want)
	}
	if got, want := normalAt(from, ctrl, to, 1, 2), f32.Pt(0, -2); got != want {
		t.Errorf("normal at the end = %v, want %v", got, want)
	}
	if got, want := normalAt(from, ctrl, to, 0, -2), f32.Pt(0, 2); got != want {
		t.Errorf("normal at the start, offset the other way = %v, want %v", got, want)
	}

	// A segment with no length has no direction, and therefore no side. The
	// alternative is a normal derived from a zero vector, which is a division
	// by zero that reaches the vertex buffer as NaN.
	if got := normalAt(f32.Pt(4, 4), f32.Pt(4, 4), f32.Pt(4, 4), 0, 2); got != (f32.Point{}) {
		t.Errorf("normal of a point = %v, want the zero point", got)
	}
	if got := normalAt(f32.Pt(4, 4), f32.Pt(4, 4), f32.Pt(4, 4), 1, 2); got != (f32.Point{}) {
		t.Errorf("normal of a point at the end = %v, want the zero point", got)
	}
}

func TestTheAngleBetweenTwoNormalsTakesTheShortestArc(t *testing.T) {
	// Both vectors sit just either side of the half turn, where the underlying
	// angle function wraps. Measured as the difference of two angles the answer
	// is nearly a full turn; measured as an arc it is the hair's breadth it
	// looks like. A join drawn from the first answer wraps the whole way round
	// the pivot and fills the shape it was meant to trim.
	const nudge = 0.01
	negativeZero := float32(math.Copysign(0, -1))
	want := math.Atan2(nudge, 1)

	if got := angleBetween(f32.Pt(-1, nudge), f32.Pt(-1, negativeZero)); math.Abs(got-want) > tolerance {
		t.Errorf("angle across the wrap = %v, want %v", got, want)
	}
	if got := angleBetween(f32.Pt(-1, negativeZero), f32.Pt(-1, nudge)); math.Abs(got+want) > tolerance {
		t.Errorf("angle across the wrap, the other way = %v, want %v", got, -want)
	}

	// The sign says which way the path bends, and the join reads it to decide
	// which of the two sides gets the arc.
	if got := angleBetween(f32.Pt(1, 0), f32.Pt(0, 1)); math.Abs(got-math.Pi/2) > tolerance {
		t.Errorf("quarter turn = %v, want %v", got, math.Pi/2)
	}
	if got := angleBetween(f32.Pt(0, 1), f32.Pt(1, 0)); math.Abs(got+math.Pi/2) > tolerance {
		t.Errorf("quarter turn back = %v, want %v", got, -math.Pi/2)
	}
	if got := angleBetween(f32.Pt(1, 2), f32.Pt(1, 2)); got != 0 {
		t.Errorf("angle to itself = %v, want 0", got)
	}
}

func TestSamplingAQuadraticHitsItsEndsAndItsMiddle(t *testing.T) {
	p0, p1, p2 := f32.Pt(0, 0), f32.Pt(10, 20), f32.Pt(20, 0)

	if got := pointAt(p0, p1, p2, 0); got != p0 {
		t.Errorf("sample at 0 = %v, want %v", got, p0)
	}
	if got := pointAt(p0, p1, p2, 1); got != p2 {
		t.Errorf("sample at 1 = %v, want %v", got, p2)
	}
	// The halfway point of a quadratic is the average of the ends and twice the
	// control, which is a place the curve passes through and the control is not.
	if got, want := pointAt(p0, p1, p2, 0.5), f32.Pt(10, 10); !near(got, want) {
		t.Errorf("sample at 0.5 = %v, want %v", got, want)
	}
}

func TestSplittingAQuadraticLeavesBothHalvesOnTheOriginalCurve(t *testing.T) {
	p0, p1, p2 := f32.Pt(0, 0), f32.Pt(10, 20), f32.Pt(20, 0)
	const at = 0.3

	first, second := splitAt(p0, p1, p2, at)

	if first.From != p0 {
		t.Errorf("the first half starts at %v, want %v", first.From, p0)
	}
	if second.To != p2 {
		t.Errorf("the second half ends at %v, want %v", second.To, p2)
	}
	if first.To != second.From {
		t.Errorf("the halves meet at %v and %v, want one point", first.To, second.From)
	}
	if want := pointAt(p0, p1, p2, at); !near(first.To, want) {
		t.Errorf("the halves meet at %v, want the curve's own point %v", first.To, want)
	}

	// Every point of each half has to be a point of the whole, or the split has
	// moved the curve rather than divided it.
	for _, u := range []float32{0, 0.25, 0.5, 0.75, 1} {
		got := pointAt(first.From, first.Ctrl, first.To, u)
		if want := pointAt(p0, p1, p2, at*u); !near(got, want) {
			t.Errorf("the first half at %v = %v, want %v", u, got, want)
		}
		got = pointAt(second.From, second.Ctrl, second.To, u)
		if want := pointAt(p0, p1, p2, at+(1-at)*u); !near(got, want) {
			t.Errorf("the second half at %v = %v, want %v", u, got, want)
		}
	}
}

func TestFlatteningProducesOneUnbrokenChainOffsetFromTheCurve(t *testing.T) {
	const offset = 2
	p0, ctrl, p2 := f32.Pt(0, 0), f32.Pt(10, 20), f32.Pt(20, 0)

	qs := flatten(p0, ctrl, p2, offset, strokeTolerance)
	if len(qs) < 2 {
		t.Fatalf("a curve flattened into %d segments, want several", len(qs))
	}

	// A chain, because the outline is filled: a gap between two segments is a
	// hole the rasteriser fills with whatever is behind it.
	for i := 1; i < len(qs); i++ {
		if qs[i-1].Quad.To != qs[i].Quad.From {
			t.Fatalf("segment %d ends at %v and %d begins at %v", i-1, qs[i-1].Quad.To, i, qs[i].Quad.From)
		}
	}

	// Every segment is flat: the control point of each is the midpoint of its
	// ends, which is what makes the result a polyline that the same quadratic
	// machinery downstream can carry.
	for i, q := range qs {
		if want := q.Quad.From.Add(q.Quad.To).Mul(0.5); !near(q.Quad.Ctrl, want) {
			t.Errorf("segment %d has control point %v, want the midpoint %v", i, q.Quad.Ctrl, want)
		}
	}

	// The ends sit a half-width off the curve's own ends, along the normal.
	if want := p0.Add(normalAt(p0, ctrl, p2, 0, offset)); !near(qs[0].Quad.From, want) {
		t.Errorf("the chain starts at %v, want %v", qs[0].Quad.From, want)
	}
	last := qs[len(qs)-1].Quad
	if distance := lengthOf(last.To.Sub(p2)); math.Abs(float64(distance)-offset) > 1e-2 {
		t.Errorf("the chain ends %v from the curve's end, want %v", distance, float32(offset))
	}
}

func TestReversingSwapsEveryEndAndUndoesItself(t *testing.T) {
	qs := StrokeQuads{
		{Quad: QuadSegment{From: f32.Pt(0, 0), Ctrl: f32.Pt(1, 1), To: f32.Pt(2, 0)}},
		{Quad: QuadSegment{From: f32.Pt(2, 0), Ctrl: f32.Pt(3, -1), To: f32.Pt(4, 0)}},
	}

	back := qs.reverse()
	if len(back) != len(qs) {
		t.Fatalf("reversing %d segments gave %d", len(qs), len(back))
	}
	if back[0].Quad.From != f32.Pt(4, 0) || back[len(back)-1].Quad.To != f32.Pt(0, 0) {
		t.Errorf("reversed path runs %v to %v, want %v to %v",
			back[0].Quad.From, back[len(back)-1].Quad.To, f32.Pt(4, 0), f32.Pt(0, 0))
	}
	// The control point stays where it is. It is the same curve read backwards,
	// and a quadratic read backwards bends around the same point.
	if back[0].Quad.Ctrl != f32.Pt(3, -1) {
		t.Errorf("reversed control point = %v, want %v", back[0].Quad.Ctrl, f32.Pt(3, -1))
	}

	forwards := back.reverse()
	for i := range qs {
		if forwards[i] != qs[i] {
			t.Errorf("segment %d came back as %v, want %v", i, forwards[i], qs[i])
		}
	}

	if got := StrokeQuads(nil).reverse(); got != nil {
		t.Errorf("reversing nothing gave %v, want nothing", got)
	}
}

func TestWindingDirectionFollowsTheOrderThePointsWereGivenIn(t *testing.T) {
	square := func(points ...f32.Point) StrokeQuads {
		var qs StrokeQuads
		for i := 1; i < len(points); i++ {
			qs = append(qs, StrokeQuad{
				Contour: 1,
				Quad:    QuadSegment{From: points[i-1], Ctrl: points[i-1].Add(points[i]).Mul(0.5), To: points[i]},
			})
		}
		return qs
	}

	corners := []f32.Point{f32.Pt(0, 0), f32.Pt(10, 0), f32.Pt(10, 10), f32.Pt(0, 10), f32.Pt(0, 0)}
	reversed := make([]f32.Point, len(corners))
	for i := range corners {
		reversed[i] = corners[len(corners)-1-i]
	}

	one := square(corners...).counterClockwise()
	other := square(reversed...).counterClockwise()
	if one == other {
		t.Errorf("both windings reported ccw = %v; the two orders have to disagree", one)
	}
}

func TestBridgingJoinsTwoRunsThatAlmostMeetAndLeavesARealGapAlone(t *testing.T) {
	head := StrokeQuads{{Quad: QuadSegment{From: f32.Pt(0, 0), Ctrl: f32.Pt(1, 0), To: f32.Pt(2, 0)}}}

	// A hair's breadth apart is a rounding error, and the outline is filled: a
	// gap of any size is a leak.
	nearby := StrokeQuads{{Quad: QuadSegment{From: f32.Pt(2, 1e-5), Ctrl: f32.Pt(3, 1e-5), To: f32.Pt(4, 1e-5)}}}
	joined := head.append(nearby)
	if len(joined) != len(head)+len(nearby)+1 {
		t.Errorf("joining across a rounding error gave %d segments, want %d", len(joined), len(head)+len(nearby)+1)
	}
	for i := 1; i < len(joined); i++ {
		if joined[i-1].Quad.To != joined[i].Quad.From {
			t.Errorf("segment %d ends at %v and %d begins at %v", i-1, joined[i-1].Quad.To, i, joined[i].Quad.From)
		}
	}

	// A real distance is a decision somebody made -- a cap, a join, a separate
	// contour -- and bridging it would draw a line that was never asked for.
	far := StrokeQuads{{Quad: QuadSegment{From: f32.Pt(50, 50), Ctrl: f32.Pt(51, 50), To: f32.Pt(52, 50)}}}
	if got := head.append(far); len(got) != len(head)+len(far) {
		t.Errorf("joining across a real gap gave %d segments, want %d", len(got), len(head)+len(far))
	}

	if got := head.append(nil); len(got) != len(head) {
		t.Errorf("appending nothing gave %d segments, want %d", len(got), len(head))
	}
	if got := StrokeQuads(nil).append(head); len(got) != len(head) {
		t.Errorf("appending to nothing gave %d segments, want %d", len(got), len(head))
	}
}

func TestClosingAddsTheSegmentBackToTheStart(t *testing.T) {
	open := StrokeQuads{
		{Quad: QuadSegment{From: f32.Pt(0, 0), Ctrl: f32.Pt(5, 0), To: f32.Pt(10, 0)}},
		{Quad: QuadSegment{From: f32.Pt(10, 0), Ctrl: f32.Pt(10, 5), To: f32.Pt(10, 10)}},
	}
	qs := open
	qs.close()
	if len(qs) != len(open)+1 {
		t.Fatalf("closing gave %d segments, want %d", len(qs), len(open)+1)
	}
	if last := qs[len(qs)-1].Quad; last.From != f32.Pt(10, 10) || last.To != f32.Pt(0, 0) {
		t.Errorf("the closing segment runs %v to %v, want %v to %v",
			last.From, last.To, f32.Pt(10, 10), f32.Pt(0, 0))
	}

	// Already closed stays as it is, or every pass round the shape would add a
	// segment of no length.
	already := StrokeQuads{{Quad: QuadSegment{From: f32.Pt(0, 0), Ctrl: f32.Pt(5, 0), To: f32.Pt(0, 0)}}}
	before := len(already)
	already.close()
	if len(already) != before {
		t.Errorf("closing a closed path gave %d segments, want %d", len(already), before)
	}
}

func TestAStraightStrokeIsTheSegmentGrownByHalfItsWidth(t *testing.T) {
	const width = 4
	outline := StrokePathCommands(StrokeStyle{Width: width}, encodePath(1, scene.Line(f32.Pt(0, 0), f32.Pt(10, 0))))
	if len(outline) == 0 {
		t.Fatal("a line stroked to nothing")
	}

	// Half the width to either side of the segment, and half the width beyond
	// each end, which is the round cap. A cap drawn as a square would reach the
	// corner instead, and that is the only difference the bounds can see.
	got := endpointBounds(outline)
	want := f32.Rect(-width/2, -width/2, 10+width/2, width/2)
	if !nearRect(got, want) {
		t.Errorf("stroked bounds = %v, want %v", got, want)
	}

	// The outline comes back closed, because it is about to be filled.
	if first, last := outline[0].Quad.From, outline[len(outline)-1].Quad.To; !near(first, last) {
		t.Errorf("the outline runs %v to %v, want one closed loop", first, last)
	}
}

func TestAZeroWidthStrokeEnclosesNoArea(t *testing.T) {
	// Nothing offsets, so the two sides of the outline lie on top of each other
	// and cancel. A width that produced area would paint a shape for a stroke
	// somebody switched off.
	outline := StrokePathCommands(StrokeStyle{Width: 0}, encodePath(1,
		scene.Line(f32.Pt(0, 0), f32.Pt(10, 0)),
		scene.Line(f32.Pt(10, 0), f32.Pt(10, 10)),
		scene.Line(f32.Pt(10, 10), f32.Pt(0, 10)),
		scene.Line(f32.Pt(0, 10), f32.Pt(0, 0)),
	))
	if got := signedArea(outline); math.Abs(got) > tolerance {
		t.Errorf("a zero-width stroke enclosed %v, want nothing", got)
	}
}

func TestAJoinBetweenCollinearSegmentsAddsNoCorner(t *testing.T) {
	const width = 4
	whole := StrokePathCommands(StrokeStyle{Width: width}, encodePath(1,
		scene.Line(f32.Pt(0, 0), f32.Pt(20, 0)),
	))
	halved := StrokePathCommands(StrokeStyle{Width: width}, encodePath(1,
		scene.Line(f32.Pt(0, 0), f32.Pt(10, 0)),
		scene.Line(f32.Pt(10, 0), f32.Pt(20, 0)),
	))

	// Cutting a straight line in two is not a bend, so the outline has to come
	// out with the same corners in the same places. A join that fired anyway
	// would round off the middle of a straight line, and it would only be
	// visible where two segments happen to be collinear -- which is most of the
	// borders this draws.
	wantCorners := corners(whole)
	gotCorners := corners(halved)
	if len(gotCorners) != len(wantCorners) {
		t.Fatalf("the halved line turns at %d points, the whole one at %d: %v against %v",
			len(gotCorners), len(wantCorners), gotCorners, wantCorners)
	}
	for i := range wantCorners {
		if !near(gotCorners[i], wantCorners[i]) {
			t.Errorf("corner %d is at %v, want %v", i, gotCorners[i], wantCorners[i])
		}
	}

	if got, want := endpointBounds(halved), endpointBounds(whole); !nearRect(got, want) {
		t.Errorf("the halved line covers %v, the whole one %v", got, want)
	}
}

func TestAClosedStrokeIsTheSameShapeWoundEitherWay(t *testing.T) {
	const width = 4
	corners := []f32.Point{f32.Pt(0, 0), f32.Pt(10, 0), f32.Pt(10, 10), f32.Pt(0, 10), f32.Pt(0, 0)}

	forwards := make([]scene.Command, 0, len(corners)-1)
	backwards := make([]scene.Command, 0, len(corners)-1)
	for i := 1; i < len(corners); i++ {
		forwards = append(forwards, scene.Line(corners[i-1], corners[i]))
		backwards = append(backwards, scene.Line(corners[len(corners)-i], corners[len(corners)-1-i]))
	}

	one := StrokePathCommands(StrokeStyle{Width: width}, encodePath(1, forwards...))
	other := StrokePathCommands(StrokeStyle{Width: width}, encodePath(1, backwards...))

	// The same square, described in the opposite order. The two sides of the
	// outline swap roles and the inner one is wound against the outer one so
	// that the fill cancels in the middle; if the choice of which to reverse
	// followed the point order rather than the winding, one of these two would
	// come back as a filled square instead of a frame.
	if got, want := endpointBounds(other), endpointBounds(one); !nearRect(got, want) {
		t.Errorf("reversed winding covers %v, want %v", got, want)
	}

	if len(other) != len(one) {
		t.Errorf("reversed winding gave %d segments, want %d", len(other), len(one))
	}

	// The same amount of ink either way. The sign is allowed to differ, because
	// the order the two sides are emitted in follows the winding, and a filled
	// outline does not care which way round it was walked.
	forwardArea, backwardArea := math.Abs(signedArea(one)), math.Abs(signedArea(other))
	if forwardArea == 0 {
		t.Fatal("a stroked square enclosed nothing")
	}
	if math.Abs(forwardArea-backwardArea)/forwardArea > 1e-4 {
		t.Errorf("the two windings enclose %v and %v, want the same shape", forwardArea, backwardArea)
	}

	// A frame rather than a filled square: the inner side is wound against the
	// outer one so that the middle cancels. Were it emitted the same way round,
	// the area would be the outer square whole.
	filled := (10 + float64(width)) * (10 + float64(width))
	if forwardArea/2 >= filled {
		t.Errorf("a stroked square encloses %v, which is the filled square %v rather than a frame around it",
			forwardArea/2, filled)
	}
}

func TestEachContourIsStrokedOnItsOwn(t *testing.T) {
	// Two squares in one path. Stroked together as a single run they would be
	// joined by a segment across the gap between them, which is a line nobody
	// drew.
	first := []scene.Command{
		scene.Line(f32.Pt(0, 0), f32.Pt(10, 0)),
		scene.Line(f32.Pt(10, 0), f32.Pt(10, 10)),
		scene.Line(f32.Pt(10, 10), f32.Pt(0, 0)),
	}
	second := []scene.Command{
		scene.Line(f32.Pt(50, 50), f32.Pt(60, 50)),
		scene.Line(f32.Pt(60, 50), f32.Pt(60, 60)),
		scene.Line(f32.Pt(60, 60), f32.Pt(50, 50)),
	}

	both := append(encodePath(1, first...), encodePath(2, second...)...)
	outline := StrokePathCommands(StrokeStyle{Width: 4}, both)

	alone := StrokePathCommands(StrokeStyle{Width: 4}, encodePath(1, first...))
	if len(outline) <= len(alone) {
		t.Fatalf("two contours gave %d segments, one gave %d", len(outline), len(alone))
	}

	// Nothing in the result may run from one triangle to the other.
	for i, q := range outline {
		if lengthOf(q.Quad.To.Sub(q.Quad.From)) > 30 {
			t.Errorf("segment %d runs from %v to %v, crossing between the contours",
				i, q.Quad.From, q.Quad.To)
		}
	}
}

func TestGapsAreNotStroked(t *testing.T) {
	// A gap moves the pen without drawing. Stroking it would paint the move.
	withGap := decodePath(encodePath(1,
		scene.Line(f32.Pt(0, 0), f32.Pt(10, 0)),
		scene.Gap(f32.Pt(10, 0), f32.Pt(20, 0)),
	))
	if len(withGap) != 1 {
		t.Fatalf("a line and a gap decoded to %d segments, want 1", len(withGap))
	}
	if got, want := withGap[0].Quad.To, f32.Pt(10, 0); got != want {
		t.Errorf("the decoded segment ends at %v, want %v", got, want)
	}
}

func TestDecodingCarriesTheContourAndFillsInAMissingControlPoint(t *testing.T) {
	qs := decodePath(encodePath(7,
		scene.Line(f32.Pt(0, 0), f32.Pt(10, 0)),
		scene.Quad(f32.Pt(10, 0), f32.Pt(15, 5), f32.Pt(20, 0)),
	))
	if len(qs) != 2 {
		t.Fatalf("decoded %d segments, want 2", len(qs))
	}
	for i, q := range qs {
		if q.Contour != 7 {
			t.Errorf("segment %d is on contour %d, want 7", i, q.Contour)
		}
	}
	// A line has no control point of its own, and it is given the midpoint so
	// that everything downstream can treat it as a quadratic.
	if got, want := qs[0].Quad.Ctrl, f32.Pt(5, 0); got != want {
		t.Errorf("the line's control point = %v, want %v", got, want)
	}
	if got, want := qs[1].Quad.Ctrl, f32.Pt(15, 5); got != want {
		t.Errorf("the curve kept control point %v, want %v", got, want)
	}
}

func TestACubicComesBackAsAChainOfQuadraticsOnTheSameCurve(t *testing.T) {
	from, ctrl0, ctrl1, to := f32.Pt(0, 0), f32.Pt(10, 30), f32.Pt(30, 30), f32.Pt(40, 0)
	quads := SplitCubic(from, ctrl0, ctrl1, to, nil)
	if len(quads) < 2 {
		t.Fatalf("a cubic split into %d quadratics, want several", len(quads))
	}

	if quads[0].From != from {
		t.Errorf("the chain starts at %v, want %v", quads[0].From, from)
	}
	if last := quads[len(quads)-1].To; last != to {
		t.Errorf("the chain ends at %v, want %v", last, to)
	}
	for i := 1; i < len(quads); i++ {
		if quads[i-1].To != quads[i].From {
			t.Fatalf("quadratic %d ends at %v and %d begins at %v", i-1, quads[i-1].To, i, quads[i].From)
		}
	}

	// Near enough to the cubic to be indistinguishable at the sizes this draws:
	// the split stops when the remaining error is a thousandth of the curve's
	// own bounding box.
	span := math.Max(float64(quadsBounds(quads).Dx()), float64(quadsBounds(quads).Dy()))
	for _, u := range []float32{0.1, 0.3, 0.5, 0.7, 0.9} {
		want := cubicSample(from, ctrl0, ctrl1, to, u)
		if distance := nearestOn(quads, want); float64(distance) > span*0.01 {
			t.Errorf("the cubic at %v is %v from the chain, want it on it", u, distance)
		}
	}

	// The caller hands in its own storage every frame, and the whole point of
	// taking it is that nothing is allocated per curve.
	reused := make([]QuadSegment, 0, len(quads))
	again := SplitCubic(from, ctrl0, ctrl1, to, reused[:0])
	if len(again) != len(quads) {
		t.Errorf("splitting into reused storage gave %d quadratics, want %d", len(again), len(quads))
	}
}

// cubics are shapes that divide into very different numbers of pieces, which is
// what makes a change in the rule for dividing them visible.
var cubics = []struct {
	name                   string
	from, ctrl0, ctrl1, to f32.Point
	pieces                 int
}{
	{"a gentle arch", f32.Pt(0, 0), f32.Pt(10, 10), f32.Pt(10, 10), f32.Pt(20, 0), 4},
	{
		"a long sweep",
		f32.Pt(-145.90305, 703.21277), f32.Pt(-940.20215, 606.05994),
		f32.Pt(74.58341, 405.815), f32.Pt(104.35474, -241.543), 8,
	},
	{
		"a tight turn",
		f32.Pt(770.35626, 639.77765), f32.Pt(735.57135, 545.07837),
		f32.Pt(286.7138, 853.7052), f32.Pt(286.7138, 890.5413), 16,
	},
	{
		"control points sitting on the ends",
		f32.Pt(0, 0), f32.Pt(0, 0), f32.Pt(100, 100), f32.Pt(100, 100), 33,
	},
}

func TestHowFinelyACubicIsDividedDoesNotDrift(t *testing.T) {
	// The length of the chain is what the vertex buffer pays for, and the
	// accuracy checks above do not pin it: a rule that divided twice as often
	// would still put every piece on the curve and cost twice as much on every
	// frame. The last of these is the one that never converges and stops at the
	// ceiling instead, which is the only thing standing between a cusp and a
	// recursion that does not end.
	for _, test := range cubics {
		t.Run(test.name, func(t *testing.T) {
			got := len(SplitCubic(test.from, test.ctrl0, test.ctrl1, test.to, nil))
			if got != test.pieces {
				t.Errorf("divided into %d quadratics, want %d", got, test.pieces)
			}
		})
	}
}

func BenchmarkSplittingACubic(b *testing.B) {
	for _, test := range cubics {
		b.Run(test.name, func(b *testing.B) {
			quads := make([]QuadSegment, 0, test.pieces)
			for b.Loop() {
				quads = SplitCubic(test.from, test.ctrl0, test.ctrl1, test.to, quads[:0])
			}
			if len(quads) != test.pieces {
				b.Fatalf("divided into %d quadratics, want %d", len(quads), test.pieces)
			}
		})
	}
}

func TestASplitCubicThatIsAlreadyAQuadraticIsLeftWhole(t *testing.T) {
	// The control points of a quadratic raised to a cubic. There is nothing to
	// approximate, so nothing should be subdivided.
	from, to := f32.Pt(0, 0), f32.Pt(20, 0)
	ctrl := f32.Pt(10, 20)
	ctrl0 := from.Add(ctrl.Sub(from).Mul(2.0 / 3.0))
	ctrl1 := to.Add(ctrl.Sub(to).Mul(2.0 / 3.0))

	quads := SplitCubic(from, ctrl0, ctrl1, to, nil)
	if len(quads) != 1 {
		t.Fatalf("an exact quadratic split into %d pieces, want 1", len(quads))
	}
	if !near(quads[0].Ctrl, ctrl) {
		t.Errorf("the recovered control point = %v, want %v", quads[0].Ctrl, ctrl)
	}
}

func TestAnArcTransformStepsRoundTheCircleOneSegmentAtATime(t *testing.T) {
	const radius = 10
	centre := f32.Pt(0, 0)
	start := f32.Pt(radius, 0)

	for _, angle := range []float32{math.Pi / 2, math.Pi, 2 * math.Pi, -math.Pi / 2} {
		transform, segments := ArcTransform(start, centre, centre, angle)
		if segments <= 0 {
			t.Fatalf("an arc of %v was drawn in %d segments", angle, segments)
		}

		at := start
		for i := 0; i < 2*segments; i++ {
			at = transform.Transform(at)
			// Half-segment steps, because the arc reaches its control point by
			// applying the same transform twice. Every one of them has to stay
			// on the circle, or the control points bulge.
			if distance := lengthOf(at.Sub(centre)); math.Abs(float64(distance-radius)) > 1e-3 {
				t.Fatalf("step %d of an arc of %v is %v from the centre, want %v", i, angle, distance, float32(radius))
			}
		}

		// Two applications per segment is the whole sweep.
		want := math.Atan2(float64(start.Y), float64(start.X)) + float64(angle)
		got := math.Atan2(float64(at.Y), float64(at.X))
		if diff := math.Abs(math.Remainder(got-want, 2*math.Pi)); diff > 1e-3 {
			t.Errorf("an arc of %v ended at %v radians, want %v", angle, got, want)
		}
	}
}

func TestAnArcOfNoAngleStillDrawsOneSegment(t *testing.T) {
	// Zero segments would leave the loop that builds the arc with nothing to
	// emit, and the path would silently lose its join.
	_, segments := ArcTransform(f32.Pt(10, 0), f32.Pt(0, 0), f32.Pt(0, 0), 0)
	if segments != 1 {
		t.Errorf("an arc of no angle was drawn in %d segments, want 1", segments)
	}
}

func TestAnEllipticalArcKeepsBothRadii(t *testing.T) {
	// Two distinct foci make an ellipse rather than a circle, and the point
	// stepped round it has to stay on the ellipse -- the sum of its distances
	// to the two foci is what does not change.
	f1, f2 := f32.Pt(-6, 0), f32.Pt(6, 0)
	start := f32.Pt(10, 0)
	want := dist(f1, start) + dist(f2, start)

	transform, segments := ArcTransform(start, f1, f2, 2*math.Pi)
	at := start
	for i := 0; i < 2*segments; i++ {
		at = transform.Transform(at)
		got := dist(f1, at) + dist(f2, at)
		if math.Abs(got-want) > 1e-2 {
			t.Fatalf("step %d sums %v to the foci, want %v", i, got, want)
		}
	}
}

func TestTransformingASegmentMovesAllThreeOfItsPoints(t *testing.T) {
	q := QuadSegment{From: f32.Pt(0, 0), Ctrl: f32.Pt(1, 2), To: f32.Pt(3, 4)}
	offset := f32.AffineId().Offset(f32.Pt(10, 20))

	got := q.Transform(offset)
	want := QuadSegment{From: f32.Pt(10, 20), Ctrl: f32.Pt(11, 22), To: f32.Pt(13, 24)}
	if got != want {
		t.Errorf("transformed segment = %v, want %v", got, want)
	}
}

// near reports whether two points are the same place.
func near(a, b f32.Point) bool {
	return math.Abs(float64(a.X-b.X)) <= tolerance && math.Abs(float64(a.Y-b.Y)) <= tolerance
}

// nearRect reports whether two rectangles cover the same ground.
func nearRect(a, b f32.Rectangle) bool {
	return near(a.Min, b.Min) && near(a.Max, b.Max)
}

// encodePath lays out scene commands the way the operation list does, so that a
// test enters the package by the door the renderer uses.
func encodePath(contour uint32, commands ...scene.Command) []byte {
	out := make([]byte, 0, len(commands)*(scene.CommandSize+4))
	for _, command := range commands {
		header := make([]byte, 4+scene.CommandSize)
		binary.LittleEndian.PutUint32(header, contour)
		ops.EncodeCommand(header[4:], command)
		out = append(out, header...)
	}
	return out
}

// endpointBounds is the ground the outline's ends cover.
//
// The control points are left out on purpose: a quadratic approximating an arc
// puts its control point outside the arc, so a box drawn round the controls is
// bigger than the shape and would have to be compared against a fudge factor
// instead of against the width somebody asked for.
func endpointBounds(qs StrokeQuads) f32.Rectangle {
	box := f32.Rectangle{
		Min: f32.Pt(float32(math.Inf(+1)), float32(math.Inf(+1))),
		Max: f32.Pt(float32(math.Inf(-1)), float32(math.Inf(-1))),
	}
	for _, q := range qs {
		for _, p := range [...]f32.Point{q.Quad.From, q.Quad.To} {
			box.Min.X = min(box.Min.X, p.X)
			box.Min.Y = min(box.Min.Y, p.Y)
			box.Max.X = max(box.Max.X, p.X)
			box.Max.Y = max(box.Max.Y, p.Y)
		}
	}
	return box
}

// quadsBounds is the ground a chain of quadratics covers, controls included,
// which is the hull the splitter measures its own error against.
func quadsBounds(quads []QuadSegment) f32.Rectangle {
	var qs StrokeQuads
	for _, q := range quads {
		qs = append(qs, StrokeQuad{Quad: q}, StrokeQuad{Quad: QuadSegment{From: q.Ctrl, To: q.Ctrl}})
	}
	return endpointBounds(qs)
}

// signedArea is twice the area the outline encloses, by the shoelace sum over
// its ends. Twice, because the factor is the same on both sides of every
// comparison made with it, and halving it would only invite a reader to think
// the number means square pixels.
func signedArea(qs StrokeQuads) float64 {
	var total float64
	for _, q := range qs {
		total += float64(q.Quad.From.X)*float64(q.Quad.To.Y) - float64(q.Quad.To.X)*float64(q.Quad.From.Y)
	}
	return total
}

// corners is the points where the outline actually turns.
//
// A stroked path carries a point wherever a segment ended, whether or not the
// direction changed there. Comparing two outlines point by point therefore
// reports a difference for a path that was merely described in more pieces;
// comparing where they turn asks the question that matters.
func corners(qs StrokeQuads) []f32.Point {
	points := make([]f32.Point, 0, len(qs)+1)
	for i, q := range qs {
		if i == 0 {
			points = append(points, q.Quad.From)
		}
		points = append(points, q.Quad.To)
	}

	turns := make([]f32.Point, 0, len(points))
	for i := 1; i < len(points)-1; i++ {
		in := points[i].Sub(points[i-1])
		out := points[i+1].Sub(points[i])
		// The cross product of the two directions, which is zero exactly when
		// they are the same direction and the path carried straight on.
		if cross := in.X*out.Y - in.Y*out.X; math.Abs(float64(cross)) > 1e-3 {
			turns = append(turns, points[i])
		}
	}
	return turns
}

// cubicSample is the point on a cubic at t, which the test needs and the
// package does not: everything downstream of the splitter sees quadratics.
func cubicSample(p0, p1, p2, p3 f32.Point, t float32) f32.Point {
	u := 1 - t
	out := p0.Mul(u * u * u)
	out = out.Add(p1.Mul(3 * u * u * t))
	out = out.Add(p2.Mul(3 * u * t * t))
	out = out.Add(p3.Mul(t * t * t))
	return out
}

// nearestOn is how far a point is from a chain of quadratics, sampled finely
// enough that the answer is about the chain rather than about the sampling.
func nearestOn(quads []QuadSegment, p f32.Point) float32 {
	nearest := float32(math.Inf(+1))
	for _, q := range quads {
		for step := 0; step <= 64; step++ {
			at := pointAt(q.From, q.Ctrl, q.To, float32(step)/64)
			nearest = min(nearest, lengthOf(at.Sub(p)))
		}
	}
	return nearest
}
