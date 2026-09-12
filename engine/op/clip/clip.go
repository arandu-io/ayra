package clip

import (
	"encoding/binary"
	"hash/maphash"
	"image"
	"math"

	"github.com/arandu-io/ayra/engine/f32"
	f32internal "github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/internal/scene"
	"github.com/arandu-io/ayra/engine/internal/stroke"
	"github.com/arandu-io/ayra/engine/op"
)

// Op is an area, and the only thing that narrows where a drawing may go.
//
// It is not filled in directly. [Outline] asks for everything a path encloses,
// [Stroke] asks for the path itself widened into a band, and each shape here
// answers with one of the two. Three ways of saying it and one thing said, so
// that what reaches the renderer is a single kind of area rather than a family
// of them.
type Op struct {
	path PathSpec

	outline bool
	width   float32
}

// Push narrows the area to this one, and hands back the means to restore what
// was in force before.
//
// The result has to be released, and released in the recording it was taken
// in. Both are checked rather than trusted, because neither is recoverable
// afterwards: an area left claimed confines everything drawn for the rest of
// the frame -- including drawing the caller has nothing to do with -- and an
// area released in another recording restores a state that was never saved
// where the renderer is looking.
func (p Op) Push(o *op.Ops) Stack {
	id, macroID := ops.PushOp(&o.Internal, ops.ClipStack)
	p.add(o)
	return Stack{ops: &o.Internal, id: id, macroID: macroID}
}

// add writes the area into the list.
//
// Three things in a fixed order: the segments, then the width they are to be
// widened by, then the area itself. The area is last because it is the one the
// renderer acts on, and it has to find the other two already there.
func (p Op) add(o *op.Ops) {
	path := p.path

	if !path.hasSegments && p.width > 0 {
		// A shape carries no segments -- it is a name and a rectangle -- so
		// there is nothing in it for a width to widen. Stroking one asks for
		// its edge, and the edge has to become real segments first. Sent as it
		// came with a width beside it, a hairline border arrives as a filled
		// block.
		switch p.path.shape {
		case ops.Rect:
			b := f32internal.FRect(path.bounds)
			var rect Path
			rect.Begin(o)
			rect.MoveTo(b.Min)
			rect.LineTo(f32.Pt(b.Max.X, b.Min.Y))
			rect.LineTo(b.Max)
			rect.LineTo(f32.Pt(b.Min.X, b.Max.Y))
			rect.Close()
			path = rect.End()
		case ops.Path:
			// An empty path. Nothing to widen and nothing to draw, which is
			// the right answer on the frame where whatever was being outlined
			// turned out to hold nothing.
		default:
			panic("clip: this shape cannot be stroked")
		}
	}

	bo := binary.LittleEndian

	if path.hasSegments {
		data := ops.Write(&o.Internal, ops.TypePathLen)
		data[0] = byte(ops.TypePath)
		bo.PutUint64(data[1:], path.hash)
		path.spec.Add(o)
	}

	bounds := path.bounds
	if p.width > 0 {
		// A stroke is centred on its path, so half the width falls outside the
		// path's own extent and the area has to grow to hold it. Rounded away
		// from zero: the area is what the renderer is told to consider, and a
		// margin one short clips the outer half of the line away all the way
		// round -- which leaves a border that is simply thinner than asked
		// for, at every width, and so is never read as clipping.
		half := int(p.width*.5 + .5)
		bounds.Min.X -= half
		bounds.Min.Y -= half
		bounds.Max.X += half
		bounds.Max.Y += half

		data := ops.Write(&o.Internal, ops.TypeStrokeLen)
		data[0] = byte(ops.TypeStroke)
		bo.PutUint32(data[1:], math.Float32bits(p.width))
	}

	data := ops.Write(&o.Internal, ops.TypeClipLen)
	data[0] = byte(ops.TypeClip)
	bo.PutUint32(data[1:], uint32(bounds.Min.X))
	bo.PutUint32(data[5:], uint32(bounds.Min.Y))
	bo.PutUint32(data[9:], uint32(bounds.Max.X))
	bo.PutUint32(data[13:], uint32(bounds.Max.Y))
	if p.outline {
		data[17] = 1
	}
	data[18] = byte(path.shape)
}

// Stack is a claimed area, and the means to give it back.
//
// It is a value the caller holds rather than something released when Push
// returns, because a control decides for itself where the narrowing ends: one
// that confines its background and then widens again for its label does both
// inside a single call.
type Stack struct {
	ops     *ops.Ops
	id      ops.StackID
	macroID uint32
}

// Pop restores the area that was in force before the push.
func (s Stack) Pop() {
	ops.PopOp(s.ops, ops.ClipStack, s.id, s.macroID)
	data := ops.Write(s.ops, ops.TypePopClipLen)
	data[0] = byte(ops.TypePopClip)
}

