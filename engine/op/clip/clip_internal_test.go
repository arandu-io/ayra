package clip

import (
	"encoding/binary"
	"image"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/internal/scene"
	"github.com/arandu-io/ayra/engine/op"
)

// written is one operation a drawing left in the list.
type written struct {
	kind    ops.OpType
	bounds  image.Rectangle
	shape   ops.Shape
	outline bool
	width   float32
}

// replay reads back what a drawing wrote.
//
// A clip paints nothing on its own, so there is no picture to look at until
// something is filled through it -- and a machine that can fill is a machine
// with a GPU, which a build server has not got. What there always is, is the
// instruction stream, and every decision made here is a field in it: the area
// claimed, the shape declared, the width a stroke asked for. Reading it back
// is how those are asserted without a display.
func replay(t *testing.T, o *op.Ops) []written {
	t.Helper()

	var (
		out    []written
		reader ops.Reader
	)
	reader.Reset(&o.Internal)
	for {
		encoded, ok := reader.Decode()
		if !ok {
			return out
		}
		switch kind := ops.OpType(encoded.Data[0]); kind {
		case ops.TypeClip:
			var area ops.ClipOp
			area.Decode(encoded.Data)
			out = append(out, written{kind: kind, bounds: area.Bounds, shape: area.Shape, outline: area.Outline})
		case ops.TypeStroke:
			width := math.Float32frombits(binary.LittleEndian.Uint32(encoded.Data[1:]))
			out = append(out, written{kind: kind, width: width})
		case ops.TypePath, ops.TypePopClip:
			out = append(out, written{kind: kind})
		}
	}
}

// areaOf returns the single clip a drawing claimed.
//
// It insists on exactly one, because a test that reads the first of several
// says nothing about the rest -- and an operation written twice is a fault
// that looks like a pass.
func areaOf(t *testing.T, o *op.Ops) written {
	t.Helper()

	var found []written
	for _, w := range replay(t, o) {
		if w.kind == ops.TypeClip {
			found = append(found, w)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the drawing claimed %d areas, want exactly one", len(found))
	}
	return found[0]
}

// curvesOf returns the segments a drawing recorded, in the order written.
//
// The segments live in a recorded macro, so they are only reachable once the
// path has been pushed: a path that is built and never used writes its
// commands where nothing walks them.
func curvesOf(t *testing.T, o *op.Ops) []scene.Command {
	t.Helper()

	const stride = 4 + scene.CommandSize

	var (
		out    []scene.Command
		reader ops.Reader
	)
	reader.Reset(&o.Internal)
	for {
		encoded, ok := reader.Decode()
		if !ok {
			return out
		}
		if ops.OpType(encoded.Data[0]) != ops.TypeAux {
			continue
		}
		for data := encoded.Data[ops.TypeAuxLen:]; len(data) >= stride; data = data[stride:] {
			out = append(out, ops.DecodeCommand(data[4:stride]))
		}
	}
}

// cubicsOf returns only the curved segments, which is what the corner and the
// outline of a round shape are made of.
func cubicsOf(t *testing.T, o *op.Ops) []scene.Command {
	t.Helper()

	var out []scene.Command
	for _, c := range curvesOf(t, o) {
		if c.Op() == scene.OpCubic {
			out = append(out, c)
		}
	}
	return out
}

// midpoint is the curve's own halfway point, which is the one place a cubic
// has to agree with the circle it stands in for.
//
// The end points agree by construction -- they are put there -- and the
// control points never lie on the curve at all. Halfway is where a wrong
// constant shows, and it is the only sample that distinguishes an arc from a
// slightly fatter or flatter one drawn through the same two ends.
func midpoint(c scene.Command) f32.Point {
	from, ctrl0, ctrl1, to := scene.DecodeCubic(c)
	return from.Add(ctrl0.Mul(3)).Add(ctrl1.Mul(3)).Add(to).Mul(1.0 / 8.0)
}

// TestEveryPushIsPopped fixes the one thing a clip stack cannot survive
// getting wrong.
//
// The renderer restores the previous area on a pop, so a push with no pop
// leaves everything drawn afterwards -- for the rest of the frame, including
// whatever the caller has nothing to do with -- confined to an area it never
// asked for. Nothing reports it; the screen simply loses its lower half.
func TestEveryPushIsPopped(t *testing.T) {
	var o op.Ops

	first := Rect(image.Rect(0, 0, 80, 80)).Push(&o)
	second := Rect(image.Rect(10, 10, 70, 70)).Push(&o)
	third := Rect(image.Rect(20, 20, 60, 60)).Push(&o)
	fourth := Rect(image.Rect(30, 30, 50, 50)).Push(&o)
	fourth.Pop()
	third.Pop()
	second.Pop()
	first.Pop()

	depth, deepest := 0, 0
	for _, w := range replay(t, &o) {
		switch w.kind {
		case ops.TypeClip:
			depth++
			deepest = max(deepest, depth)
		case ops.TypePopClip:
			depth--
			if depth < 0 {
				t.Fatalf("an area was released that was never claimed")
			}
		}
	}
	if deepest != 4 {
		t.Errorf("the stack reached depth %d, want 4", deepest)
	}
	if depth != 0 {
		t.Errorf("%d areas were left claimed at the end of the frame, want none", depth)
	}
}

// TestAStackPoppedOutOfTurnPanics keeps the imbalance loud.
//
// Releasing the outer area while an inner one is still claimed cannot be
// honoured -- the renderer has one stack and restores whatever is on top of
// it -- so the alternative to the panic is a frame drawn against the wrong
// area, which looks like a layout fault a long way from the code that caused
// it.
func TestAStackPoppedOutOfTurnPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("releasing the outer area first did not panic")
		}
	}()

	var o op.Ops
	outer := Rect(image.Rect(0, 0, 80, 80)).Push(&o)
	Rect(image.Rect(10, 10, 70, 70)).Push(&o)
	outer.Pop()
}

