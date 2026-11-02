package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
)

// Toggle is the state half of a control that is either on or off: a checkbox,
// a switch, or one option of a radio group.
//
// One value for all three, because the state is the same state. What differs is
// the drawing.
//
// Changed flips the value whichever control it is. A radio group turns its other
// options off in the caller's own loop, because only the caller knows which
// options are in the group -- nothing here does.
type Toggle struct {
	click engine.Clickable
	on    bool
}

// On reports whether it is set.
func (t *Toggle) On() bool { return t.on }

// Set turns it on or off without a person touching it, for the state a screen
// arrives with.
func (t *Toggle) Set(on bool) { t.on = on }

// Changed reports a completed press and consumes it, having already applied the
// change.
//
// Applied here rather than by the caller because forgetting is silent: the
// control draws its old state, the press appears to do nothing, and the bug
// looks like the pointer missed.
//
// A toggle drawn disabled is no target for the pointer or the keyboard, so it
// reports nothing and its value does not move while it is drawn that way.
func (t *Toggle) Changed(c ayra.Context) bool {
	if !t.click.Clicked(c.Context) {
		return false
	}
	t.on = !t.on
	return true
}

// Hovered reports whether the pointer is over it.
func (t *Toggle) Hovered() bool { return t.click.Hovered() }

// CheckboxProps is what a box somebody ticks is drawn from.
type CheckboxProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Label is the text beside it. A checkbox with no label is a checkbox
	// nobody can describe, so it is drawn but the caller has said something
	// about it elsewhere.
	Label string
	// Mixed draws the box as partly set -- a dash where the tick goes -- for the
	// box that stands for a group of which some are set and some are not, the
	// "select all" of a list. It is what is drawn and not a third value of the
	// state: a press still flips the [Toggle], and what the group does next is
	// the caller's to decide, as the box has no way to know the others.
	Mixed bool
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p CheckboxProps) PartNames() []string { return []string{"root"} }

// Layout draws the checkbox and its label, and returns the room they took.
func (p CheckboxProps) Layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return withRoot(c, "checkbox", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p CheckboxProps) layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return layoutToggle(c, state, p.Disabled, p.Label, func(c ayra.Context, state *Toggle, disabled bool) ayra.Dimensions {
		return drawBox(c, state, disabled, p.Mixed)
	})
}

// RadioProps is what one option of a set is drawn from.
//
// The group is the caller's: this draws one option and reports when it was
// chosen, and turning the others off is the screen's own loop. A group type
// here would own the options, and a screen whose options come from a server
// would then have to build one every frame.
type RadioProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Label is the text beside it.
	Label string
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p RadioProps) PartNames() []string { return []string{"root"} }

// Layout draws the option and its label.
func (p RadioProps) Layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return withRoot(c, "radio", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p RadioProps) layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return layoutToggle(c, state, p.Disabled, p.Label, drawDot)
}

// SwitchProps is what a control that turns something on is drawn from.
//
// It is a switch rather than a checkbox when the change takes effect at once. A
// checkbox is a value in a form that is submitted later, and the difference is
// what somebody expects to happen when they let go.
type SwitchProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Label is the text beside it.
	Label string
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SwitchProps) PartNames() []string { return []string{"root"} }

// Layout draws the switch and its label.
func (p SwitchProps) Layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return withRoot(c, "switch", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p SwitchProps) layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return layoutToggle(c, state, p.Disabled, p.Label, drawTrack)
}

