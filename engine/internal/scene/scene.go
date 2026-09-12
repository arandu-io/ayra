// Package scene encodes the drawing commands the rasteriser reads.
//
// A command is a fixed run of 32-bit words: an op in the first, its arguments
// in the rest. Every command takes the same room whether or not it fills it,
// which is what lets a reader walk the stream by stride rather than decode each
// command to learn how long it was. Both readers of it do exactly that, and the
// padding is what they are paying for.
//
// Nothing in here is checked by the compiler. Every word is the same type, so a
// coordinate written into the wrong one builds, runs, and draws a shape nobody
// asked for. That is why the encoder and the decoder of each op are written to
// mirror each other rather than to be short, and why the packing of a point
// appears in one place rather than at each of the fifteen it is written at.
//
// Only the four path ops are produced and read in this module: paths are built
// by the clip package and decoded into quadratics for filling and for stroking.
// The rest of the ops -- colour, transform, clip bounds, image, line width, fill
// mode -- are the encoding's other half, and nothing here writes or reads one.
// They are kept because the format is shared with a renderer that does, and a
// number dropped from the middle of it renumbers everything after it.
package scene

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"unsafe"

	"github.com/arandu-io/ayra/engine/internal/f32"
)

// Op says which command this is.
//
// The number is what goes into the stream, so the order of the constants below
// is the contract and not a detail: an op inserted in the middle renumbers
// every one after it, and anything reading a stream written before the change
// decodes every command as the wrong one.
type Op uint32

const (
	OpNop Op = iota
	OpLine
	OpQuad
	OpCubic
	OpFillColor
	OpLineWidth
	OpTransform
	OpBeginClip
	OpEndClip
	OpFillImage
	OpSetFillMode
	// OpGap is last rather than beside OpLine, which it resembles, because it
	// was added after the numbers above it were already being read.
	OpGap
)

// FillMode says how the area a path encloses is filled.
type FillMode uint32

const (
	// FillModeNonzero fills by the nonzero winding rule, which is what a filled
	// shape means.
	FillModeNonzero = 0
	// FillModeStroke fills the band a stroked path covers.
	FillModeStroke = 1
)

// commandWords is how many words one command occupies, and it is what the
// longest of them needs: a cubic is an op and four points.
//
// It is written as the sum rather than as nine so that the number and its
// reason cannot drift apart, and because [CommandSize] -- the stride every
// reader of the stream advances by -- is measured from it.
const commandWords = 1 + 4*2

// Command is one drawing command: an op in the first word, its arguments in the
// rest.
type Command [commandWords]uint32

// CommandSize is the stride of a command in the byte stream, and what a reader
// advances by.
//
// It is measured from the type rather than stated, because a stated size that
// stops matching the array is an offset that slips by the difference on every
// command: the first shape comes out right and the tenth is noise.
const CommandSize = int(unsafe.Sizeof(Command{}))

// Op answers which command this is.
func (c Command) Op() Op {
	return Op(c[0])
}

// String describes the command for somebody reading a stream.
//
// It answers for every op, including one this package does not define, and that
// is deliberate: it is read when something is already wrong -- a trace, a failing
// test, a stream that draws the wrong thing -- and a description that stops the
// program on the one command that does not belong takes the diagnosis with it.
// A bare number is a poor answer and it is better than none.
//
// Each op prints its arguments rather than only its name. Which command went
// wrong is half of what a reader needs; what it said is the other half.
func (c Command) String() string {
	switch op := c.Op(); op {
	case OpNop:
		return "nop"
	case OpLine:
		from, to := DecodeLine(c)
		return fmt.Sprintf("line(%v, %v)", from, to)
	case OpGap:
		from, to := DecodeGap(c)
		return fmt.Sprintf("gap(%v, %v)", from, to)
	case OpQuad:
		from, ctrl, to := DecodeQuad(c)
		return fmt.Sprintf("quad(%v, %v, %v)", from, ctrl, to)
	case OpCubic:
		from, ctrl0, ctrl1, to := DecodeCubic(c)
		return fmt.Sprintf("cubic(%v, %v, %v, %v)", from, ctrl0, ctrl1, to)
	case OpFillColor:
		return fmt.Sprintf("fillcolor %#.8x", c[1])
	case OpLineWidth:
		return fmt.Sprintf("linewidth %v", math.Float32frombits(c[1]))
	case OpTransform:
		return fmt.Sprintf("transform (%v)", affine(c))
	case OpBeginClip:
		return fmt.Sprintf("beginclip (%v)", rectangle(c))
	case OpEndClip:
		return fmt.Sprintf("endclip (%v)", rectangle(c))
	case OpFillImage:
		return fmt.Sprintf("fillimage %d at %v", c[1], unpackOffset(c[2]))
	case OpSetFillMode:
		return fmt.Sprintf("setfillmode %s", fillModeName(FillMode(c[1])))
	default:
		return fmt.Sprintf("op %d", uint32(op))
	}
}

