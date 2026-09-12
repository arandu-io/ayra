// Package stroke turns a stroked path into a filled one.
//
// A stroke is a line with a width, and nothing below this point draws lines:
// the rasteriser fills areas. So a path is walked twice, once down each side at
// half the width, the two walks are joined wherever the path turns a corner and
// closed off wherever it ends, and what comes back is an outline. Filling that
// outline paints the stroke.
//
// Everything here is carried as quadratic Bézier segments, straight ones
// included -- a straight segment is one whose control point is the midpoint of
// its ends. One shape of segment means the offsetting, the joining and the
// flattening are each written once. A straight line carried as a case of its own
// would be a second path through all three, and the two would drift apart at
// exactly the places a corner or a cusp makes the drift visible as a seam.
package stroke

import (
	"encoding/binary"

	"github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/internal/scene"
)

// StrokeStyle is how wide a stroked path is drawn.
//
// It repeats the type the clipping package offers its callers, rather than
// sharing one: that package reaches this one to do the work, and a type owned
// there and named here would close the circle.
//
// Width is the whole of it, because there is one join and one cap and both are
// round. A field naming a choice with a single value is where a second way of
// drawing the same corner gets in.
type StrokeStyle struct {
	Width float32
}

// QuadSegment is one quadratic Bézier segment: where it begins, the point it
// bends around, and where it ends.
type QuadSegment struct {
	From, Ctrl, To f32.Point
}

// Transform moves all three of a segment's points.
//
// The control point travels with the ends and nothing has to be recomputed,
// because an affine transform of a Bézier is the Bézier of its transformed
// points. That is what lets a curve be placed on screen while it is still a
// curve, rather than flattened first and scaled afterwards -- which would fix
// the number of pieces at whatever size it happened to be flattened for.
func (q QuadSegment) Transform(t f32.Affine2D) QuadSegment {
	q.From = t.Transform(q.From)
	q.Ctrl = t.Transform(q.Ctrl)
	q.To = t.Transform(q.To)
	return q
}

// StrokeQuad is a segment together with the contour it belongs to.
//
// The contour number is what separates one sub-path from the next inside a
// single run of commands. Two shapes drawn into one path are two contours, and
// stroking them as one run would cap neither and draw a line from the end of
// the first to the start of the second.
type StrokeQuad struct {
	Contour uint32
	Quad    QuadSegment
}

// StrokeQuads is a run of segments: a path being read, a side being built, or a
// finished outline.
type StrokeQuads []StrokeQuad

// StrokePathCommands reads encoded path commands and returns the outline that
// draws them as a stroke of the given style.
//
// It is the whole of what this package is for. Everything else here is reached
// from it, and the result is meant to be filled -- it is a closed outline, not a
// path with a width still attached.
func StrokePathCommands(style StrokeStyle, commands []byte) StrokeQuads {
	return decodePath(commands).stroke(style)
}

// decodePath reads encoded commands into the segments the offsetting works on.
//
// Each command is preceded by the number of the contour it belongs to. Cubics
// are reduced to quadratics here, so that nothing past this function has to know
// a path can hold them.
func decodePath(commands []byte) StrokeQuads {
	const header = 4
	stride := header + scene.CommandSize

	// Two quadratics per command is what a path of curves tends to come to, and
	// a guess that is close costs one growth instead of several.
	quads := make(StrokeQuads, 0, 2*len(commands)/stride)
	cubic := make([]QuadSegment, 0, 10)

	for len(commands) >= stride {
		contour := binary.LittleEndian.Uint32(commands)
		command := ops.DecodeCommand(commands[header:])

		switch command.Op() {
		case scene.OpLine:
			var q QuadSegment
			q.From, q.To = scene.DecodeLine(command)
			// A line has no control point of its own, and it is given the
			// midpoint of its ends: that is the quadratic which is this line.
			q.Ctrl = q.From.Add(q.To).Mul(.5)
			quads = append(quads, StrokeQuad{Contour: contour, Quad: q})

		case scene.OpGap:
			// A gap moves the pen without drawing, so there is no line under it
			// to widen. Stroking it would paint the move.

		case scene.OpQuad:
			var q QuadSegment
			q.From, q.Ctrl, q.To = scene.DecodeQuad(command)
			quads = append(quads, StrokeQuad{Contour: contour, Quad: q})

		case scene.OpCubic:
			from, ctrl0, ctrl1, to := scene.DecodeCubic(command)
			// The same storage every time round: a cubic is reduced on every
			// frame it is drawn on, and a slice per curve is a frame's worth of
			// garbage for nothing.
			cubic = SplitCubic(from, ctrl0, ctrl1, to, cubic[:0])
			for _, q := range cubic {
				quads = append(quads, StrokeQuad{Contour: contour, Quad: q})
			}

		default:
			panic("unsupported scene command")
		}

		commands = commands[stride:]
	}

	return quads
}