// layoutToggle is the shape all three share: a mark, a gap, and a label, all of
// it one target for the pointer.
//
// One target and not two, because a label that does not answer the press is the
// commonest way a small control becomes hard to hit -- and it is hard to hit in
// exactly the way nobody notices while testing with a mouse.
//
// A label that does not fit goes on to a second line, beside the mark and under
// its own first line, as Kyse's does. It was cut at the edge with an ellipsis,
// and the sentence a person agrees to is the one they most need to read to the
// end. The mark stays level with the first line, which is where the eye starts.
//
// A control with a label is at least 44 dp tall, which is what a thumb needs:
// the mark is 18 and the line is not much more, and a form of them was a column
// of targets half the height of a finger.
func layoutToggle(c ayra.Context, state *Toggle, disabled bool, label string, mark func(ayra.Context, *Toggle, bool) ayra.Dimensions) ayra.Dimensions {
	if disabled {
		c.Context = c.Context.Disabled()
	}

	row := func(c ayra.Context) ayra.Dimensions {
		gtx := c.Context
		inner := c
		inner.Constraints.Min = image.Point{}

		// The first line of the label is what the mark is centred on. It is
		// measured on a letter, so a label of one word and a label of three
		// paragraphs put the mark in the same place.
		measured := op.Record(inner.Ops)
		line := drawText(inner, "Ag", unit.Sp(inner.Theme.Type.Body), color.NRGBA{}, 1, 0, plain()).Size.Y
		glyph := mark(inner, state, disabled).Size.Y
		measured.Stop()
		drop := max((line-glyph)/2, 0)

		recorded := op.Record(inner.Ops)
		dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(inner.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				shifted := op.Offset(image.Pt(0, drop)).Push(gtx.Ops)
				size := mark(inner.With(gtx), state, disabled).Size
				shifted.Pop()
				return layout.Dimensions{Size: image.Pt(size.X, size.Y+drop)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if label == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ink := inner.Theme.Colours.Foreground
					if disabled {
						ink = fade(ink)
					}
					return drawText(inner.With(gtx), label, unit.Sp(inner.Theme.Type.Body), ink, 0, 0, plain())
				})
			}),
		)
		row := recorded.Stop()

		height := dims.Size.Y
		if label != "" {
			height = max(height, inner.Dp(unit.Dp(touchTarget)))
		}
		centred := op.Offset(image.Pt(0, (height-dims.Size.Y)/2)).Push(gtx.Ops)
		row.Add(gtx.Ops)
		centred.Pop()
		return layout.Dimensions{Size: image.Pt(max(dims.Size.X, c.Constraints.Min.X), height), Baseline: dims.Baseline + (height-dims.Size.Y)/2}
	}

	if label == "" {
		// A mark on its own is 18 points, or 36 by 20: the area that answers is
		// grown to a finger and the picture does not move.
		return TouchArea(c, func(c ayra.Context, w ayra.Widget) ayra.Dimensions {
			return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions { return w(c.With(gtx)) })
		}, row)
	}
	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions { return row(c.With(gtx)) })
}

// drawBox draws a checkbox: a square that fills and carries a tick when set, and
// a dash when it is partly so.
func drawBox(c ayra.Context, state *Toggle, disabled, mixed bool) ayra.Dimensions {
	side := c.Dp(unit.Dp(18))
	fill, border, ink := toggleColours(c, state.On() || mixed, disabled)

	box := image.Rectangle{Max: image.Pt(side, side)}
	shape := clip.UniformRRect(box, c.Dp(unit.Dp(4)))

	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(hairline(c))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	switch {
	case mixed:
		drawDash(c, side, ink)
	case state.On():
		drawTick(c, side, ink)
	}

	return ayra.Dimensions{Size: box.Max}
}

// drawTick draws the mark inside a ticked box: Kyse's Check, at the size its
// stylesheet gives the tick of a checkbox, 0.875rem in a box of 1rem, centred.
//
// The browser draws that tick from the stylesheet, as a Lucide check; the one in
// Kyse's own icon set is Check, and this is the same one the Kyse components
// that draw a tick themselves use.
func drawTick(c ayra.Context, side int, ink color.NRGBA) {
	mark := side * innerSideSmall / innerSide
	shift := op.Offset(image.Pt((side-mark)/2, (side-mark)/2)).Push(c.Ops)
	drawInner(c, innerIcon("checkbox", "Check"), mark, ink, 0)
	shift.Pop()
}

// drawDash draws the mark of a box that is partly set: a bar across the middle,
// as wide as the tick is and as thick as the strokes of the tick's icon.
func drawDash(c ayra.Context, side int, ink color.NRGBA) {
	width := side * innerSideSmall / innerSide
	thickness := max(c.Dp(unit.Dp(2)), 2)
	left, top := (side-width)/2, (side-thickness)/2
	bar := image.Rect(left, top, left+width, top+thickness)
	paint.FillShape(c.Ops, ink, clip.UniformRRect(bar, thickness/2).Op(c.Ops))
}

