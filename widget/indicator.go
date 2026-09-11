package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// LabelProps is the name of a field, drawn above it.
//
// It is not [TextProps] with a size chosen at the call site. A label is a role
// -- the thing that names a control -- and giving it its own type is what makes
// every form in a product use the same size, weight and colour for it without
// anybody deciding again.
type LabelProps struct {
	// Text is the name.
	Text string
	// Required marks the field as one that cannot be left blank.
	Required bool
	// Disabled draws it as belonging to a control that is unavailable.
	Disabled bool
}

// Layout draws the label and returns the room it took.
func (p LabelProps) Layout(c ayra.Context) ayra.Dimensions {
	ink := c.Theme.Colours.MutedForeground
	if p.Disabled {
		ink = fade(ink)
	}

	content := p.Text
	if p.Required {
		// The mark is part of the string rather than a second draw, so it
		// wraps with the word it belongs to instead of stranding itself on the
		// next line.
		content += " *"
	}
	return drawText(c, content, unit.Sp(c.Theme.Type.Small), ink, 1, text.Start, plain())
}

// ProgressProps is how far along something is.
type ProgressProps struct {
	// Fraction is between 0 and 1. Anything outside is clamped, because a bar
	// drawn past its own track is a bar that has overwritten what is beside it.
	Fraction float32
	// Indeterminate draws a bar for work whose length is unknown, as a track
	// with no fill: something that pretends to know how far along it is lies
	// every time it reaches ninety percent and stops.
	Indeterminate bool
}

// Layout draws the bar across the width it was given.
func (p ProgressProps) Layout(c ayra.Context) ayra.Dimensions {
	height := c.Dp(unit.Dp(8))
	width := c.Constraints.Max.X

	track := image.Rectangle{Max: image.Pt(width, height)}
	paint.FillShape(c.Ops, c.Theme.Colours.Muted, clip.UniformRRect(track, height/2).Op(c.Ops))

	if !p.Indeterminate {
		fraction := min(max(p.Fraction, 0), 1)
		filled := int(float32(width) * fraction)
		if filled > 0 {
			bar := image.Rect(0, 0, max(filled, height), height)
			paint.FillShape(c.Ops, c.Theme.Colours.Primary, clip.UniformRRect(bar, height/2).Op(c.Ops))
		}
	}

	return ayra.Dimensions{Size: track.Max}
}

// AvatarProps is the mark that stands for a person.
type AvatarProps struct {
	// Initials are what is drawn. One or two letters: three is a monogram
	// nobody reads at this size.
	Initials string
	// Size is the diameter. Zero takes the one a row of them is built from.
	Size unit.Dp
}

// Layout draws the avatar and returns the room it took.
func (p AvatarProps) Layout(c ayra.Context) ayra.Dimensions {
	diameter := p.Size
	if diameter == 0 {
		diameter = 36
	}
	side := c.Dp(diameter)

	circle := image.Rectangle{Max: image.Pt(side, side)}
	paint.FillShape(c.Ops, c.Theme.Colours.Muted, clip.UniformRRect(circle, side/2).Op(c.Ops))

	c.Constraints = layout.Exact(circle.Max)
	layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Point{}
		size := unit.Sp(float32(diameter) * 0.4)
		return drawText(inner, p.Initials, size, c.Theme.Colours.MutedForeground, 1, text.Middle, semibold())
	})

	return ayra.Dimensions{Size: circle.Max}
}

// SkeletonProps is the shape of something that has not arrived.
//
// It is drawn rather than left blank because the two say different things: a
// blank region says there is nothing, and this says there is something and it
// is coming.
type SkeletonProps struct {
	// Height is how tall the block is. Zero takes a line of text.
	Height unit.Dp
	// Width is how wide. Zero fills the room there is, which is right for a
	// paragraph and wrong for a name.
	Width unit.Dp
	// Round draws it as a circle, for an avatar that has not loaded.
	Round bool
}

// Layout draws the placeholder.
func (p SkeletonProps) Layout(c ayra.Context) ayra.Dimensions {
	height := c.Dp(p.Height)
	if p.Height == 0 {
		height = c.Dp(unit.Dp(16))
	}
	width := c.Constraints.Max.X
	if p.Width != 0 {
		width = c.Dp(p.Width)
	}

	radius := controlRadius(c)
	if p.Round {
		radius = min(width, height) / 2
	}

	block := image.Rectangle{Max: image.Pt(width, height)}
	paint.FillShape(c.Ops, c.Theme.Colours.Muted, clip.UniformRRect(block, radius).Op(c.Ops))
	return ayra.Dimensions{Size: block.Max}
}

// KbdProps is a key somebody is told to press.
type KbdProps struct {
	// Keys is what is written on it: "K", or "Ctrl", or a combination the
	// caller has already joined.
	Keys string
}

// Layout draws the key and returns the room it took.
func (p KbdProps) Layout(c ayra.Context) ayra.Dimensions {
	c.Constraints.Min = image.Point{}

	return surface(c, c.Theme.Colours.Muted, c.Theme.Colours.Border, c.Dp(unit.Dp(4)), func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), p.Keys, unit.Sp(c.Theme.Type.Mono), c.Theme.Colours.MutedForeground, 1, text.Middle, mono())
		})
	})
}

// StatusProps is a state, drawn as a dot and a word.
//
// The dot is what makes it readable at a glance in a list, and the word is what
// makes it readable at all: a row of coloured dots with no words is a legend
// somebody has to have memorised.
type StatusProps struct {
	// Label is the state, in words.
	Label string
	// Tone is how it should read. Zero is neutral.
	Tone Tone
}

// Layout draws the dot and the word.
func (p StatusProps) Layout(c ayra.Context) ayra.Dimensions {
	c.Constraints.Min = image.Point{}
	ink := p.Tone.ink(c.Theme)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			side := c.Dp(unit.Dp(8))
			dot := image.Rectangle{Max: image.Pt(side, side)}
			paint.FillShape(gtx.Ops, ink, clip.UniformRRect(dot, side/2).Op(gtx.Ops))
			return layout.Dimensions{Size: dot.Max}
		}),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), p.Label, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.Foreground, 1, text.Start, plain())
		}),
	)
}
