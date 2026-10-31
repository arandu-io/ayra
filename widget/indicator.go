package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// LabelProps is the name of a field, drawn above it.
//
// It is not [TextProps] with a size chosen at the call site. A label is a role
// -- the thing that names a control -- and giving it its own type is what makes
// every form in a product use the same size, weight and colour for it without
// anybody deciding again.
type LabelProps struct {
	// ComponentProps is what the caller changes on this one instance. A label publishes one part, "root".
	ComponentProps
	// Text is the name.
	Text string
	// Required marks the field as one that cannot be left blank.
	Required bool
	// Disabled draws it as belonging to a control that is unavailable.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p LabelProps) PartNames() []string { return []string{"root"} }

// Layout draws the label and returns the room it took.
func (p LabelProps) Layout(c ayra.Context) ayra.Dimensions {
	style := c.Theme.Resolve("label", "root", p.VariantName("default"), theme.Style{
		Foreground: theme.Colour(c.Theme.Colours.MutedForeground),
		Font:       theme.Font{Size: theme.Pt(c.Theme.Type.Small)},
	}, p.ComponentProps)
	if p.Disabled {
		style.Foreground = theme.Colour(fade(style.Foreground.NRGBA()))
	}

	content := p.Text
	if p.Required {
		// The mark is part of the string rather than a second draw, so it
		// wraps with the word it belongs to instead of stranding itself on the
		// next line.
		content += " *"
	}
	return insetOf(style.Padding).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return drawPart(c.With(gtx), content, style, unit.Sp(c.Theme.Type.Small), 1, text.Start, plain())
	})
}

// ProgressProps is how far along something is.
type ProgressProps struct {
	// ComponentProps is what the caller changes on this one instance. A progress bar publishes "root", the track, and "fill", the bar.
	ComponentProps
	// Fraction is between 0 and 1. Anything outside is clamped, because a bar
	// drawn past its own track is a bar that has overwritten what is beside it.
	Fraction float32
	// Indeterminate draws a bar for work whose length is unknown, as a track
	// with no fill: something that pretends to know how far along it is lies
	// every time it reaches ninety percent and stops.
	Indeterminate bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ProgressProps) PartNames() []string { return []string{"root", "fill"} }

// Layout draws the bar across the width it was given.
func (p ProgressProps) Layout(c ayra.Context) ayra.Dimensions {
	height := c.Dp(unit.Dp(8))
	width := c.Constraints.Max.X

	root := c.Theme.Resolve("progress", "root", p.VariantName("default"), theme.Style{
		Background: theme.Colour(c.Theme.Colours.Muted),
	}, p.ComponentProps)
	fill := c.Theme.Resolve("progress", "fill", p.VariantName("default"), theme.Style{
		Background: theme.Colour(c.Theme.Colours.Primary),
	}, p.ComponentProps)

	track := image.Rectangle{Max: image.Pt(width, height)}
	trackRadius, barRadius := height/2, height/2
	if root.Radius.IsSet() {
		trackRadius = c.Dp(unit.Dp(root.Radius.Points()))
	}
	if fill.Radius.IsSet() {
		barRadius = c.Dp(unit.Dp(fill.Radius.Points()))
	}
	paint.FillShape(c.Ops, root.Background.NRGBA(), clip.UniformRRect(track, trackRadius).Op(c.Ops))

	if !p.Indeterminate {
		fraction := min(max(p.Fraction, 0), 1)
		filled := int(float32(width) * fraction)
		if filled > 0 {
			bar := image.Rect(0, 0, max(filled, height), height)
			paint.FillShape(c.Ops, fill.Background.NRGBA(), clip.UniformRRect(bar, barRadius).Op(c.Ops))
		}
	}

	return ayra.Dimensions{Size: track.Max}
}

// AvatarProps is the mark that stands for a person.
type AvatarProps struct {
	// ComponentProps is what the caller changes on this one instance. An avatar publishes "root", the disc, and "initials".
	ComponentProps
	// Initials are what is drawn. One or two letters: three is a monogram
	// nobody reads at this size.
	Initials string
	// Size is the diameter. Zero takes the one a row of them is built from.
	Size unit.Dp
	// Photo is the person's picture, drawn in the circle instead of the
	// initials once it has loaded; the initials stand in until then, and when it
	// fails. It is a pointer to state the caller holds, because a picture has to
	// be remembered from one frame to the next and an avatar is drawn from a
	// value that remembers nothing.
	Photo *Image
	// Source produces the photo, the first time the avatar is drawn. Nil leaves
	// Photo to whoever fills it with [Image.Set].
	Source ImageSource
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p AvatarProps) PartNames() []string { return []string{"root", "initials"} }

// Layout draws the avatar and returns the room it took.
func (p AvatarProps) Layout(c ayra.Context) ayra.Dimensions {
	diameter := p.Size
	if diameter == 0 {
		diameter = 36
	}
	side := c.Dp(diameter)

	root := c.Theme.Resolve("avatar", "root", p.VariantName("default"), theme.Style{
		Background: theme.Colour(c.Theme.Colours.Muted),
	}, p.ComponentProps)
	initials := c.Theme.Resolve("avatar", "initials", p.VariantName("default"), theme.Style{
		Foreground: theme.Colour(c.Theme.Colours.MutedForeground),
		Font:       theme.Font{Size: theme.Pt(float32(diameter) * 0.4)},
	}, p.ComponentProps)

	circle := image.Rectangle{Max: image.Pt(side, side)}
	radius := side / 2
	if root.Radius.IsSet() {
		radius = c.Dp(unit.Dp(root.Radius.Points()))
	}
	paint.FillShape(c.Ops, root.Background.NRGBA(), clip.UniformRRect(circle, radius).Op(c.Ops))

	if p.Photo != nil {
		p.Photo.ensure(c, p.Source)
		p.Photo.mu.Lock()
		picture, photo := p.Photo.picture, p.Photo.op
		p.Photo.mu.Unlock()
		if picture != nil {
			DrawImage(c, photo, circle.Max, Cover, radius)
			return ayra.Dimensions{Size: circle.Max}
		}
	}

	c.Constraints = layout.Exact(circle.Max)
	layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Point{}
		return drawPart(inner, p.Initials, initials, unit.Sp(float32(diameter)*0.4), 1, text.Middle, semibold())
	})

	return ayra.Dimensions{Size: circle.Max}
}

