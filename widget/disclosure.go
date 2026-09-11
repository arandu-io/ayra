package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	giowidget "github.com/arandu-io/ayra/engine/widget"
)

// Disclosure is the state half of something that opens and closes.
//
// On the web this control is a browser element and carries no logic at all --
// which is why porting it is writing it. The machine is four lines, and they
// are four lines every product would otherwise write once per screen.
type Disclosure struct {
	click giowidget.Clickable
	open  bool
}

// Open shows the content, Close hides it, and Showing reports which.
func (d *Disclosure) Open()         { d.open = true }
func (d *Disclosure) Close()        { d.open = false }
func (d *Disclosure) Showing() bool { return d.open }
func (d *Disclosure) Toggle()       { d.open = !d.open }

// Changed reports that somebody opened or closed it, and consumes that.
func (d *Disclosure) Changed(c ayra.Context) bool {
	if !d.click.Clicked(c.Context) {
		return false
	}
	d.open = !d.open
	return true
}

// CollapsibleProps is a summary that reveals more.
type CollapsibleProps struct {
	// Summary is the line that is always visible.
	Summary string
}

// Layout draws the summary and, when open, the content under it.
func (p CollapsibleProps) Layout(c ayra.Context, state *Disclosure, content ayra.Widget) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return summaryRow(c.With(gtx), state, p.Summary)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !state.open {
				// Nothing drawn and no room taken. A closed section that still
				// occupied its height would push everything under it down for
				// content nobody can see.
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 8, Left: 22}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return content(c.With(gtx))
			})
		}),
	)
}

// summaryRow draws the pressable line: a marker that turns, and the text.
func summaryRow(c ayra.Context, state *Disclosure, summary string) ayra.Dimensions {
	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return layout.Inset{Top: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return marker(inner.With(gtx), state.open)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return drawText(inner.With(gtx), summary, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, semibold())
				}),
			)
		})
	})
}

// marker draws the triangle that says which way the section is.
//
// It points right when closed and down when open, which is the one convention
// every platform shares -- and the reason it is drawn rather than written as a
// character is that a glyph would come from whichever font the screen happens
// to carry.
func marker(c ayra.Context, open bool) ayra.Dimensions {
	side := c.Dp(unit.Dp(10))
	width := float32(side)

	var arrow clip.Path
	arrow.Begin(c.Ops)
	if open {
		arrow.MoveTo(f32.Pt(0, width*0.3))
		arrow.LineTo(f32.Pt(width, width*0.3))
		arrow.LineTo(f32.Pt(width/2, width*0.8))
	} else {
		arrow.MoveTo(f32.Pt(width*0.3, 0))
		arrow.LineTo(f32.Pt(width*0.8, width/2))
		arrow.LineTo(f32.Pt(width*0.3, width))
	}
	arrow.Close()

	paint.FillShape(c.Ops, c.Theme.Colours.MutedForeground, clip.Outline{Path: arrow.End()}.Op())
	return ayra.Dimensions{Size: image.Pt(side, side)}
}

// AccordionProps is a set of sections where the open one is the only one.
//
// One at a time is the difference from a column of collapsibles, and it is the
// reason this is a type: keeping the others closed is a loop every screen would
// otherwise write, and the one that forgets produces a page that grows until it
// cannot be read.
type AccordionProps struct {
	// Sections are the summaries, in order.
	Sections []string
}

// Accordion is the state half: which section is open, and the press on each.
type Accordion struct {
	open   int
	clicks []giowidget.Clickable
}

// Open shows one section and closes the rest. A negative index closes them all.
func (a *Accordion) Open(index int) { a.open = index }

// Showing is the open section, or -1 when none is.
func (a *Accordion) Showing() int { return a.open }

// Layout draws the sections, asking content for the body of the open one.
func (p AccordionProps) Layout(c ayra.Context, state *Accordion, content func(int) ayra.Widget) ayra.Dimensions {
	for len(state.clicks) < len(p.Sections) {
		state.clicks = append(state.clicks, giowidget.Clickable{})
	}
	if len(state.clicks) > 0 && state.open == 0 && len(p.Sections) == 0 {
		state.open = -1
	}

	for index := range p.Sections {
		if state.clicks[index].Clicked(c.Context) {
			if state.open == index {
				state.open = -1
			} else {
				state.open = index
			}
		}
	}

	children := make([]layout.FlexChild, 0, len(p.Sections)*3)
	for index, summary := range p.Sections {
		index, summary := index, summary

		if index > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SeparatorProps{}.Layout(c.With(gtx))
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return state.clicks[index].Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 10, Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return marker(inner.With(gtx), state.open == index)
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return drawText(inner.With(gtx), summary, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, semibold())
						}),
					)
				})
			})
		}))
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if state.open != index || content == nil {
				return layout.Dimensions{}
			}
			body := content(index)
			if body == nil {
				return layout.Dimensions{}
			}
			return layout.Inset{Bottom: 10, Left: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return body(c.With(gtx))
			})
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}
