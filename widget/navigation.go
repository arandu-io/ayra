package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/theme"
)

// breadcrumbStep is the least width, in points, a step of a breadcrumb is laid
// out in, however narrow the column it is in.
const breadcrumbStep unit.Dp = 48

// Crumbs is the state half of a breadcrumb: the press on each step.
type Crumbs struct {
	clicks []engine.Clickable
}

// Clicked reports which step was pressed, and -1 when none was.
//
// An index rather than a boolean per step, because what a screen does with it
// is navigate somewhere, and "which one" is the whole question. A caller
// looping to find the true would be writing this function again.
func (c *Crumbs) Clicked(ctx ayra.Context) int {
	for index := range c.clicks {
		if c.clicks[index].Clicked(ctx.Context) {
			return index
		}
	}
	return -1
}

// BreadcrumbProps is the trail back to where somebody came from.
type BreadcrumbProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Steps are the places, in order, with the current one last. The last is
	// drawn as the page rather than as a link: a trail whose final step invites
	// a press offers to navigate to where somebody already is.
	Steps []string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p BreadcrumbProps) PartNames() []string { return []string{"root"} }

// Layout draws the trail and returns the room it took.
func (p BreadcrumbProps) Layout(c ayra.Context, state *Crumbs) ayra.Dimensions {
	return withRoot(c, "breadcrumb", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p BreadcrumbProps) layout(c ayra.Context, state *Crumbs) ayra.Dimensions {
	for len(state.clicks) < len(p.Steps) {
		state.clicks = append(state.clicks, engine.Clickable{})
	}

	// Each step is recorded with the caret before it, and the row is broken
	// where the next does not fit: a trail longer than a phone was cut at its
	// edge, and the page the person is on was the part cut off. Kyse's wraps.
	type piece struct {
		draw op.CallOp
		size image.Point
	}
	pieces := make([]piece, 0, len(p.Steps))
	least := c.Dp(touchTarget)

	// What a step may be laid out in: the line, less the caret before it, and never
	// less than breadcrumbStep. A step offered the few points a narrow column had
	// left was set a letter to a line; one that does not fit in this is cut with an
	// ellipsis -- the page the person is on included, which is cut and never left
	// out.
	caret := c.Dp(unit.Dp(12 + innerSideSmall))
	stepRoom := max(c.Constraints.Max.X-caret, c.Dp(breadcrumbStep))
	for index, step := range p.Steps {
		index, step := index, step
		recording := op.Record(c.Ops)
		inner := c
		inner.Constraints.Min = image.Point{}
		dims := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(inner.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if index == 0 {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawInner(c.With(gtx), innerIcon("breadcrumb", "CaretRight"), innerPixels(c, innerSideSmall), c.Theme.Colours.MutedForeground, 0)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				gtx.Constraints.Max.X = stepRoom
				in := c.With(gtx)
				if index == len(p.Steps)-1 {
					return drawText(in, step, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.Foreground, 1, text.Start, semibold())
				}
				// The press area is as high as a finger, and the words are in
				// the middle of it.
				return state.clicks[index].Layout(in.Context, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = least
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min = image.Point{}
							return drawText(c.With(gtx), step, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
						}),
					)
				})
			}),
		)
		pieces = append(pieces, piece{draw: recording.Stop(), size: dims.Size})
	}

	// The pieces are set into lines first, so that each can be centred in the
	// height of its line: a step as high as a finger beside one that is not.
	type placed struct {
		piece
		x int
	}
	var lines [][]placed
	var heights []int
	var x, lineHeight, widest int
	room := c.Constraints.Max.X
	line := []placed(nil)
	for _, piece := range pieces {
		if x > 0 && x+piece.size.X > room {
			lines, heights = append(lines, line), append(heights, lineHeight)
			line, x, lineHeight = nil, 0, 0
		}
		line = append(line, placed{piece: piece, x: x})
		x += piece.size.X
		widest = max(widest, x)
		lineHeight = max(lineHeight, piece.size.Y)
	}
	lines, heights = append(lines, line), append(heights, lineHeight)

	y := 0
	for index, line := range lines {
		for _, item := range line {
			at := op.Offset(image.Pt(item.x, y+(heights[index]-item.size.Y)/2)).Push(c.Ops)
			item.draw.Add(c.Ops)
			at.Pop()
		}
		y += heights[index]
	}
	return ayra.Dimensions{Size: image.Pt(widest, y)}
}

// LinkProps is text that goes somewhere.
//
// It is drawn rather than reused from a button because the two are different
// promises: a button does something on this screen, and a link leaves it. The
// rule under it is what says which, and it is the only thing that can -- there
// is no cursor to change on a touch screen.
type LinkProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Text is what is written.
	Text string
	// Muted draws it as a secondary link, for one in a footer or beside a
	// paragraph rather than in it.
	Muted bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p LinkProps) PartNames() []string { return []string{"root"} }

// Layout draws the link and returns the room it took.
func (p LinkProps) Layout(c ayra.Context, state *Button) ayra.Dimensions {
	return withRoot(c, "link", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p LinkProps) layout(c ayra.Context, state *Button) ayra.Dimensions {
	ink := c.Theme.Colours.Primary
	if p.Muted {
		ink = c.Theme.Colours.MutedForeground
	}

	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Point{}

		size := unit.Sp(c.Theme.Type.Body)
		dims := drawText(inner, p.Text, size, ink, 1, text.Start, plain())
		underline(inner, dims, p.Text, theme.Style{Foreground: theme.Colour(ink), Font: theme.Font{Size: theme.Pt(float32(size))}})
		return dims
	})
}

// EmptyProps is what a region with nothing in it says.
//
// A region left blank and a region that is empty look the same and mean
// different things: the first is something that has not loaded, the second is
// something there is none of. Saying which is the whole job.
type EmptyProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Title is the one line.
	Title string
	// Body is the sentence under it, and the place to say what would put
	// something here.
	Body string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p EmptyProps) PartNames() []string { return []string{"root"} }

// Layout draws the message centred in the room it was given.
func (p EmptyProps) Layout(c ayra.Context) ayra.Dimensions {
	return withRoot(c, "empty", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c)
	})
}

// layout is Layout without the root part around it.
func (p EmptyProps) layout(c ayra.Context) ayra.Dimensions {
	return layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(inner.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawText(inner.With(gtx), p.Title, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Middle, semibold())
			}),
			layout.Rigid(layout.Spacer{Height: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.Body == "" {
					return layout.Dimensions{}
				}
				return drawText(inner.With(gtx), p.Body, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 0, text.Middle, plain())
			}),
		)
	})
}

// StatProps is one measured figure and what it counts.
type StatProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Value is the figure, already formatted: this draws it and does not decide
	// how a number is written, because that is a locale's decision and not a
	// control's.
	Value string
	// Label is what was counted.
	Label string
	// Note is the smaller line under it -- a change, a period, a comparison.
	Note string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p StatProps) PartNames() []string { return []string{"root"} }

// Layout draws the figure above its label.
func (p StatProps) Layout(c ayra.Context) ayra.Dimensions {
	return withRoot(c, "stat", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c)
	})
}

// layout is Layout without the root part around it.
func (p StatProps) layout(c ayra.Context) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), p.Value, unit.Sp(c.Theme.Type.Display), c.Theme.Colours.Foreground, 1, text.Start, semibold())
		}),
		layout.Rigid(layout.Spacer{Height: 2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), p.Label, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.Note == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), p.Note, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
			})
		}),
	)
}