// TestARectangleClaimsExactlyItsOwnArea is the plainest case and the one
// everything else is measured against.
//
// A rectangle is sent as a shape rather than as four segments, so nothing
// rounds it and nothing flattens it: what the caller asked for is what the
// renderer is told, to the pixel.
func TestARectangleClaimsExactlyItsOwnArea(t *testing.T) {
	want := image.Rect(12, 34, 56, 78)

	var o op.Ops
	Rect(want).Push(&o).Pop()

	got := areaOf(t, &o)
	if got.bounds != want {
		t.Errorf("the clip claimed %v, want %v", got.bounds, want)
	}
	if got.shape != ops.Rect {
		t.Errorf("the clip was sent as shape %d, want the rectangle", got.shape)
	}
	if !got.outline {
		t.Error("the rectangle was not sent as a filled area")
	}
}

// TestClippingARectangleWithItselfClaimsTheSameArea is the identity a nested
// clip has to have.
//
// An area intersected with itself is itself, and a layout that wraps a region
// it has already narrowed -- which is what happens whenever a control is put
// inside a panel of its own size -- must not come out a pixel smaller each
// time.
func TestClippingARectangleWithItselfClaimsTheSameArea(t *testing.T) {
	same := image.Rect(10, 10, 90, 90)

	var o op.Ops
	outer := Rect(same).Push(&o)
	inner := Rect(same).Push(&o)
	inner.Pop()
	outer.Pop()

	var claimed []image.Rectangle
	for _, w := range replay(t, &o) {
		if w.kind == ops.TypeClip {
			claimed = append(claimed, w.bounds)
		}
	}
	if len(claimed) != 2 {
		t.Fatalf("%d areas were claimed, want two", len(claimed))
	}
	if claimed[0] != same || claimed[1] != same {
		t.Errorf("the nested areas are %v and %v, want %v twice", claimed[0], claimed[1], same)
	}
}

// TestAnInvertedRectangleClaimsNoArea fixes what a backwards rectangle means.
//
// A rectangle whose maximum is below its minimum arrives from arithmetic, not
// from a person: a width subtracted from a smaller one, an inset larger than
// the room it was applied to. Kept as it came, it describes an area with
// negative extent, and the honest reading of that is nothing at all -- not an
// area mirrored about its own corner, which is what canonicalising it here
// would silently draw.
func TestAnInvertedRectangleClaimsNoArea(t *testing.T) {
	backwards := image.Rectangle{Min: image.Pt(60, 60), Max: image.Pt(20, 20)}

	var o op.Ops
	Rect(backwards).Push(&o).Pop()

	got := areaOf(t, &o)
	if got.bounds != backwards {
		t.Errorf("the clip claimed %v, want the rectangle unchanged at %v", got.bounds, backwards)
	}
	if !got.bounds.Empty() {
		t.Errorf("the claimed area %v is not empty", got.bounds)
	}
}