// PathSpec is a finished outline, ready to be asked for as an area.
//
// It holds no segments of its own. The commands stay in the list they were
// written into and this carries the reference to them, the extent they cover
// and the key they are recognised by. That is what lets an outline be built
// during a frame without allocating, and a screen builds a few hundred of them
// per frame.
type PathSpec struct {
	spec op.CallOp
	// hasSegments tracks whether there are any segments in the path.
	hasSegments bool
	bounds      image.Rectangle
	shape       ops.Shape
	hash        uint64
}

// pathSeed keys the identity every outline carries.
//
// One seed for the process, so that the same outline built on two frames is
// recognised as the same and prepared once. It is made rather than fixed
// because the key is only ever compared within a single run, and a fixed seed
// is a set of collisions anybody can work out in advance.
var pathSeed = maphash.MakeSeed()

// Path builds an outline out of lines and Bézier curves.
//
// It writes straight into the list it was begun on and allocates nothing, so
// an outline may be rebuilt from scratch on every frame -- which is what a
// screen that animates anything does.
//
// The pen starts at the origin. Every segment is drawn from wherever the pen
// is and leaves it at the far end, and the relative form of each -- [Path.Line]
// beside [Path.LineTo] -- is measured from there.
type Path struct {
	ops         *ops.Ops
	contour     int
	pen         f32.Point
	macro       op.MacroOp
	start       f32.Point
	hasSegments bool
	bounds      f32internal.Rectangle
	hash        maphash.Hash
}

// Pos returns the current pen position.
func (p *Path) Pos() f32.Point { return p.pen }

// Begin starts an outline, writing its segments into o.
//
// [Path.End] finishes it and has to be called: the segments are recorded
// rather than appended, and a recording left open swallows every operation
// written after it.
func (p *Path) Begin(o *op.Ops) {
	*p = Path{
		ops:     &o.Internal,
		macro:   op.Record(o),
		contour: 1,
	}
	p.hash.SetSeed(pathSeed)
	ops.BeginMulti(p.ops)
	data := ops.WriteMulti(p.ops, ops.TypeAuxLen)
	data[0] = byte(ops.TypeAux)
}

// End finishes the outline and returns it.
func (p *Path) End() PathSpec {
	p.gap()
	c := p.macro.Stop()
	ops.EndMulti(p.ops)
	return PathSpec{
		spec:        c,
		hasSegments: p.hasSegments,
		bounds:      p.bounds.Round(),
		hash:        p.hash.Sum64(),
	}
}

// Move moves the pen by delta without drawing.
func (p *Path) Move(delta f32.Point) {
	to := delta.Add(p.pen)
	p.MoveTo(to)
}

// MoveTo moves the pen to an absolute point without drawing.
//
// It ends the contour it was in, so the next segment starts a new one. Moving
// to where the pen already is does nothing at all: generated outlines restate
// their position constantly -- a loop that begins each side with a move, a
// caller that names the start again before closing -- and if each of those
// split the contour, one closed outline would arrive as a handful of open
// pieces and fill as slivers.
func (p *Path) MoveTo(to f32.Point) {
	if p.pen == to {
		return
	}
	p.gap()
	p.end()
	p.pen = to
	p.start = to
}

// gap records that the contour was left open where it is.
//
// A closed contour begins and ends in the same place. When the pen is
// somewhere else the shape has a hole in its boundary, and saying so is better
// than leaving the renderer to discover it: what it does with a gap it was
// told about is bounded, and what it does with one it infers is whatever the
// crossing count happens to come to.
func (p *Path) gap() {
	if p.pen != p.start {
		data := ops.WriteMulti(p.ops, scene.CommandSize+4)
		bo := binary.LittleEndian
		bo.PutUint32(data[0:], uint32(p.contour))
		p.cmd(data[4:], scene.Gap(p.pen, p.start))
	}
}

// end completes the current contour.
func (p *Path) end() {
	p.contour++
}

// Line draws a line from the pen, moving it by delta.
func (p *Path) Line(delta f32.Point) {
	to := delta.Add(p.pen)
	p.LineTo(to)
}