// Line is a straight segment of a path.
func Line(start, end f32.Point) Command {
	cmd := Command{0: uint32(OpLine)}
	putPoint(&cmd, 0, start)
	putPoint(&cmd, 1, end)
	return cmd
}

// Gap is the jump a path makes when a contour was left open.
//
// It carries the same two points as a line and is a separate op because the two
// readers of the stream want opposite things from it: a fill closes the contour
// across it, since winding is only defined for a closed one, and a stroke drops
// it, because nothing was drawn there. Encoded as a line it would be right for
// the fill and a stroke straight back to where the contour began.
func Gap(start, end f32.Point) Command {
	cmd := Command{0: uint32(OpGap)}
	putPoint(&cmd, 0, start)
	putPoint(&cmd, 1, end)
	return cmd
}

// Quad is a quadratic segment: the two ends and the point that bends it.
func Quad(start, ctrl, end f32.Point) Command {
	cmd := Command{0: uint32(OpQuad)}
	putPoint(&cmd, 0, start)
	putPoint(&cmd, 1, ctrl)
	putPoint(&cmd, 2, end)
	return cmd
}

// Cubic is a cubic segment: the two ends and a control point for each of them.
//
// The controls are stored in the order the pen passes them, so the one
// belonging to the start comes first. Exchanged, both ends stay where they are
// and the curve between them bulges the wrong way -- a shape that is wrong
// everywhere except at the two points anybody would check.
func Cubic(start, ctrl0, ctrl1, end f32.Point) Command {
	cmd := Command{0: uint32(OpCubic)}
	putPoint(&cmd, 0, start)
	putPoint(&cmd, 1, ctrl0)
	putPoint(&cmd, 2, ctrl1)
	putPoint(&cmd, 3, end)
	return cmd
}

// DecodeLine reads back the ends of a line.
func DecodeLine(cmd Command) (from, to f32.Point) {
	mustBe(cmd, OpLine)
	return point(cmd, 0), point(cmd, 1)
}

// DecodeGap reads back the ends of a gap.
func DecodeGap(cmd Command) (from, to f32.Point) {
	mustBe(cmd, OpGap)
	return point(cmd, 0), point(cmd, 1)
}

// DecodeQuad reads back the points of a quadratic segment.
func DecodeQuad(cmd Command) (from, ctrl, to f32.Point) {
	mustBe(cmd, OpQuad)
	return point(cmd, 0), point(cmd, 1), point(cmd, 2)
}

// DecodeCubic reads back the points of a cubic segment.
func DecodeCubic(cmd Command) (from, ctrl0, ctrl1, to f32.Point) {
	mustBe(cmd, OpCubic)
	return point(cmd, 0), point(cmd, 1), point(cmd, 2), point(cmd, 3)
}

// Transform is the matrix the commands after it are drawn through.
//
// This is the one command whose words are written out here instead of through a
// helper, because it is the one where the order changes: the matrix is handed
// over a row at a time and stored a column at a time, so the two shears cross.
// Stored in the order they were read, an upright drawing is unchanged and every
// rotation comes out mirrored. The function that reads them back is directly
// below, so that the crossing is visible in both directions at once.
func Transform(m f32.Affine2D) Command {
	sx, hx, ox, hy, sy, oy := m.Elems()
	return Command{
		0: uint32(OpTransform),
		1: math.Float32bits(sx),
		2: math.Float32bits(hy),
		3: math.Float32bits(hx),
		4: math.Float32bits(sy),
		5: math.Float32bits(ox),
		6: math.Float32bits(oy),
	}
}

// affine turns the stored columns back into the rows the constructor takes.
func affine(cmd Command) f32.Affine2D {
	return f32.NewAffine2D(
		math.Float32frombits(cmd[1]), math.Float32frombits(cmd[3]), math.Float32frombits(cmd[5]),
		math.Float32frombits(cmd[2]), math.Float32frombits(cmd[4]), math.Float32frombits(cmd[6]),
	)
}