// TestAStrokeWidensTheAreaByHalfItsWidth pins the arithmetic that decides how
// much room a line needs.
//
// A stroke is centred on its path, so half of it falls outside the path's own
// bounds. Too little and the outer half of every border is clipped away --
// which on a one pixel rule is the whole of it on one side; too much costs
// nothing visible and is therefore never noticed. The table is here because
// the rounding is the part that is easy to change by accident: the half width
// is rounded away from zero, so an odd width takes the larger margin.
func TestAStrokeWidensTheAreaByHalfItsWidth(t *testing.T) {
	line := image.Rect(20, 20, 60, 60)

	for _, c := range []struct {
		width  float32
		margin int
	}{
		{width: 0.5, margin: 0},
		{width: 1, margin: 1},
		{width: 2, margin: 1},
		{width: 3, margin: 2},
		{width: 4, margin: 2},
		{width: 5, margin: 3},
		{width: 10, margin: 5},
	} {
		var o op.Ops
		var p Path
		p.Begin(&o)
		p.MoveTo(f32.Pt(float32(line.Min.X), float32(line.Min.Y)))
		p.LineTo(f32.Pt(float32(line.Max.X), float32(line.Max.Y)))
		Stroke{Path: p.End(), Width: c.width}.Op().Push(&o).Pop()

		want := line.Inset(-c.margin)
		got := areaOf(t, &o)
		if got.bounds != want {
			t.Errorf("a stroke %v wide claimed %v, want %v", c.width, got.bounds, want)
		}

		var widths []float32
		for _, w := range replay(t, &o) {
			if w.kind == ops.TypeStroke {
				widths = append(widths, w.width)
			}
		}
		if len(widths) != 1 || widths[0] != c.width {
			t.Errorf("a stroke %v wide was sent as %v, want exactly one of that width", c.width, widths)
		}
	}
}

// TestAStrokedRectangleIsSentAsAPath fixes the conversion that keeps a border
// from being a fill.
//
// A rectangle travels as a shape the renderer draws solid. Stroking one asks
// for its edge and not its middle, and there is no width in a shape for the
// renderer to read -- so the four sides have to become real segments before
// the width is attached. Sent as a shape with a width beside it, a one pixel
// border comes out as a filled block.
func TestAStrokedRectangleIsSentAsAPath(t *testing.T) {
	var o op.Ops
	Stroke{Path: Rect(image.Rect(20, 20, 60, 60)).Path(), Width: 2}.Op().Push(&o).Pop()

	segments := 0
	for _, w := range replay(t, &o) {
		if w.kind == ops.TypePath {
			segments++
		}
	}
	if segments != 1 {
		t.Errorf("the stroked rectangle sent %d paths, want one", segments)
	}
	if got := areaOf(t, &o); got.shape != ops.Path {
		t.Errorf("the stroked rectangle was sent as shape %d, want the path", got.shape)
	}
}

// TestAnEmptyPathStillStrokes keeps the width from reaching for segments that
// are not there.
//
// A path with nothing in it is what a caller ends up with whenever the thing
// being outlined turned out to be empty -- a list with no rows, a selection
// that matched nothing. Drawing nothing is the right answer; panicking on the
// frame that happens to have no data is not. What is claimed is the stroke's
// own margin about the origin and nothing else: there is no path to send, so
// there is nothing inside it to fill.
func TestAnEmptyPathStillStrokes(t *testing.T) {
	var o op.Ops
	var p Path
	p.Begin(&o)
	Stroke{Path: p.End(), Width: 3}.Op().Push(&o).Pop()

	for _, w := range replay(t, &o) {
		if w.kind == ops.TypePath {
			t.Error("an empty path was sent to the renderer, want none")
		}
	}
	if got := areaOf(t, &o); got.bounds != image.Rect(-2, -2, 2, 2) {
		t.Errorf("an empty stroke claimed %v, want only its own margin", got.bounds)
	}
}

