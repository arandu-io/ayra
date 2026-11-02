package widget

import (
	"image"
	"image/color"
	"sync"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
)

// IconProps is an icon of the Phosphor set -- the counterpart of Kyse's icons,
// under the same names.
//
//	widget.IconProps{Icon: icons.Trash(), Label: "Delete this reading"}.Layout(c)
//
// It is drawn as a filled shape in the colour of the text around it, with the
// lighter half at a fifth of its strength, and takes the size of a line of text
// unless told another. An icon beside a word that already says what it means is
// decorative and says nothing to a screen reader; an icon alone is the label.
type IconProps struct {
	// ComponentProps is what the caller changes on this one instance. An icon
	// publishes one part, "root": its foreground is the colour, its font size
	// is the side, and its padding is the room around it.
	ComponentProps
	// Icon is what is drawn. The zero Icon draws nothing and takes no room.
	Icon icons.Icon
	// Size is the side of the square, in points. Zero takes the size of body
	// text, which is what an icon inside a line of it wants.
	Size unit.Dp
	// Label is what a screen reader says instead of the picture. Empty is
	// decorative: announcing "trash, Delete" reads the button twice.
	Label string
	// Tone is how much attention it asks for, as for text. Zero reads against
	// the surface.
	Tone Tone
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p IconProps) PartNames() []string { return []string{"root"} }

// Layout draws the icon and returns the room it took.
func (p IconProps) Layout(c ayra.Context) ayra.Dimensions {
	if p.Icon.IsZero() {
		return ayra.Dimensions{}
	}
	t := c.Theme

	side := float32(p.Size)
	if side == 0 {
		side = t.Type.Body
	}
	style := t.Resolve("icon", "root", p.VariantName(p.Tone.String()), theme.Style{
		Foreground: theme.Colour(p.Tone.ink(t)),
		Font:       theme.Font{Size: theme.Pt(side)},
	}, p.ComponentProps)

	pixels := c.Dp(unit.Dp(style.Font.Size.Or(theme.Pt(side)).Points()))
	return insetOf(style.Padding).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		close := func() {}
		if p.Label != "" {
			close = announce(inner, image.Pt(pixels, pixels), Announcement{Label: p.Label})
		}
		DrawIcon(inner, p.Icon, pixels, style.Foreground.NRGBA())
		close()
		// The text's baseline runs a little above the bottom of the square, as the
		// browser half sets an icon beside a word (vertical-align: -0.125em). A
		// baseline at the top edge, which is what Baseline: pixels said, hung the
		// whole icon below the line of the word it stands beside.
		return ayra.Dimensions{Size: image.Pt(pixels, pixels), Baseline: pixels / 8}
	})
}

// DrawIcon paints an icon on a square of this many pixels at the current origin,
// for a widget the product writes itself: the body in the ink, the shade at a
// fifth of it. It takes no room and announces nothing; the widget that calls it
// does both.
func DrawIcon(c ayra.Context, icon icons.Icon, pixels int, ink color.NRGBA) {
	if icon.IsZero() || pixels <= 0 {
		return
	}
	scale := float32(pixels) / icons.Size

	if icon.Shade() != "" {
		shade := ink
		shade.A /= 5
		fillPath(c.Ops, icon.Shade(), scale, shade)
	}
	fillPath(c.Ops, icon.Body(), scale, ink)
}

// parsed keeps the segments of every path drawn, by the path that was parsed. An
// icon is drawn on every frame and parsed once.
var parsed sync.Map

func segmentsOf(d string) []icons.Segment {
	if cached, found := parsed.Load(d); found {
		return cached.([]icons.Segment)
	}
	segments, err := icons.ParsePath(d)
	if err != nil {
		// A path the parser cannot read draws nothing, and the test that parses
		// all of them is what says which icon it was.
		segments = nil
	}
	parsed.Store(d, segments)
	return segments
}

func fillPath(ops *op.Ops, d string, scale float32, ink color.NRGBA) {
	segments := segmentsOf(d)
	if len(segments) == 0 || ink.A == 0 {
		return
	}

	point := func(p icons.Point) f32.Point { return f32.Pt(p.X*scale, p.Y*scale) }

	var path clip.Path
	path.Begin(ops)
	for _, s := range segments {
		switch s.Op {
		case icons.MoveTo:
			path.MoveTo(point(s.P[0]))
		case icons.LineTo:
			path.LineTo(point(s.P[0]))
		case icons.CubicTo:
			path.CubeTo(point(s.P[0]), point(s.P[1]), point(s.P[2]))
		case icons.Close:
			path.Close()
		}
	}
	paint.FillShape(ops, ink, clip.Outline{Path: path.End()}.Op())
}
