package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// Video is the state half of a live video: the texture made from the newest
// picture, kept so that a picture is uploaded to the GPU once and not once per
// frame.
//
// It holds no connection and no decoder. The picture arrives in the props each
// frame, from whatever owns the stream, and what a person sees is drawn from
// that alone -- which is what lets a screen be drawn in a test with no network
// and no hardware, and what keeps this package from carrying a codec.
type Video struct {
	brush paint.ImageOp
	seq   uint64
	img   *image.RGBA
}

// VideoArea tells an overlay where the picture was drawn.
type VideoArea struct {
	// Picture is the rectangle the picture covers, in pixels of the area the
	// widget was given. An overlay draws inside it.
	Picture image.Rectangle
	// Source is the picture's own size in pixels, which is the coordinate
	// space the node's boxes are expressed in.
	Source image.Point
}

// Scale is how many drawn pixels one pixel of the source takes.
func (a VideoArea) Scale() float32 {
	if a.Source.X <= 0 {
		return 0
	}
	return float32(a.Picture.Dx()) / float32(a.Source.X)
}

// Rect maps a rectangle in the source's pixels onto the drawn picture.
func (a VideoArea) Rect(source image.Rectangle) image.Rectangle {
	s := a.Scale()
	return image.Rectangle{
		Min: a.Picture.Min.Add(image.Pt(int(float32(source.Min.X)*s+0.5), int(float32(source.Min.Y)*s+0.5))),
		Max: a.Picture.Min.Add(image.Pt(int(float32(source.Max.X)*s+0.5), int(float32(source.Max.Y)*s+0.5))),
	}
}

// VideoProps is a live video.
//
// Every field is an answer about a stream this control cannot see: the picture
// to draw, and a sentence about the state it is in. The control draws the
// picture as large as the room allows without cropping it, puts the sentence
// over it, and calls Overlay last so that the application can draw what belongs
// on the picture -- the boxes of the plates, a timestamp -- in the picture's own
// coordinates.
type VideoProps struct {
	// ComponentProps is what the caller changes on this one instance. A video
	// publishes two parts: "root", whose background is the ground the picture
	// is letterboxed on and whose radius rounds it, and "caption", the sentence
	// over the picture.
	ComponentProps
	// Frame is the picture to draw, and nil draws none. The control reads it
	// during layout and the engine reads it again when the frame is rasterised,
	// so it must not be written to until the next frame; a player's frame is
	// held for exactly that long.
	Frame *image.RGBA
	// Seq identifies the picture. The texture is made again when it changes and
	// reused when it does not, so a stream that has not produced a new picture
	// costs no upload. A caller with no counter of its own can leave it zero,
	// and the picture is then compared by identity.
	Seq uint64
	// Size is the shape to hold while there is no picture, so that the space a
	// camera takes does not jump when its first picture arrives. Zero is 16 by
	// 9.
	Size image.Point
	// Caption is the sentence the state needs: that the camera has no signal,
	// that the video is being prepared. With a picture it is drawn over a dimmed
	// copy of it, because a still picture of a camera that is no longer sending
	// must not pass for a live one.
	Caption string
	// Label names the video for a screen reader. Empty announces nothing.
	Label string
	// Overlay draws over the picture, last, inside the area it was given.
	Overlay func(c ayra.Context, area VideoArea)
	// Disabled draws the video dimmed.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p VideoProps) PartNames() []string { return []string{"root", "caption"} }

// liveAspect is the shape of a camera that has not said otherwise.
var liveAspect = image.Pt(16, 9)

// Layout draws the video and returns the room it took.
func (p VideoProps) Layout(c ayra.Context, state *Video) ayra.Dimensions {
	source := p.Size
	if p.Frame != nil && !p.Frame.Bounds().Empty() {
		source = p.Frame.Bounds().Size()
	}
	if source.X <= 0 || source.Y <= 0 {
		source = liveAspect
	}

	size := fitAspect(c.Constraints.Max, source)
	if size.X <= 0 || size.Y <= 0 {
		return ayra.Dimensions{}
	}

	if p.Frame != nil && !p.Frame.Bounds().Empty() {
		if state.img != p.Frame || (p.Seq != 0 && state.seq != p.Seq) {
			state.brush = paint.NewImageOp(p.Frame)
			state.img, state.seq = p.Frame, p.Seq
		}
	} else {
		state.brush, state.img, state.seq = paint.ImageOp{}, nil, 0
	}

	closeNode := announce(c, size, Announcement{Label: p.Label, Description: p.Caption, Class: Plain, Disabled: p.Disabled})
	defer closeNode()

	// The ground is black whatever the scheme: a video is not part of the
	// palette, and a letterbox that followed the theme would flash white around
	// a dark picture.
	root := c.Theme.Resolve("video", "root", p.VariantName("default"), theme.Style{
		Background: theme.Colour(color.NRGBA{A: 0xFF}),
		Radius:     theme.Pt(c.Theme.Metrics.Radius),
	}, p.ComponentProps)
	radius := c.Dp(unit.Dp(root.Radius.Points()))
	area := image.Rectangle{Max: size}
	defer clip.UniformRRect(area, radius).Push(c.Ops).Pop()

	paint.Fill(c.Ops, root.Background.NRGBA())

	if state.brush.Size() != (image.Point{}) {
		func() {
			scale := float32(size.X) / float32(source.X)
			defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale))).Push(c.Ops).Pop()
			defer clip.Rect{Max: source}.Push(c.Ops).Pop()
			state.brush.Add(c.Ops)
			paint.PaintOp{}.Add(c.Ops)
		}()
	}

	dim := p.Caption != "" && state.brush.Size() != image.Point{} || p.Disabled
	if dim {
		paint.Fill(c.Ops, color.NRGBA{A: 0x99})
	}

	if p.Caption != "" {
		p.caption(c, size)
	}

	if p.Overlay != nil {
		inner := c
		inner.Constraints.Min, inner.Constraints.Max = size, size
		p.Overlay(inner, VideoArea{Picture: area, Source: source})
	}
	return ayra.Dimensions{Size: size}
}