// TestPathBoundsCoverEverySegment fixes that nothing a path touches falls
// outside the area claimed for it.
//
// The bounds are what the renderer is told to look at. Anything drawn beyond
// them is not drawn wrong, it is not drawn at all -- and a curve that bulges
// one pixel past its end points is exactly the case that survives every test
// written against straight lines.
func TestPathBoundsCoverEverySegment(t *testing.T) {
	for _, c := range []struct {
		name  string
		build func(p *Path)
		want  image.Rectangle
	}{
		{
			name:  "a line",
			build: func(p *Path) { p.MoveTo(f32.Pt(32, 32)); p.LineTo(f32.Pt(64, 64)) },
			want:  image.Rect(32, 32, 64, 64),
		},
		{
			name:  "a quadratic that bulges past its ends",
			build: func(p *Path) { p.MoveTo(f32.Pt(32, 32)); p.QuadTo(f32.Pt(64, 32), f32.Pt(60, 64)) },
			want:  image.Rect(32, 32, 64, 64),
		},
		{
			name: "a cubic that bulges past its ends",
			build: func(p *Path) {
				p.MoveTo(f32.Pt(32, 32))
				p.CubeTo(f32.Pt(64, 32), f32.Pt(32, 64), f32.Pt(48, 48))
			},
			want: image.Rect(32, 32, 64, 64),
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var o op.Ops
			var p Path
			p.Begin(&o)
			c.build(&p)
			spec := p.End()

			if !spec.hasSegments {
				t.Fatal("the path recorded no segments")
			}
			if spec.bounds != c.want {
				t.Errorf("the path claimed %v, want %v", spec.bounds, c.want)
			}
		})
	}
}

// TestAnArcAroundItsOwnCentreClosesTheCircle fixes the one arc that has a
// known answer.
//
// A full turn about a single focus is a circle, and its bounds are the square
// about the centre. It is worth pinning because the arc is the only segment
// built from an angle rather than from points: the number of pieces it is cut
// into is a decision made inside, and too few of them draws a polygon that
// still passes every test asserting where the arc starts and ends.
func TestAnArcAroundItsOwnCentreClosesTheCircle(t *testing.T) {
	const radius = 16
	centre := f32.Pt(48, 48)

	var o op.Ops
	var p Path
	p.Begin(&o)
	p.MoveTo(f32.Pt(centre.X, centre.Y-radius))
	p.ArcTo(centre, centre, 2*math.Pi)
	spec := p.End()

	want := image.Rect(32, 32, 64, 64)
	if !spec.bounds.In(want.Inset(-1)) || !want.Inset(1).In(spec.bounds) {
		t.Errorf("a full turn claimed %v, want the square %v within a pixel", spec.bounds, want)
	}
}

// TestRelativeSegmentsMatchAbsoluteOnes fixes that the two ways of naming a
// point describe the same drawing.
//
// Every segment has a relative form and an absolute one, and they exist so
// that a shape can be written whichever way reads better at the call site.
// The moment they disagree the API has two meanings for one picture, and the
// one that is wrong is whichever a given caller happened not to use.
func TestRelativeSegmentsMatchAbsoluteOnes(t *testing.T) {
	var absolute op.Ops
	var a Path
	a.Begin(&absolute)
	a.MoveTo(f32.Pt(20, 20))
	a.LineTo(f32.Pt(50, 20))
	a.QuadTo(f32.Pt(70, 30), f32.Pt(50, 50))
	a.CubeTo(f32.Pt(40, 60), f32.Pt(30, 60), f32.Pt(20, 50))
	first := a.End()

	var relative op.Ops
	var r Path
	r.Begin(&relative)
	r.Move(f32.Pt(20, 20))
	r.Line(f32.Pt(30, 0))
	r.Quad(f32.Pt(20, 10), f32.Pt(0, 30))
	r.Cube(f32.Pt(-10, 10), f32.Pt(-20, 10), f32.Pt(-30, 0))
	second := r.End()

	if first.bounds != second.bounds {
		t.Errorf("the relative path claimed %v, the absolute one %v", second.bounds, first.bounds)
	}
	if first.hash != second.hash {
		t.Error("the relative path recorded different segments from the absolute one")
	}
}

