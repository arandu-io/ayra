package paint

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// ColorOp sets the brush to a single colour.
//
// It is what nearly everything is drawn with, and the one brush a caller can
// hold without deciding anything: a colour with no alpha marks nothing, so a
// variant that has no fill and a row that is not the chosen one are drawn by
// the same code as the ones that do, rather than by an if.
type ColorOp struct {
	// Color is the colour, in sRGB, with alpha held apart from the channels.
	Color color.NRGBA
}

// Add sets the brush to this colour, for every fill until a brush is set again.
func (c ColorOp) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypeColorLen)
	data[0] = byte(ops.TypeColor)
	data[1] = c.Color.R
	data[2] = c.Color.G
	data[3] = c.Color.B
	data[4] = c.Color.A
}

// LinearGradientOp sets the brush to a colour that changes along a line.
//
// A pixel takes Color1 where it projects onto Stop1, Color2 where it projects
// onto Stop2, and a mix of the two in between. Past either end it is that
// end's colour, so the brush covers the plane however far the fill reaches and
// the stops describe only where the change happens. A caller sizes the change
// to the shape it is about to fill and never has to put a stop far enough out
// to catch the corners.
//
// Nothing perpendicular to the line changes, which is what makes this a linear
// gradient and not a field: the colour is a function of one distance.
type LinearGradientOp struct {
	// Stop1 is where Color1 is reached.
	Stop1  f32.Point
	Color1 color.NRGBA
	// Stop2 is where Color2 is reached.
	Stop2  f32.Point
	Color2 color.NRGBA
}

// Add sets the brush to this gradient.
func (c LinearGradientOp) Add(o *op.Ops) {
	if c.Stop1 == c.Stop2 {
		// A gradient that ends where it begins has no direction and no length,
		// and how far along it a pixel lies is a division by nothing. Sent on
		// as it stands, that reaches the rasteriser as an infinity, and an
		// infinity is a colour only on the hardware that happens to clamp it
		// the way this one wants -- elsewhere it is a shape filled with
		// whatever the driver does with a value that is not a number, which is
		// a fault that appears on one machine in five and on none of the ones
		// it was written on.
		//
		// The answer is the end colour, and not by convention: every point of
		// a plane is at or past the end of a gradient with no length.
		//
		// It arrives by arithmetic rather than by intent -- a gradient sized
		// to a row that came out empty, an animation caught on the frame where
		// its two ends meet -- so it has to be answered rather than refused.
		ColorOp{Color: c.Color2}.Add(o)
		return
	}

	data := ops.Write(&o.Internal, ops.TypeLinearGradientLen)
	data[0] = byte(ops.TypeLinearGradient)

	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(c.Stop1.X))
	bo.PutUint32(data[5:], math.Float32bits(c.Stop1.Y))
	bo.PutUint32(data[9:], math.Float32bits(c.Stop2.X))
	bo.PutUint32(data[13:], math.Float32bits(c.Stop2.Y))

	data[17] = c.Color1.R
	data[18] = c.Color1.G
	data[19] = c.Color1.B
	data[20] = c.Color1.A
	data[21] = c.Color2.R
	data[22] = c.Color2.G
	data[23] = c.Color2.B
	data[24] = c.Color2.A
}

// ImageFilter is how a picture's pixels are read when it is drawn at a size
// other than its own.
type ImageFilter byte

const (
	// FilterLinear mixes the neighbouring pixels. It is what a photograph and
	// a drawn icon both want, and it is the zero value because a picture drawn
	// without a filter being chosen is almost always one of those.
	FilterLinear ImageFilter = iota
	// FilterNearest takes the nearest pixel whole. It is for pictures whose
	// pixels are the subject rather than the medium, where mixing would invent
	// values that were never in the source.
	FilterNearest
)

