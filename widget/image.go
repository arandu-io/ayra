package widget

import (
	"context"
	"image"
	"image/color"
	_ "image/gif"  // the formats a photograph arrives in
	_ "image/jpeg" // are registered here, so DecodeImage reads them,
	_ "image/png"  // and WebP registers itself the same way
	"io"
	"sync"

	_ "golang.org/x/image/webp"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// Fit is how a picture is made to fit the box it is given.
type Fit uint8

const (
	// Cover fills the box and crops what overflows, which is what a photograph of
	// a person or a place wants. Zero, because it is what most pictures are.
	Cover Fit = iota
	// Contain shows the whole picture inside the box and leaves the rest empty,
	// which is what a logo or a plate wants.
	Contain
	// Fill stretches it to the box, whatever that does to the proportions.
	Fill
)

// DecodeImage reads a picture in any format a photograph comes in: PNG, JPEG, GIF
// and WebP. It is what an [ImageSource] calls on the bytes it fetched.
func DecodeImage(r io.Reader) (image.Image, error) {
	picture, _, err := image.Decode(r)
	return picture, err
}

// ImageSource is what an application implements to produce a picture: from a
// URL, from a file, from a cache. It runs off the frame loop, in its own
// goroutine, and the context is cancelled when the picture is no longer wanted.
// The network, and what authorizes it, belong to the implementation.
type ImageSource interface {
	Load(ctx context.Context) (image.Image, error)
}

// ImageSourceFunc is a function as an [ImageSource].
type ImageSourceFunc func(ctx context.Context) (image.Image, error)

// Load calls the function.
func (f ImageSourceFunc) Load(ctx context.Context) (image.Image, error) { return f(ctx) }

// Image is the state of a picture: what was loaded, or why it was not. The
// caller holds one per picture, and a picture that is drawn in a list is one per
// row: the state is what keeps it from being fetched again on every frame.
type Image struct {
	mu      sync.Mutex
	picture image.Image
	op      paint.ImageOp
	err     error
	started bool
	cancel  context.CancelFunc
}

// Set gives the picture directly, for one the application already holds.
func (i *Image) Set(picture image.Image) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.picture, i.err, i.started = picture, nil, true
	i.op = paint.NewImageOp(picture)
}

// Picture is the picture, and nil while it is loading or when it failed.
func (i *Image) Picture() image.Image {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.picture
}

// ensure starts the load the first time a picture is drawn with a source, and
// never again: the state is what keeps a picture from being fetched on every
// frame.
func (i *Image) ensure(c ayra.Context, source ImageSource) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.started || source == nil {
		return
	}
	i.started = true
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel
	redraw := c.Redraw
	go func() {
		picture, err := source.Load(ctx)
		i.mu.Lock()
		defer i.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			i.err = err
		} else if picture != nil {
			i.picture, i.op = picture, paint.NewImageOp(picture)
		}
		redraw()
	}()
}

// Err is why the picture could not be loaded, and nil when it was or is on its
// way.
func (i *Image) Err() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.err
}

// Release cancels a load still in flight, for a picture scrolled out of a list.
func (i *Image) Release() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.cancel != nil {
		i.cancel()
	}
}

// ImageProps is a picture -- the counterpart of Kyse's ResponsiveImage, for a
// window that is handed the picture it should draw and does not choose a source
// by the screen it is on.
//
// It draws a skeleton while the picture loads, the picture when it arrives, and
// the alternative text on a muted block when it could not be loaded: a blank
// space says there is nothing, and each of these says what is true.
type ImageProps struct {
	// ComponentProps is what the caller changes on this one instance. A picture
	// publishes one part, "root": its background is what shows behind it, and
	// its radius rounds it.
	ComponentProps
	// Source produces the picture. Nil draws the placeholder only.
	Source ImageSource
	// Width and Height are the box, in points. Zero takes the room it is given
	// across, and for the height the picture's own proportions -- or a wide
	// rectangle while it is not known.
	Width, Height unit.Dp
	// Fit is how the picture fills the box. Zero covers it.
	Fit Fit
	// Alt says what the picture shows: to a screen reader, and on the block
	// that stands in when it fails. A picture that carries information has one.
	Alt string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ImageProps) PartNames() []string { return []string{"root"} }

// Layout draws the picture and returns the room it took.
func (p ImageProps) Layout(c ayra.Context, state *Image) ayra.Dimensions {
	state.ensure(c, p.Source)
	state.mu.Lock()
	picture, imageOp, failed := state.picture, state.op, state.err != nil
	state.mu.Unlock()

	t := c.Theme
	style := t.Resolve("image", "root", p.VariantName("default"), theme.Style{
		Background: theme.Colour(t.Colours.Muted),
		Foreground: theme.Colour(t.Colours.MutedForeground),
		Radius:     theme.Pt(t.Metrics.Radius),
	}, p.ComponentProps)

	width := c.Constraints.Max.X
	if p.Width != 0 {
		width = c.Dp(p.Width)
	}
	height := width * 9 / 16
	if picture != nil {
		if b := picture.Bounds(); b.Dx() > 0 {
			height = width * b.Dy() / b.Dx()
		}
	}
	if p.Height != 0 {
		height = c.Dp(p.Height)
	}
	size := image.Pt(width, height)
	radius := c.Dp(unit.Dp(style.Radius.Points()))

	close := func() {}
	if p.Alt != "" {
		close = announce(c, size, Announcement{Label: p.Alt})
	}
	defer close()

	switch {
	case picture != nil:
		DrawImage(c, imageOp, size, p.Fit, radius)
	case failed:
		paintShape(c, size, style.Background.NRGBA(), color.NRGBA{}, radius, 0)
		if p.Alt != "" {
			inner := c
			inner.Constraints.Min, inner.Constraints.Max = image.Point{}, size
			offset := op.Offset(image.Pt(0, size.Y/2-c.Dp(unit.Dp(t.Type.Small)))).Push(c.Ops)
			inner.Constraints.Min.X = size.X
			drawText(inner, p.Alt, unit.Sp(t.Type.Small), style.Foreground.NRGBA(), 1, text.Middle, plain())
			offset.Pop()
		}
	default:
		paintShape(c, size, style.Background.NRGBA(), color.NRGBA{}, radius, 0)
	}
	return ayra.Dimensions{Size: size}
}

// DrawImage paints a picture into a box at the current origin, fitted as asked and
// clipped to a rounded rectangle, for a widget the product writes itself. It
// takes no room and announces nothing.
func DrawImage(c ayra.Context, picture paint.ImageOp, size image.Point, fit Fit, radius int) {
	source := picture.Size()
	if source.X == 0 || source.Y == 0 || size.X <= 0 || size.Y <= 0 {
		return
	}

	scaleX, scaleY := float32(size.X)/float32(source.X), float32(size.Y)/float32(source.Y)
	switch fit {
	case Cover:
		scaleX = max(scaleX, scaleY)
		scaleY = scaleX
	case Contain:
		scaleX = min(scaleX, scaleY)
		scaleY = scaleX
	}
	offset := f32.Pt((float32(size.X)-float32(source.X)*scaleX)/2, (float32(size.Y)-float32(source.Y)*scaleY)/2)

	defer clip.UniformRRect(image.Rectangle{Max: size}, radius).Push(c.Ops).Pop()
	defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scaleX, scaleY)).Offset(offset)).Push(c.Ops).Pop()
	picture.Add(c.Ops)
	paint.PaintOp{}.Add(c.Ops)
}