// TestThePenFollowsTheSegments fixes the position every relative segment is
// measured from.
//
// A pen left behind by one segment is an offset applied to the next, so a
// wrong position is not one bad segment -- it is every segment after it,
// which reads as a shape that drifts rather than as a shape that is wrong.
func TestThePenFollowsTheSegments(t *testing.T) {
	var o op.Ops
	var p Path
	p.Begin(&o)

	p.MoveTo(f32.Pt(10, 10))
	if p.Pos() != f32.Pt(10, 10) {
		t.Fatalf("after a move the pen is at %v, want (10, 10)", p.Pos())
	}
	p.Line(f32.Pt(5, 0))
	if p.Pos() != f32.Pt(15, 10) {
		t.Fatalf("after a relative line the pen is at %v, want (15, 10)", p.Pos())
	}
	p.QuadTo(f32.Pt(20, 10), f32.Pt(20, 20))
	if p.Pos() != f32.Pt(20, 20) {
		t.Fatalf("after a quadratic the pen is at %v, want (20, 20)", p.Pos())
	}
	p.Close()
	if p.Pos() != f32.Pt(10, 10) {
		t.Errorf("after closing, the pen is at %v, want the start at (10, 10)", p.Pos())
	}
}

// TestMovingToThePenWritesNothing keeps a redundant move from splitting a
// contour.
//
// A move ends the contour it was in. Generated paths move to where they
// already are all the time -- a loop that begins each side with a move, a
// caller that restates the start before closing -- and if each of those broke
// the contour, a closed outline would arrive as a handful of open pieces and
// fill as a set of slivers.
func TestMovingToThePenWritesNothing(t *testing.T) {
	var plain op.Ops
	var a Path
	a.Begin(&plain)
	a.MoveTo(f32.Pt(10, 10))
	a.LineTo(f32.Pt(40, 10))
	a.LineTo(f32.Pt(40, 40))
	a.Close()
	first := a.End()

	var restated op.Ops
	var b Path
	b.Begin(&restated)
	b.MoveTo(f32.Pt(10, 10))
	b.MoveTo(f32.Pt(10, 10))
	b.LineTo(f32.Pt(40, 10))
	b.MoveTo(f32.Pt(40, 10))
	b.LineTo(f32.Pt(40, 40))
	b.Close()
	second := b.End()

	if first.hash != second.hash {
		t.Error("restating the pen position changed the path")
	}
	if first.bounds != second.bounds {
		t.Errorf("restating the pen position claimed %v instead of %v", second.bounds, first.bounds)
	}
}

// TestEqualPathsShareAHashAndUnequalOnesDoNot fixes the key a renderer caches
// by.
//
// The hash travels with the path so that the same outline drawn on a hundred
// frames is prepared once. Two different paths that hash alike is the bad
// direction and it is silent: the second one is never prepared, and what
// appears on screen is the first one's shape wearing the second one's
// position.
func TestEqualPathsShareAHashAndUnequalOnesDoNot(t *testing.T) {
	build := func(o *op.Ops, corner f32.Point) PathSpec {
		var p Path
		p.Begin(o)
		p.MoveTo(f32.Pt(10, 10))
		p.LineTo(corner)
		p.LineTo(f32.Pt(10, 40))
		p.Close()
		return p.End()
	}

	var first, second, third op.Ops
	a := build(&first, f32.Pt(40, 10))
	b := build(&second, f32.Pt(40, 10))
	c := build(&third, f32.Pt(41, 10))

	if a.hash != b.hash {
		t.Error("the same path twice produced two keys")
	}
	if a.hash == c.hash {
		t.Error("two different paths produced one key")
	}
}

