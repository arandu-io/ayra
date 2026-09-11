package main

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/widget"
)

// stack draws widgets down the page with the same room between each.
func stack(gap unit.Dp, children ...ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		items := make([]layout.FlexChild, 0, len(children)*2)
		for index, child := range children {
			child := child
			if index > 0 {
				items = append(items, layout.Rigid(layout.Spacer{Height: gap}.Layout))
			}
			items = append(items, layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return child(c.With(gtx))
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, items...)
	}
}

// flow draws widgets across the page, starting a new line when the next one
// would not fit.
//
// A flex row does not wrap. Handed more children than there is width for it
// lays the rest out anyway, and they are drawn past the edge where nothing
// reports them missing. Most of what is drawn here is a closed set -- six
// variants, five sizes, six kinds of field -- and a closed set with two members
// off the side is a picture that says the set has four.
func flow(gap unit.Dp, children ...ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		room := c.Constraints.Max.X
		step := c.Dp(gap)

		// Every child is measured from the room it is offered rather than the
		// room it is required to fill, and then placed. A child of a wrapping
		// row cannot be told its position before the one beside it has a width,
		// so the two passes are what wrapping is.
		inner := c
		inner.Constraints.Min = image.Point{}

		drawn := make([]op.CallOp, len(children))
		sizes := make([]image.Point, len(children))
		for index, child := range children {
			record := op.Record(c.Ops)
			sizes[index] = child(inner).Size
			drawn[index] = record.Stop()
		}

		at := image.Point{}
		tallest, total := 0, image.Point{}

		for index := range children {
			if at.X > 0 && at.X+sizes[index].X > room {
				at = image.Pt(0, at.Y+tallest+step)
				tallest = 0
			}

			offset := op.Offset(at).Push(c.Ops)
			drawn[index].Add(c.Ops)
			offset.Pop()

			at.X += sizes[index].X + step
			tallest = max(tallest, sizes[index].Y)
			total.X = max(total.X, at.X-step)
			total.Y = max(total.Y, at.Y+sizes[index].Y)
		}
		return ayra.Dimensions{Size: total}
	}
}

// stage draws a control inside a box of a fixed size.
//
// A control that covers what is behind it is told how much that is by the room
// it is handed, and the room a scrolling column hands a child is the whole
// scroll. A dialog laid out there washes a scrim over a million points of
// nothing and centres its panel far below the row it belongs to. The box is how
// much such a control is allowed to cover, so that what it covers is what a
// reader can see.
func stage(width, height unit.Dp, content ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		size := image.Pt(min(c.Dp(width), c.Constraints.Max.X), c.Dp(height))

		inner := c
		inner.Constraints = layout.Exact(size)

		defer clip.Rect{Max: size}.Push(c.Ops).Pop()
		content(inner)
		return ayra.Dimensions{Size: size}
	}
}

// framed draws a control on the quieter surface, so that a demonstration whose
// own edges are pale has one somebody can see.
func framed(content ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return widget.CardProps{Muted: true, Padding: 12}.Layout(c, content)
	}
}

// note is the muted line that names what a row of a demonstration is showing --
// a variant, a size, a state.
//
// It is one line and never two: a catalogue where the writing is taller than
// the control has stopped being a picture of the control.
func note(content string) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return widget.TextProps{
			Content:  content,
			Role:     widget.Caption,
			Tone:     widget.Muted,
			MaxLines: 1,
		}.Layout(c)
	}
}

// labelled puts a name above a control, for a row of them that would otherwise
// be a set of unexplained shapes.
func labelled(name string, content ayra.Widget) ayra.Widget {
	return stack(4, note(name), content)
}

// tight draws a control at the size its own content asks for.
//
// A column hands its children the full width as both the least and the most
// they may take, and several controls fill what they are given -- which is
// right in a form and wrong in a row of six, where it makes the first one as
// wide as the page and leaves nothing for the rest.
func tight(content ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		c.Constraints.Min = image.Point{}
		return content(c)
	}
}

// sized draws a control in a column of a fixed width, for one that fills
// whatever it is offered and would otherwise take the whole page.
func sized(width unit.Dp, content ayra.Widget) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		room := min(c.Dp(width), c.Constraints.Max.X)
		c.Constraints.Min.X, c.Constraints.Max.X = room, room
		return content(c)
	}
}

// variants is the closed set of what a control can be for, in the order the
// library declares it.
//
// Written out rather than counted from the type, because Go does not answer
// what the values of a constant set are. A member added to the library and not
// here is a member this catalogue does not draw, which is what the completeness
// gate exists to say about controls and cannot say about the values inside one.
var variants = []widget.Variant{
	widget.Default,
	widget.Secondary,
	widget.Outline,
	widget.Ghost,
	widget.Destructive,
	widget.Link,
}

// sizes is the closed set of how much room a control takes, in the order the
// library declares it.
var sizes = []widget.Size{
	widget.Medium,
	widget.ExtraSmall,
	widget.Small,
	widget.Large,
	widget.Icon,
}
