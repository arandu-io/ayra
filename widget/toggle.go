package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
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
	// Label is the text beside it. A checkbox with no label is a checkbox
	// nobody can describe, so it is drawn but the caller has said something
	// about it elsewhere.
	Label string
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// Layout draws the checkbox and its label, and returns the room they took.
func (p CheckboxProps) Layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return layoutToggle(c, state, p.Disabled, p.Label, drawBox)
}

// RadioProps is what one option of a set is drawn from.
//
// The group is the caller's: this draws one option and reports when it was
// chosen, and turning the others off is the screen's own loop. A group type
// here would own the options, and a screen whose options come from a server
// would then have to build one every frame.
type RadioProps struct {
	// Label is the text beside it.
	Label string
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// Layout draws the option and its label.
func (p RadioProps) Layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return layoutToggle(c, state, p.Disabled, p.Label, drawDot)
}

// SwitchProps is what a control that turns something on is drawn from.
//
// It is a switch rather than a checkbox when the change takes effect at once. A
// checkbox is a value in a form that is submitted later, and the difference is
// what somebody expects to happen when they let go.
type SwitchProps struct {
	// Label is the text beside it.
	Label string
	// Disabled draws it as unavailable and stops it answering.
	Disabled bool
}

// Layout draws the switch and its label.
func (p SwitchProps) Layout(c ayra.Context, state *Toggle) ayra.Dimensions {
	return layoutToggle(c, state, p.Disabled, p.Label, drawTrack)
}

// layoutToggle is the shape all three share: a mark, a gap, and a label, all of
// it one target for the pointer.
//
// One target and not two, because a label that does not answer the press is the
// commonest way a small control becomes hard to hit -- and it is hard to hit in
// exactly the way nobody notices while testing with a mouse.
func layoutToggle(c ayra.Context, state *Toggle, disabled bool, label string, mark func(ayra.Context, *Toggle, bool) ayra.Dimensions) ayra.Dimensions {
	if disabled {
		c.Context = c.Context.Disabled()
	}

	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Point{}

		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(inner.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return mark(inner.With(gtx), state, disabled)
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
					return drawText(inner.With(gtx), label, unit.Sp(inner.Theme.Type.Body), ink, 1, 0, plain())
				})
			}),
		)
	})
}

// drawBox draws a checkbox: a square that fills and carries a tick when set.
func drawBox(c ayra.Context, state *Toggle, disabled bool) ayra.Dimensions {
	side := c.Dp(unit.Dp(18))
	fill, border, ink := toggleColours(c, state.On(), disabled)

	box := image.Rectangle{Max: image.Pt(side, side)}
	shape := clip.UniformRRect(box, c.Dp(unit.Dp(4)))

	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border.A > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(hairline(c))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	if state.On() {
		drawTick(c, side, ink)
	}

	return ayra.Dimensions{Size: box.Max}
}

// drawTick draws the mark inside a ticked box.
//
// A stroked path of two segments, and not the pair of rectangles this had
// first: two rectangles meeting at a right angle draw a letter L, which is
// what it looked like -- correct in every dimension and wrong to every eye.
func drawTick(c ayra.Context, side int, ink color.NRGBA) {
	width := float32(side)
	thickness := float32(side) / 8

	var tick clip.Path
	tick.Begin(c.Ops)
	tick.MoveTo(f32.Pt(width*0.26, width*0.52))
	tick.LineTo(f32.Pt(width*0.43, width*0.70))
	tick.LineTo(f32.Pt(width*0.76, width*0.32))

	stroke := clip.Stroke{Path: tick.End(), Width: thickness}
	paint.FillShape(c.Ops, ink, stroke.Op())
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
		// different thing that has to stay distinguishable.
		fill = c.Theme.Colours.Muted
		if disabled {
			fill = fade(fill)
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
	paint.FillShape(c.Ops, knob, clip.UniformRRect(handle, size/2).Op(c.Ops))

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