// TestClosingAPathReturnsThePenToTheStart fixes what closing means.
//
// Closing has to draw the side back to the beginning, not merely declare the
// contour over: the fill rule counts crossings, and a contour left open by a
// pixel is a shape the ray escapes through -- which fills the whole area
// beyond it rather than leaving a small notch.
func TestClosingAPathReturnsThePenToTheStart(t *testing.T) {
	var o op.Ops
	var p Path
	p.Begin(&o)
	p.MoveTo(f32.Pt(10, 10))
	p.LineTo(f32.Pt(40, 10))
	p.LineTo(f32.Pt(40, 40))
	p.Close()
	Outline{Path: p.End()}.Op().Push(&o).Pop()

	if p.Pos() != f32.Pt(10, 10) {
		t.Fatalf("after closing, the pen is at %v, want the start at (10, 10)", p.Pos())
	}

	last := f32.Point{}
	lines := 0
	for _, c := range curvesOf(t, &o) {
		if c.Op() != scene.OpLine {
			continue
		}
		_, to := scene.DecodeLine(c)
		last = to
		lines++
	}
	if lines != 3 {
		t.Errorf("the closed triangle recorded %d sides, want three", lines)
	}
	if last != f32.Pt(10, 10) {
		t.Errorf("the closing side ends at %v, want the start at (10, 10)", last)
	}
}

// TestRoundedCornersLieOnTheirCircle is the arithmetic this package is most
// easily broken by and least likely to be caught at.
//
// A corner is a cubic standing in for a quarter circle, and how far the
// control points reach decides how round it comes out. The constant that sets
// that reach is a single number, and every wrong value still produces a curve
// that starts and ends in the right places and looks plausible at one radius.
// What it stops doing is passing through the circle at the halfway point --
// so that is what is measured, at four radii at once, with a different one on
// each corner so that a pair swapped between them cannot pass either.
func TestRoundedCornersLieOnTheirCircle(t *testing.T) {
	rect := image.Rect(20, 30, 220, 190)
	shape := RRect{Rect: rect, NW: 4, NE: 9, SE: 25, SW: 61}

	var o op.Ops
	shape.Push(&o).Pop()

	cubics := cubicsOf(t, &o)
	if len(cubics) != 4 {
		t.Fatalf("the rounded rectangle recorded %d corners, want four", len(cubics))
	}

	// The order the outline is walked in: north edge first, then round each
	// corner clockwise.
	for i, corner := range []struct {
		name   string
		centre f32.Point
		radius float32
	}{
		{name: "north east", centre: f32.Pt(220-9, 30+9), radius: 9},
		{name: "south east", centre: f32.Pt(220-25, 190-25), radius: 25},
		{name: "south west", centre: f32.Pt(20+61, 190-61), radius: 61},
		{name: "north west", centre: f32.Pt(20+4, 30+4), radius: 4},
	} {
		at := midpoint(cubics[i])
		got := math.Hypot(float64(at.X-corner.centre.X), float64(at.Y-corner.centre.Y))
		if math.Abs(got-float64(corner.radius)) > float64(corner.radius)*1e-4 {
			t.Errorf("the %s corner passes %v from its centre, want its radius %v", corner.name, got, corner.radius)
		}
	}
}

// TestAnEllipseLiesOnItsOwnOutline is the same measurement on the shape that
// is nothing but curve.
//
// A rounded corner is a quarter of a circle and a mistake in it hides in a few
// pixels; an ellipse is four of them end to end and the same mistake is the
// whole outline. The radii are swept rather than sampled once because the
// error a wrong constant produces scales with the radius: a value that is
// visibly wrong on a large circle is within a pixel on a small one, and an
// avatar is small.
func TestAnEllipseLiesOnItsOwnOutline(t *testing.T) {
	for _, bounds := range []image.Rectangle{
		image.Rect(0, 0, 2, 2),
		image.Rect(0, 0, 3, 3),
		image.Rect(10, 10, 17, 17),
		image.Rect(0, 0, 32, 32),
		image.Rect(5, 7, 106, 108),
		image.Rect(0, 0, 513, 513),
		image.Rect(0, 0, 200, 60),
		image.Rect(40, 0, 80, 300),
	} {
		var o op.Ops
		Ellipse(bounds).Push(&o).Pop()

		cubics := cubicsOf(t, &o)
		if len(cubics) != 4 {
			t.Fatalf("the ellipse in %v recorded %d quarters, want four", bounds, len(cubics))
		}

		centre := f32.Pt(float32(bounds.Min.X+bounds.Max.X)/2, float32(bounds.Min.Y+bounds.Max.Y)/2)
		semi := f32.Pt(float32(bounds.Dx())/2, float32(bounds.Dy())/2)

		for quarter, c := range cubics {
			at := midpoint(c)
			x := float64(at.X-centre.X) / float64(semi.X)
			y := float64(at.Y-centre.Y) / float64(semi.Y)
			if got := math.Hypot(x, y); math.Abs(got-1) > 1e-4 {
				t.Errorf("quarter %d of the ellipse in %v passes at %v of its radius, want 1", quarter, bounds, got)
			}
		}
	}
}