// ImageOp sets the brush to a picture.
//
// It is a value and not a handle to be closed: it is made from a picture,
// copied about freely, and dropped when nothing holds it. What it costs is
// paid once, when it is made, and that is the reason to keep one rather than
// build a new one per frame.
type ImageOp struct {
	// Filter is how the pixels are read when the picture is drawn at a size
	// other than its own.
	Filter ImageFilter

	// uniform says the picture was one colour and is carried as one. A single
	// colour needs no texture and no sampler, and fetching one to return a
	// value already known would be the most expensive way to do the cheapest
	// thing here.
	uniform bool
	color   color.NRGBA

	// src holds the pixels in the layout the rasteriser uploads directly. It
	// is nil for a uniform brush and for the zero value, and either of those
	// is a brush that sets nothing.
	src *image.RGBA

	// handle identifies these pixels to the texture cache.
	//
	// It is a pointer allocated once per brush, not anything derived from the
	// contents. Two brushes made from one picture are therefore two entries,
	// which wastes an upload; a key derived from the pixels would make two
	// pictures share an entry whenever they happened to agree, which shows the
	// wrong picture. Wasting an upload is the cheaper mistake.
	handle any
}

// NewImageOp makes a brush from a picture.
//
// The picture is treated as settled: the pixels may be copied here and the
// result of uploading them kept for as long as the brush lives, so writing to
// the picture afterwards changes nothing on screen. A picture that does change
// is drawn by making a new brush from it, which is also how the old upload is
// let go.
//
// A picture already in the layout the rasteriser wants is kept as it is and
// nothing is copied. Anything else is converted once, here, rather than per
// frame.
func NewImageOp(src image.Image) ImageOp {
	switch src := src.(type) {
	case *image.Uniform:
		// One colour over an infinite area is a colour. Carrying it as a
		// picture would be a texture, a sampler and a fetch per pixel to
		// answer what a brush can hold in four bytes.
		return ImageOp{
			uniform: true,
			color:   color.NRGBAModel.Convert(src.C).(color.NRGBA),
		}
	case *image.RGBA:
		return ImageOp{
			src:    src,
			handle: new(int),
		}
	}

	dst := image.NewRGBA(image.Rectangle{Max: src.Bounds().Size()})
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return ImageOp{
		src:    dst,
		handle: new(int),
	}
}

// Size is how many pixels the picture has, which is not how big it is drawn.
//
// A brush that holds no picture -- the zero value, a single colour, a picture
// with nothing in it -- answers nought in both directions rather than
// refusing. Every one of those reaches a caller from somewhere ordinary: a
// field nobody has set yet, a decode that produced nothing, a thumbnail that
// has not arrived.
func (i ImageOp) Size() image.Point {
	if i.src == nil {
		return image.Point{}
	}
	return i.src.Bounds().Size()
}

// Add sets the brush to this picture.
//
// A picture with no pixels sets nothing at all, and that is deliberate: the
// brush already in force stays in force, and the fill that follows draws what
// it would have drawn. The alternative is a texture of no size reaching the
// rasteriser, which is not a visible fault but a refused frame on whichever
// driver minds.
func (i ImageOp) Add(o *op.Ops) {
	if i.uniform {
		ColorOp{Color: i.color}.Add(o)
		return
	}
	if i.src == nil || i.src.Bounds().Empty() {
		return
	}
	data := ops.Write2(&o.Internal, ops.TypeImageLen, i.src, i.handle)
	data[0] = byte(ops.TypeImage)
	data[1] = byte(i.Filter)
}

// PaintOp fills the current clip with the current brush.
//
// It has no fields, and that is the design rather than an omission: where it
// paints is the clip, what it paints with is the brush, and where that lands is
// the transformation. An area of its own would be a second way to say the first
// of those, and the two would disagree.
type PaintOp struct{}

// Add fills the current clip with the current brush.
func (d PaintOp) Add(o *op.Ops) {
	data := ops.Write(&o.Internal, ops.TypePaintLen)
	data[0] = byte(ops.TypePaint)
}

// Fill paints a plane with no edges of its own in the given colour.
//
// The clip is what gives it any, so it is called with one already pushed. Use
// [FillShape] unless several fills share the shape, which is the case this
// exists for: the shape is pushed once and the fills follow.
func Fill(ops *op.Ops, c color.NRGBA) {
	ColorOp{Color: c}.Add(ops)
	PaintOp{}.Add(ops)
}

// FillShape fills one shape with one colour, which is most of what is drawn.
//
// The shape is pushed and popped around the fill, so what follows is clipped by
// whatever was in force before. Forgetting that pop is the fault this spares a
// caller, and it is not a visible one: everything drawn after it is silently
// cut to a shape that has nothing to do with it.
func FillShape(ops *op.Ops, c color.NRGBA, shape clip.Op) {
	defer shape.Push(ops).Pop()
	Fill(ops, c)
}