// drawDot draws a radio option: a circle with a smaller one inside when set.
func drawDot(c ayra.Context, state *Toggle, disabled bool) ayra.Dimensions {
	side := c.Dp(unit.Dp(18))
	fill, border, ink := toggleColours(c, state.On(), disabled)

	circle := image.Rectangle{Max: image.Pt(side, side)}
	shape := clip.UniformRRect(circle, side/2)

	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(hairline(c))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	if state.On() {
		inset := side / 4
		dot := image.Rect(inset, inset, side-inset, side-inset)
		paint.FillShape(c.Ops, ink, clip.UniformRRect(dot, (side-2*inset)/2).Op(c.Ops))
	}

	return ayra.Dimensions{Size: circle.Max}
}

// drawTrack draws a switch: a track with a knob at one end.
func drawTrack(c ayra.Context, state *Toggle, disabled bool) ayra.Dimensions {
	height := c.Dp(unit.Dp(20))
	width := c.Dp(unit.Dp(36))
	fill, border, knob := toggleColours(c, state.On(), disabled)

	if !state.On() {
		// An off switch is a filled track rather than an outline: an outlined
		// one at this size reads as a disabled control, and disabled is a
		// different thing that has to stay distinguishable. The track is the
		// input colour, as Kyse's is, and the knob is the page's own colour in
		// the light scheme and the text's in the dark one -- toggleColours gives
		// an off control no ink, and a knob with none was a track with nothing on
		// it, which is how an off switch read as a pill and not as a switch.
		fill, knob = c.Theme.Colours.Input, c.Theme.Colours.Background
		if luminance(c.Theme.Colours.Background) < 128 {
			knob = c.Theme.Colours.Foreground
		}
		if disabled {
			fill, knob = fade(fill), fade(knob)
		}
	}

	track := image.Rectangle{Max: image.Pt(width, height)}
	shape := clip.UniformRRect(track, height/2)
	paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	if border.A > 0 && !state.On() {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(hairline(c))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	margin := max(c.Dp(unit.Dp(2)), 2)
	size := height - 2*margin
	left := margin
	if state.On() {
		left = width - margin - size
	}
	handle := image.Rect(left, margin, left+size, margin+size)
	round := clip.UniformRRect(handle, size/2)
	paint.FillShape(c.Ops, knob, round.Op(c.Ops))
	if !state.On() {
		// A ring, so the knob has an edge against a track close to it in
		// lightness: white on #e5e5e5 is 1.3 to 1 without one.
		ring := c.Theme.Colours.MutedForeground
		ring.A = ring.A * 3 / 5
		if disabled {
			ring = fade(ring)
		}
		stroke := clip.Stroke{Path: round.Path(c.Ops), Width: float32(hairline(c))}
		paint.FillShape(c.Ops, ring, stroke.Op())
	}

	return ayra.Dimensions{Size: track.Max}
}

// toggleColours answers the fill, the border and the mark for a control that is
// on or off.
//
// Half opacity says "not now" rather than a grey of its own, for the reason a
// button gives: a grey per state is a colour the palette has to name, and every
// name is a chance for one of them to be wrong.
func toggleColours(c ayra.Context, on, disabled bool) (fill, border, ink color.NRGBA) {
	transparent := color.NRGBA{}

	fill, border, ink = transparent, c.Theme.Colours.Border, transparent
	if on {
		fill, border, ink = c.Theme.Colours.Primary, transparent, c.Theme.Colours.PrimaryForeground
	}

	if disabled {
		fill, border, ink = fade(fill), fade(border), fade(ink)
	}
	return fill, border, ink
}

// luminance is how bright a colour is, from 0 to 255.
func luminance(c color.NRGBA) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// touchTarget is the least height, in points, of a row or a strip a finger has to
// hit: a control with a label, the tabs, the summary of a section, a step of a
// breadcrumb and a destination of the navigation. The text in them is 17 points
// tall, and a row that is only as tall as its text is one a thumb misses.
const touchTarget unit.Dp = 44