// LineTo draws a line from the pen to an absolute point.
func (p *Path) LineTo(to f32.Point) {
	if to == p.pen {
		return
	}
	data := ops.WriteMulti(p.ops, scene.CommandSize+4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	p.cmd(data[4:], scene.Line(p.pen, to))
	p.expand(p.pen)
	p.expand(to)
	p.pen = to
}

// cmd writes one segment and folds it into the outline's identity.
//
// The identity is taken from the encoded bytes rather than from the points
// that produced them, so that two ways of arriving at the same segment -- a
// relative move and the absolute one it works out to -- are one outline to
// whatever caches it.
func (p *Path) cmd(data []byte, c scene.Command) {
	ops.EncodeCommand(data, c)
	p.hash.Write(data)
}

// expand grows the extent to include a point.
//
// Control points are included along with the ends, which covers more than the
// curve actually reaches: a curve stays inside the hull of the points that
// define it, so the extent is right in the direction that matters. Too large
// costs a renderer a little work it turns out not to need; too small is a
// shape with its bulge shaved off, and that is the direction a bound computed
// from the end points alone errs in.
func (p *Path) expand(pt f32.Point) {
	if !p.hasSegments {
		p.hasSegments = true
		p.bounds = f32internal.Rectangle{Min: pt, Max: pt}
		return
	}

	b := p.bounds
	if pt.X < b.Min.X {
		b.Min.X = pt.X
	}
	if pt.Y < b.Min.Y {
		b.Min.Y = pt.Y
	}
	if pt.X > b.Max.X {
		b.Max.X = pt.X
	}
	if pt.Y > b.Max.Y {
		b.Max.Y = pt.Y
	}
	p.bounds = b
}

// Quad draws a quadratic Bézier from the pen, with both the control point and
// the end given relative to it.
func (p *Path) Quad(ctrl, to f32.Point) {
	ctrl = ctrl.Add(p.pen)
	to = to.Add(p.pen)
	p.QuadTo(ctrl, to)
}

// QuadTo draws a quadratic Bézier from the pen to to, bending towards ctrl.
//
// The curve passes through neither control point; it is pulled towards them.
func (p *Path) QuadTo(ctrl, to f32.Point) {
	if ctrl == p.pen && to == p.pen {
		return
	}
	data := ops.WriteMulti(p.ops, scene.CommandSize+4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	p.cmd(data[4:], scene.Quad(p.pen, ctrl, to))
	p.expand(p.pen)
	p.expand(ctrl)
	p.expand(to)
	p.pen = to
}

// ArcTo draws an elliptical arc from the pen, along the ellipse whose focus
// points are f1 and f2, turning angle radians. A positive angle turns counter
// clockwise and a negative one clockwise. Equal focus points describe a
// circle.
//
// The arc is cut into a fixed number of pieces per turn and each is drawn as a
// quadratic, because a renderer is handed curves and not angles. How many
// pieces is the one number here that trades accuracy against work, and it is
// not a per-call choice: an outline whose smoothness depended on who drew it
// would have corners in some screens and not in others.
func (p *Path) ArcTo(f1, f2 f32.Point, angle float32) {
	m, segments := stroke.ArcTransform(p.pen, f1, f2, angle)
	for range segments {
		p0 := p.pen
		p1 := m.Transform(p0)
		p2 := m.Transform(p1)
		ctl := p1.Mul(2).Sub(p0.Add(p2).Mul(.5))
		p.QuadTo(ctl, p2)
	}
}

// Arc is [Path.ArcTo] with the focus points given relative to the pen.
func (p *Path) Arc(f1, f2 f32.Point, angle float32) {
	f1 = f1.Add(p.pen)
	f2 = f2.Add(p.pen)
	p.ArcTo(f1, f2, angle)
}

// Cube draws a cubic Bézier from the pen, with the control points and the end
// given relative to it.
func (p *Path) Cube(ctrl0, ctrl1, to f32.Point) {
	p.CubeTo(p.pen.Add(ctrl0), p.pen.Add(ctrl1), p.pen.Add(to))
}

// CubeTo draws a cubic Bézier from the pen to to, through the control points
// ctrl0 and ctrl1.
//
// Two control points is what a quarter circle needs, so this is what every
// rounded corner and every round shape here is built from.
func (p *Path) CubeTo(ctrl0, ctrl1, to f32.Point) {
	if ctrl0 == p.pen && ctrl1 == p.pen && to == p.pen {
		return
	}
	data := ops.WriteMulti(p.ops, scene.CommandSize+4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	p.cmd(data[4:], scene.Cubic(p.pen, ctrl0, ctrl1, to))
	p.expand(p.pen)
	p.expand(ctrl0)
	p.expand(ctrl1)
	p.expand(to)
	p.pen = to
}

// Close closes the current contour.
//
// It draws the side back to where the contour began rather than merely
// declaring it over, because what is inside an outline is decided by counting
// crossings and a boundary left open by a pixel is one the count escapes
// through. The result is not a shape with a nick in it; it is a fill that runs
// away across everything past the gap.
func (p *Path) Close() {
	if p.pen != p.start {
		p.LineTo(p.start)
	}
	p.end()
}

// Stroke is a path widened into a band, which is how a border, a rule or any
// other drawn line is asked for.
type Stroke struct {
	// Path is the line to widen.
	Path PathSpec
	// Width is how wide to widen it. The band is centred on the path, so half
	// of it falls on either side.
	Width float32
}

// Op returns the area the widened line covers.
func (s Stroke) Op() Op {
	return Op{
		path:  s.Path,
		width: s.Width,
	}
}

// Outline is everything a path encloses, which is how a filled shape is asked
// for.
//
// It is one field away from [Stroke] and means the opposite: the inside rather
// than the line itself. Choosing the wrong one draws a control's border as a
// solid block over its own label, which reads as a painting order fault and is
// not one.
type Outline struct {
	// Path is the outline to fill inside of.
	Path PathSpec
}

// Op returns the area inside the path.
func (o Outline) Op() Op {
	return Op{
		path:    o.Path,
		outline: true,
	}
}
