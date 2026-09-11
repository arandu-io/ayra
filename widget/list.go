package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// ItemProps is one row of a list: a title, a line under it, and something on
// the right.
//
// It is a control rather than a layout somebody writes per screen, because a
// list whose rows are each written separately is a list whose rows drift a
// point apart -- and the drift is only visible when two of them are on screen
// at once, which is always.
type ItemProps struct {
	// Title is the row's own name.
	Title string
	// Body is the line under it. Empty draws the title alone.
	Body string
	// Pressable makes the whole row answer a press. A row that is not
	// pressable is information; one that is, is navigation.
	Pressable bool
	// Selected draws it as the current one.
	Selected bool
}

// Layout draws the row with whatever the caller puts on its right.
func (p ItemProps) Layout(c ayra.Context, state *Button, trailing ayra.Widget) ayra.Dimensions {
	draw := func(c ayra.Context) ayra.Dimensions {
		fill := c.Theme.Colours.Background
		if p.Selected {
			fill = c.Theme.Colours.Accent
		}

		return surface(c, fill, fill, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
			return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(inner.Context,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return p.lines(inner.With(gtx))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if trailing == nil {
							return layout.Dimensions{}
						}
						side := inner.With(gtx)
						side.Constraints.Min = image.Point{}
						return trailing(side)
					}),
				)
			})
		})
	}

	if !p.Pressable {
		return draw(c)
	}
	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return draw(c.With(gtx))
	})
}

// lines draws the title and whatever is under it.
func (p ItemProps) lines(c ayra.Context) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), p.Title, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, plain())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.Body == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), p.Body, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
			})
		}),
	)
}

// MeterProps is a measured value inside a range, drawn as a bar.
//
// It is not a [ProgressProps] with a different name. Progress is work going
// somewhere and ends; a meter is a level that goes up and down -- disk used,
// seats taken, a budget spent -- and never finishes. Drawing one as the other
// tells somebody their disk is loading.
type MeterProps struct {
	// Value, Low and High are the reading and the range it sits in. When Low
	// and High are both zero the range is nought to one.
	Value float32
	Low   float32
	High  float32
	// Over marks the bar as past what it should be, for a quota that has been
	// exceeded rather than merely filled.
	Over bool
}

// Layout draws the meter across the width it was given.
func (p MeterProps) Layout(c ayra.Context) ayra.Dimensions {
	low, high := p.Low, p.High
	if low == 0 && high == 0 {
		high = 1
	}
	if high <= low {
		// A range that does not increase has no position in it, and a division
		// by its width is an infinity that draws a bar off the screen.
		high = low + 1
	}

	fraction := (p.Value - low) / (high - low)
	fraction = min(max(fraction, 0), 1)

	height := c.Dp(unit.Dp(8))
	width := c.Constraints.Max.X

	track := image.Rectangle{Max: image.Pt(width, height)}
	paint.FillShape(c.Ops, c.Theme.Colours.Muted, clip.UniformRRect(track, height/2).Op(c.Ops))

	ink := c.Theme.Colours.Foreground
	if p.Over {
		ink = c.Theme.Colours.Destructive
	}
	if filled := int(float32(width) * fraction); filled > 0 {
		bar := image.Rect(0, 0, max(filled, height), height)
		paint.FillShape(c.Ops, ink, clip.UniformRRect(bar, height/2).Op(c.Ops))
	}

	return ayra.Dimensions{Size: track.Max}
}

// ButtonGroupProps is a set of buttons that belong together.
//
// They are drawn as one object with divisions rather than as buttons with a gap
// between them, and that is the whole point: a gap says "three choices", and a
// joined row says "one choice, three values".
type ButtonGroupProps struct {
	// Labels are the buttons, in order.
	Labels []string
	// Selected is the index drawn as chosen, or -1 for a group where pressing
	// is an action rather than a choice.
	Selected int
	// Size is how much room each takes. Zero is the one a toolbar is built
	// from.
	Size Size
}

// Group is the state half: the press on each button.
//
// It holds the buttons themselves rather than their clickables, so that what
// reaches ButtonProps.Layout is the one that has been there since the first
// frame. Handing it a copy is the defect this exists to avoid: a clickable
// copied per frame never sees the release of its own press, so the button can
// be held down and never fires.
type Group struct {
	buttons []Button
}

// Clicked reports which button was pressed, and -1 when none was.
func (g *Group) Clicked(c ayra.Context) int {
	for index := range g.buttons {
		if g.buttons[index].click.Clicked(c.Context) {
			return index
		}
	}
	return -1
}

// Layout draws the group and returns the room it took.
//
// The buttons are measured before anything is drawn, because the dividers
// between them need a height and a flex child cannot know its siblings'. Read
// from the constraints instead, a divider takes the height of the room the
// group was given -- which drew two rules straight down the screen, through
// everything under the group.
func (p ButtonGroupProps) Layout(c ayra.Context, state *Group) ayra.Dimensions {
	for len(state.buttons) < len(p.Labels) {
		state.buttons = append(state.buttons, Button{})
	}

	size := p.Size
	if size == Medium {
		size = Small
	}

	inner := c
	inner.Constraints.Min = image.Point{}

	drawn := make([]op.CallOp, len(p.Labels))
	sizes := make([]image.Point, len(p.Labels))
	height := 0

	for index, label := range p.Labels {
		variant := Ghost
		if index == p.Selected {
			variant = Secondary
		}

		record := op.Record(inner.Ops)
		sizes[index] = ButtonProps{Label: label, Variant: variant, Size: size}.Layout(inner, &state.buttons[index]).Size
		drawn[index] = record.Stop()
		height = max(height, sizes[index].Y)
	}

	thickness := hairline(c)
	width := 0
	for index, size := range sizes {
		if index > 0 {
			width += thickness
		}
		width += size.X
	}

	total := image.Pt(width, height)
	shape := clip.UniformRRect(image.Rectangle{Max: total}, controlRadius(c))
	paint.FillShape(c.Ops, c.Theme.Colours.Background, shape.Op(c.Ops))

	at := 0
	for index := range p.Labels {
		if index > 0 {
			rule := image.Rect(at, 0, at+thickness, height)
			paint.FillShape(c.Ops, c.Theme.Colours.Border, clip.Rect(rule).Op())
			at += thickness
		}

		offset := op.Offset(image.Pt(at, 0)).Push(c.Ops)
		drawn[index].Add(c.Ops)
		offset.Pop()
		at += sizes[index].X
	}

	stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(thickness)}
	paint.FillShape(c.Ops, c.Theme.Colours.Border, stroke.Op())

	return ayra.Dimensions{Size: total}
}