// caption centres the sentence on the picture.
func (p VideoProps) caption(c ayra.Context, size image.Point) {
	style := c.Theme.Resolve("video", "caption", p.VariantName("default"), theme.Style{
		Foreground: theme.Colour(color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF}),
	}, p.ComponentProps)
	inset := c.Dp(unit.Dp(12))
	inner := c
	inner.Constraints.Min = image.Point{}
	inner.Constraints.Max = image.Pt(max(size.X-2*inset, 1), size.Y)

	measure := op.Record(c.Ops)
	dims := drawText(inner, p.Caption, unit.Sp(c.Theme.Type.Small), style.Foreground.NRGBA(), 3, text.Middle, plain())
	drawn := measure.Stop()

	at := image.Pt((size.X-dims.Size.X)/2, (size.Y-dims.Size.Y)/2)
	defer op.Offset(at).Push(c.Ops).Pop()
	drawn.Add(c.Ops)
}

// fitAspect is the largest size of the source's shape that fits in the room,
// without cropping.
//
// An unbounded dimension takes the other: a video in a column is as wide as the
// column, and one in a row of unbounded height is as tall as its width implies.
func fitAspect(room, source image.Point) image.Point {
	const unbounded = 1 << 24
	w, h := room.X, room.Y
	switch {
	case w <= 0 && h <= 0:
		return image.Point{}
	case w <= 0 || w >= unbounded:
		w = h * source.X / source.Y
	case h <= 0 || h >= unbounded:
		h = w * source.Y / source.X
	}
	if w*source.Y > h*source.X {
		w = h * source.X / source.Y
	} else {
		h = w * source.Y / source.X
	}
	return image.Pt(w, h)
}
