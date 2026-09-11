package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
)

// Side is which edge something comes in from.
type Side uint8

const (
	// Right is where a detail panel belongs, because that is where the reading
	// eye ends. Zero, for the same reason.
	Right Side = iota
	// Left is where navigation belongs.
	Left
	// Bottom is where a sheet belongs on a phone, within reach of a thumb.
	Bottom
)

// String names the side, for a diagnostic screen and a test failure.
func (s Side) String() string {
	switch s {
	case Left:
		return "left"
	case Bottom:
		return "bottom"
	}
	return "right"
}

// DrawerProps is a panel that slides in over the screen.
//
// It is a [DialogProps] anchored to an edge rather than centred, and the
// difference is what it is for: a dialog asks something and is answered, and a
// drawer holds a place somebody works in while the screen stays behind it.
type DrawerProps struct {
	// Side is the edge it comes from. Zero is the right.
	Side Side
	// Size is how wide it is, or how tall from the bottom. Zero takes a width
	// a form reads at.
	Size unit.Dp
	// Modal darkens what is behind and swallows presses on it.
	Modal bool
	// Dismissible lets escape and a press outside close it.
	Dismissible bool
}

// Layout draws the drawer over the screen and returns the room it took.
func (p DrawerProps) Layout(c ayra.Context, state *Dialog, content ayra.Widget) ayra.Dimensions {
	if !state.Showing() {
		return ayra.Dimensions{}
	}

	area := c.Constraints.Max
	defer clip.Rect{Max: area}.Push(c.Ops).Pop()

	if p.Modal {
		paint.FillShape(c.Ops, scrim(), clip.Rect{Max: area}.Op())
		state.outside.Add(c.Ops)
		registerTarget(c, &state.outside)
	}
	if p.Dismissible {
		DialogProps{Dismissible: true}.readDismissal(c, state)
	}

	size := p.Size
	if size == 0 {
		size = 380
	}

	panel := p.bounds(c, area, size)
	inner := c
	inner.Constraints = layout.Exact(panel.Size())

	measure := op.Record(c.Ops)
	dims := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, 0, func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(20).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return content(c.With(gtx))
		})
	})
	drawn := measure.Stop()

	offset := op.Offset(panel.Min).Push(c.Ops)
	clipped := clip.Rect(image.Rectangle{Max: panel.Size()}).Push(c.Ops)
	state.inside.Add(c.Ops)
	registerTarget(c, &state.inside)
	drawn.Add(c.Ops)
	clipped.Pop()
	offset.Pop()

	_ = dims
	return ayra.Dimensions{Size: area}
}

// bounds answers where the panel sits against its edge.
func (p DrawerProps) bounds(c ayra.Context, area image.Point, size unit.Dp) image.Rectangle {
	extent := c.Dp(size)

	switch p.Side {
	case Left:
		return image.Rect(0, 0, min(extent, area.X), area.Y)
	case Bottom:
		top := max(area.Y-extent, 0)
		return image.Rect(0, top, area.X, area.Y)
	}
	left := max(area.X-extent, 0)
	return image.Rect(left, 0, area.X, area.Y)
}

// SplitProps is a button with a second control beside it for the other things
// it could do.
//
// The main action is the one somebody presses without reading, and the rest are
// behind the second half. A menu with the common action inside it makes the
// common action the slowest one.
type SplitProps struct {
	// Label is the main action.
	Label string
	// Variant is what the pair is for. Zero is the ordinary one.
	Variant Variant
	// Size is how much room they take.
	Size Size
	// Disabled draws both as unavailable.
	Disabled bool
}

// Split is the state half: the press on each side.
type Split struct {
	main Button
	more Button
}

// Pressed reports that the main action was chosen.
func (s *Split) Pressed(c ayra.Context) bool { return s.main.Clicked(c) }

// Opened reports that the second half was pressed.
func (s *Split) Opened(c ayra.Context) bool { return s.more.Clicked(c) }

// Layout draws the pair as one object.
func (p SplitProps) Layout(c ayra.Context, state *Split) ayra.Dimensions {
	c.Constraints.Min = image.Point{}

	measure := op.Record(c.Ops)
	main := ButtonProps{Label: p.Label, Variant: p.Variant, Size: p.Size, Disabled: p.Disabled}.Layout(c, &state.main)
	mainDrawn := measure.Stop()

	measure = op.Record(c.Ops)
	more := ButtonProps{Label: "...", Variant: p.Variant, Size: p.Size, Disabled: p.Disabled}.Layout(c, &state.more)
	moreDrawn := measure.Stop()

	thickness := hairline(c)
	total := image.Pt(main.Size.X+thickness+more.Size.X, max(main.Size.Y, more.Size.Y))

	mainDrawn.Add(c.Ops)

	// The gap between the two is left unpainted rather than filled with the
	// page's own background. Filled, it is invisible on a page and wrong on a
	// card: a hairline of the window's colour drawn across a surface that is a
	// different colour. Unpainted, whatever is behind shows through, which is
	// what separates them wherever the pair is put.
	offset := op.Offset(image.Pt(main.Size.X+thickness, 0)).Push(c.Ops)
	moreDrawn.Add(c.Ops)
	offset.Pop()

	return ayra.Dimensions{Size: total}
}