// SkeletonProps is the shape of something that has not arrived.
//
// It is drawn rather than left blank because the two say different things: a
// blank region says there is nothing, and this says there is something and it
// is coming.
type SkeletonProps struct {
	// ComponentProps is what the caller changes on this one instance. A skeleton publishes one part, "root".
	ComponentProps
	// Height is how tall the block is. Zero takes a line of text.
	Height unit.Dp
	// Width is how wide. Zero fills the room there is, which is right for a
	// paragraph and wrong for a name.
	Width unit.Dp
	// Round draws it as a circle, for an avatar that has not loaded.
	Round bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SkeletonProps) PartNames() []string { return []string{"root"} }

// Layout draws the placeholder.
func (p SkeletonProps) Layout(c ayra.Context) ayra.Dimensions {
	style := c.Theme.Resolve("skeleton", "root", p.VariantName("default"), theme.Style{
		Background: theme.Colour(c.Theme.Colours.Muted),
	}, p.ComponentProps)
	height := c.Dp(p.Height)
	if p.Height == 0 {
		height = c.Dp(unit.Dp(16))
	}
	width := c.Constraints.Max.X
	if p.Width != 0 {
		width = c.Dp(p.Width)
	}

	radius := controlRadius(c)
	if style.Radius.IsSet() {
		radius = c.Dp(unit.Dp(style.Radius.Points()))
	}
	if p.Round {
		radius = min(width, height) / 2
	}

	block := image.Rectangle{Max: image.Pt(width, height)}
	paint.FillShape(c.Ops, style.Background.NRGBA(), clip.UniformRRect(block, radius).Op(c.Ops))
	return ayra.Dimensions{Size: block.Max}
}

// KbdProps is a key somebody is told to press.
type KbdProps struct {
	// ComponentProps is what the caller changes on this one instance. A key publishes one part, "root".
	ComponentProps
	// Keys is what is written on it: "K", or "Ctrl", or a combination the
	// caller has already joined.
	Keys string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p KbdProps) PartNames() []string { return []string{"root"} }

// Layout draws the key and returns the room it took.
func (p KbdProps) Layout(c ayra.Context) ayra.Dimensions {
	c.Constraints.Min = image.Point{}

	style := c.Theme.Resolve("kbd", "root", p.VariantName("default"), theme.Style{
		Background: theme.Colour(c.Theme.Colours.Muted),
		Foreground: theme.Colour(c.Theme.Colours.MutedForeground),
		Border:     theme.Colour(c.Theme.Colours.Border),
		Radius:     theme.Pt(4),
		Padding:    theme.InsetXY(6, 2),
		Font:       theme.Font{Size: theme.Pt(c.Theme.Type.Mono)},
	}, p.ComponentProps)

	ink := style
	ink.Padding = theme.Insets{}
	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return drawPart(c, p.Keys, ink, unit.Sp(c.Theme.Type.Mono), 1, text.Middle, mono())
	})
}

// StatusProps is a state, drawn as a dot and a word.
//
// The dot is what makes it readable at a glance in a list, and the word is what
// makes it readable at all: a row of coloured dots with no words is a legend
// somebody has to have memorised.
type StatusProps struct {
	// ComponentProps is what the caller changes on this one instance. A status publishes "root", "dot" and "label".
	ComponentProps
	// Label is the state, in words.
	Label string
	// Tone is how it should read. Zero is neutral.
	Tone Tone
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p StatusProps) PartNames() []string { return []string{"root", "dot", "label"} }

// Layout draws the dot and the word.
func (p StatusProps) Layout(c ayra.Context) ayra.Dimensions {
	c.Constraints.Min = image.Point{}
	variant := p.VariantName(p.Tone.String())

	root := c.Theme.Resolve("status", "root", variant, theme.Style{}, p.ComponentProps)
	dotStyle := c.Theme.Resolve("status", "dot", variant, theme.Style{
		Background: theme.Colour(p.Tone.ink(c.Theme)),
	}, p.ComponentProps)
	label := c.Theme.Resolve("status", "label", variant, theme.Style{
		Foreground: theme.Colour(c.Theme.Colours.Foreground),
		Font:       theme.Font{Size: theme.Pt(c.Theme.Type.Small)},
	}, p.ComponentProps)

	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				side := c.Dp(unit.Dp(8))
				radius := side / 2
				if dotStyle.Radius.IsSet() {
					radius = c.Dp(unit.Dp(dotStyle.Radius.Points()))
				}
				dot := image.Rectangle{Max: image.Pt(side, side)}
				paint.FillShape(gtx.Ops, dotStyle.Background.NRGBA(), clip.UniformRRect(dot, radius).Op(gtx.Ops))
				return layout.Dimensions{Size: dot.Max}
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawPart(c.With(gtx), p.Label, label, unit.Sp(c.Theme.Type.Small), 1, text.Start, plain())
			}),
		)
	})
}
