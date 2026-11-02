package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Side is the edge it comes from. Zero is the right.
	Side Side
	// Size is how wide it is, or how tall from the bottom. Zero takes a width
	// a form reads at.
	Size unit.Dp
	// Modal darkens what is behind and swallows presses on it.
	Modal bool
	// Dismissible lets escape close it, and a press outside close it when Modal
	// is set as well: the press outside is read from the barrier the scrim puts
	// up, and without one there is nothing to read.
	Dismissible bool
	// Scroll lays the content out in a column that scrolls when it is taller than
	// the panel. It is a prop and not the rule because the two contents a drawer
	// holds want different things: a form of fields is as tall as it is and wants
	// to scroll, and a sheet that fills the panel -- a footer of buttons at the
	// bottom, a list of its own that scrolls -- is given the panel's height and
	// would be given an unbounded one in a column. The content of a scrolling
	// drawer takes the room it needs and is not stretched to the panel's.
	Scroll bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DrawerProps) PartNames() []string { return []string{"root"} }

// Layout draws the drawer over the screen and returns the room it took.
func (p DrawerProps) Layout(c ayra.Context, state *Dialog, content ayra.Widget) ayra.Dimensions {
	return withRoot(c, "drawer", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state, content)
	})
}

// layout is Layout without the root part around it.
func (p DrawerProps) layout(c ayra.Context, state *Dialog, content ayra.Widget) ayra.Dimensions {
	if !state.Showing() {
		return ayra.Dimensions{}
	}

	// Over the window, as the dialog is: the drawer comes in from the edge of the
	// window and not from the edge of the box that opened it.
	dims := inWindow(c, func(c ayra.Context, window image.Rectangle) ayra.Dimensions {
		return p.place(c, state, content, window)
	})
	return dims
}

// place draws the scrim over the window and the panel against its edge.
//
// The panel runs to the edge of the window, under the system bars, and its
// content is held inside them: what has to be read is not drawn behind the
// status bar or the keyboard.
func (p DrawerProps) place(c ayra.Context, state *Dialog, content ayra.Widget, window image.Rectangle) ayra.Dimensions {
	area := c.Constraints.Max
	state.overlay(c, window, p.Modal, p.Modal || p.Dismissible)

	size := p.Size
	if size == 0 {
		size = 380
	}

	panel := p.bounds(c, area, window, size)
	inner := c
	inner.Constraints = layout.Exact(panel.Size())

	// What the system covers, on each edge the panel touches.
	var covered struct{ left, top, right, bottom int }
	if panel.Min.X <= window.Min.X {
		covered.left = max(0, -window.Min.X)
	}
	if panel.Min.Y <= window.Min.Y {
		covered.top = max(0, -window.Min.Y)
	}
	if panel.Max.X >= window.Max.X {
		covered.right = max(0, window.Max.X-area.X)
	}
	if panel.Max.Y >= window.Max.Y {
		covered.bottom = max(0, window.Max.Y-area.Y)
	}
	dp := func(pixels int) unit.Dp { return unit.Dp(float32(pixels) / c.Metric.PxPerDp) }

	measure := op.Record(c.Ops)
	surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, 0, func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{
			Left: 20 + dp(covered.left), Top: 20 + dp(covered.top),
			Right: 20 + dp(covered.right), Bottom: 20 + dp(covered.bottom),
		}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			if p.Scroll {
				return state.scrolled(c.With(gtx), content)
			}
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

	// Read after the content, as the dialog does, so what is open over the
	// drawer answers the back key first.
	state.dismiss(c, p.Modal, p.Dismissible)
	return ayra.Dimensions{Size: area}
}

// bounds answers where the panel sits against its edge of the window, in the
// coordinates of the room the screen was given. The bottom of a sheet is the
// bottom of that room, which is above the keyboard when it is up, and the panel
// is drawn on down to the bottom of the window.
func (p DrawerProps) bounds(c ayra.Context, area image.Point, window image.Rectangle, size unit.Dp) image.Rectangle {
	extent := c.Dp(size)

	switch p.Side {
	case Left:
		return image.Rect(window.Min.X, window.Min.Y, window.Min.X+min(extent, window.Dx()), window.Max.Y)
	case Bottom:
		top := max(area.Y-extent, window.Min.Y)
		return image.Rect(window.Min.X, top, window.Max.X, window.Max.Y)
	}
	return image.Rect(window.Max.X-min(extent, window.Dx()), window.Min.Y, window.Max.X, window.Max.Y)
}

// SplitProps is a button with a second control beside it for the other things
// it could do.
//
// The main action is the one somebody presses without reading, and the rest are
// behind the second half. A menu with the common action inside it makes the
// common action the slowest one.
type SplitProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Label is the main action.
	Label string
	// Variant is what the pair is for. Zero is the ordinary one.
	Variant Variant
	// Size is how much room they take.
	Size Size
	// MenuLabel names the second button for a screen reader, which is all that
	// names it: it draws a caret. Empty says "More options".
	MenuLabel string
	// Disabled draws both as unavailable.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SplitProps) PartNames() []string { return []string{"root"} }

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
	return withRoot(c, "split", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p SplitProps) layout(c ayra.Context, state *Split) ayra.Dimensions {
	c.Constraints.Min = image.Point{}

	measure := op.Record(c.Ops)
	main := ButtonProps{Label: p.Label, Variant: p.Variant, Size: p.Size, Disabled: p.Disabled}.Layout(c, &state.main)
	mainDrawn := measure.Stop()

	measure = op.Record(c.Ops)
	more := ButtonProps{
		Label: orDefault(p.MenuLabel, "More options"), Icon: innerIcon("split-button", "CaretDown"), IconOnly: true,
		Variant: p.Variant, Size: p.Size, Disabled: p.Disabled,
	}.Layout(c, &state.more)
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