// SetLineWidth is the width strokes after it are drawn at.
func SetLineWidth(width float32) Command {
	return Command{
		0: uint32(OpLineWidth),
		1: math.Float32bits(width),
	}
}

// BeginClip opens a clip whose contents are held to bbox.
func BeginClip(bbox f32.Rectangle) Command {
	cmd := Command{0: uint32(OpBeginClip)}
	putRectangle(&cmd, bbox)
	return cmd
}

// EndClip closes a clip.
//
// It repeats the bounds [BeginClip] was given, so that a reader walking the
// stream knows what is being closed without keeping a stack of its own.
func EndClip(bbox f32.Rectangle) Command {
	cmd := Command{0: uint32(OpEndClip)}
	putRectangle(&cmd, bbox)
	return cmd
}

// FillColor is the colour the shapes after it are filled with.
//
// The four channels are packed into one word, red in the top byte and alpha in
// the bottom.
func FillColor(col color.RGBA) Command {
	return Command{
		0: uint32(OpFillColor),
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	}
}

// FillImage is the image the shapes after it are filled with, and where its own
// origin sits.
func FillImage(index int, offset image.Point) Command {
	return Command{
		0: uint32(OpFillImage),
		1: uint32(index),
		2: packOffset(offset),
	}
}

// SetFillMode is how the area a path encloses is filled from here on.
func SetFillMode(mode FillMode) Command {
	return Command{
		0: uint32(OpSetFillMode),
		1: uint32(mode),
	}
}

// point reads the nth point of a command: two words, X then Y, after the op.
func point(cmd Command, n int) f32.Point {
	return f32.Pt(math.Float32frombits(cmd[1+2*n]), math.Float32frombits(cmd[1+2*n+1]))
}

// putPoint writes the nth point of a command.
//
// This pair is where the layout of a point lives, and it is a pair so that it
// is one place. Spelled out wherever a point is stored, the index is a fresh
// chance at each of them to write X into the word Y is read from -- a drawing
// reflected about its own diagonal, from code that compiles and from an edit
// that looks like a rename.
func putPoint(cmd *Command, n int, p f32.Point) {
	cmd[1+2*n] = math.Float32bits(p.X)
	cmd[1+2*n+1] = math.Float32bits(p.Y)
}

// rectangle reads the two corners a clip command carries.
func rectangle(cmd Command) f32.Rectangle {
	return f32.Rectangle{Min: point(cmd, 0), Max: point(cmd, 1)}
}

// putRectangle writes the near corner and then the far one.
func putRectangle(cmd *Command, r f32.Rectangle) {
	putPoint(cmd, 0, r.Min)
	putPoint(cmd, 1, r.Max)
}

// packOffset puts both halves of an image offset in one word, x in the low
// half.
//
// The trip through the unsigned type is what keeps a negative offset negative,
// and a negative one is ordinary: an image drawn from above or to the left of
// the origin has one. Widened as a signed value instead, it fills the high half
// with its sign bits -- and the high half is the other axis.
func packOffset(p image.Point) uint32 {
	return uint32(uint16(int16(p.X))) | uint32(uint16(int16(p.Y)))<<16
}

// unpackOffset reads back what packOffset wrote.
func unpackOffset(word uint32) image.Point {
	return image.Pt(int(int16(uint16(word))), int(int16(uint16(word>>16))))
}

// fillModeName names a mode for a person, and falls back to the number when it
// is neither of the two.
func fillModeName(mode FillMode) string {
	switch mode {
	case FillModeNonzero:
		return "nonzero"
	case FillModeStroke:
		return "stroke"
	}
	return fmt.Sprintf("%d", uint32(mode))
}

// mustBe stops a decoder that was handed a command of another op.
//
// Every word of every command is a float, so a quad read as a line answers two
// of its three points and reports nothing wrong: the caller gets points that
// are real numbers, in range, and from the wrong segment. There is no reading
// of that better than stopping. The caller switched on the op and then called
// the decoder of a different one, and every shape it draws from here is wrong.
func mustBe(cmd Command, op Op) {
	if got := cmd.Op(); got != op {
		panic(fmt.Sprintf("scene: a command of op %d was decoded as op %d", uint32(got), uint32(op)))
	}
}