// TestAnEllipseWithNoWidthIsNotDrawn keeps a degenerate size from being a
// division.
//
// An ellipse is modelled as a circle stretched in one direction, and the
// stretch is a ratio of the two sides. A side of nothing makes that ratio an
// infinity, and the outline it produces is a set of coordinates no renderer
// can do anything sensible with. A size of nothing draws nothing.
func TestAnEllipseWithNoWidthIsNotDrawn(t *testing.T) {
	for _, bounds := range []image.Rectangle{
		{Min: image.Pt(1, 2), Max: image.Pt(1, 2)},
		image.Rect(10, 10, 10, 40),
		image.Rect(10, 10, 40, 10),
	} {
		var o op.Ops
		spec := Ellipse(bounds).Path(&o)

		if spec.hasSegments {
			t.Errorf("the ellipse in %v recorded segments, want none", bounds)
		}
		if spec.shape != ops.Rect {
			t.Errorf("the ellipse in %v was sent as shape %d, want the empty rectangle", bounds, spec.shape)
		}
	}
}

// TestARoundRectangleWithNoRadiusIsARectangle keeps the cheap case cheap.
//
// Four radii of nothing describe the rectangle itself, and sending it as a
// path means four segments and a curve cache entry for a shape the renderer
// can fill from its corners. It also means a shape whose edges are rasterised
// from segments instead of being pixel aligned, which is a rectangle with soft
// sides.
func TestARoundRectangleWithNoRadiusIsARectangle(t *testing.T) {
	rect := image.Rect(10, 20, 90, 70)

	var o op.Ops
	RRect{Rect: rect}.Push(&o).Pop()

	got := areaOf(t, &o)
	if got.shape != ops.Rect {
		t.Errorf("a rectangle with no corners was sent as shape %d, want the rectangle", got.shape)
	}
	if got.bounds != rect {
		t.Errorf("it claimed %v, want %v", got.bounds, rect)
	}
	if len(curvesOf(t, &o)) != 0 {
		t.Error("a rectangle with no corners recorded segments, want none")
	}
}

// TestUniformCornersAreTheSameOnEverySide fixes the constructor that exists to
// spare the caller writing one radius four times.
//
// It is one line and it is worth a test because the four fields have never
// been in a memorable order: a constructor that filled three of them and left
// the fourth at zero would draw three round corners and one sharp one, which
// reads as a rendering fault rather than as a typing one.
func TestUniformCornersAreTheSameOnEverySide(t *testing.T) {
	rect := image.Rect(0, 0, 40, 40)
	got := UniformRRect(rect, 7)

	if got.Rect != rect {
		t.Errorf("the rectangle came out %v, want %v", got.Rect, rect)
	}
	if got.NW != 7 || got.NE != 7 || got.SE != 7 || got.SW != 7 {
		t.Errorf("the corners came out nw=%d ne=%d se=%d sw=%d, want 7 on each", got.NW, got.NE, got.SE, got.SW)
	}
}

// TestARoundedRectangleStaysInsideItsRectangle fixes that rounding a corner
// only ever takes area away.
//
// The area claimed is what the renderer is told to consider, and a rounded
// rectangle that claimed a pixel beyond its own rectangle would be a control
// painting over its neighbour. Corners cut in; nothing about them reaches out.
func TestARoundedRectangleStaysInsideItsRectangle(t *testing.T) {
	rect := image.Rect(10, 20, 110, 90)

	var o op.Ops
	UniformRRect(rect, 12).Push(&o).Pop()

	if got := areaOf(t, &o); !got.bounds.In(rect) {
		t.Errorf("the rounded rectangle claimed %v, which reaches outside %v", got.bounds, rect)
	}
}
