package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	giowidget "github.com/arandu-io/ayra/engine/widget"
)

// Crumbs is the state half of a breadcrumb: the press on each step.
type Crumbs struct {
	clicks []giowidget.Clickable
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
	// Steps are the places, in order, with the current one last. The last is
	// drawn as the page rather than as a link: a trail whose final step invites
	// a press offers to navigate to where somebody already is.
	Steps []string
}

// Layout draws the trail and returns the room it took.
func (p BreadcrumbProps) Layout(c ayra.Context, state *Crumbs) ayra.Dimensions {
	for len(state.clicks) < len(p.Steps) {
		state.clicks = append(state.clicks, giowidget.Clickable{})
	}

	children := make([]layout.FlexChild, 0, len(p.Steps)*2)
	for index, step := range p.Steps {
		index, step := index, step
		if index > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), "/", unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
				})
			}))
		}

		last := index == len(p.Steps)-1
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}

			if last {
				return drawText(inner, step, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.Foreground, 1, text.Start, semibold())
			}
			return state.clicks[index].Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), step, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
			})
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}

// LinkProps is text that goes somewhere.
//
// It is drawn rather than reused from a button because the two are different
// promises: a button does something on this screen, and a link leaves it. The
// rule under it is what says which, and it is the only thing that can -- there
// is no cursor to change on a touch screen.
type LinkProps struct {
	// Text is what is written.
	Text string
	// Muted draws it as a secondary link, for one in a footer or beside a
	// paragraph rather than in it.
	Muted bool
}

// Layout draws the link and returns the room it took.
func (p LinkProps) Layout(c ayra.Context, state *Button) ayra.Dimensions {
	ink := c.Theme.Colours.Primary
	if p.Muted {
		ink = c.Theme.Colours.MutedForeground
	}

	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Point{}

		size := unit.Sp(c.Theme.Type.Body)
		dims := drawText(inner, p.Text, size, ink, 1, text.Start, plain())
		underline(inner, dims, p.Text, size, ink)
		return dims
	})
}

// EmptyProps is what a region with nothing in it says.
//
// A region left blank and a region that is empty look the same and mean
// different things: the first is something that has not loaded, the second is
// something there is none of. Saying which is the whole job.
type EmptyProps struct {
	// Title is the one line.
	Title string
	// Body is the sentence under it, and the place to say what would put
	// something here.
	Body string
}

// Layout draws the message centred in the room it was given.
func (p EmptyProps) Layout(c ayra.Context) ayra.Dimensions {
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
	// Value is the figure, already formatted: this draws it and does not decide
	// how a number is written, because that is a locale's decision and not a
	// control's.
	Value string
	// Label is what was counted.
	Label string
	// Note is the smaller line under it -- a change, a period, a comparison.
	Note string
}

// Layout draws the figure above its label.
func (p StatProps) Layout(c ayra.Context) ayra.Dimensions {
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
