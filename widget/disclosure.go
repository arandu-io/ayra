package widget

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
)

// Disclosure is the state half of something that opens and closes.
//
// On the web this control is a browser element and carries no logic at all --
// which is why porting it is writing it. The machine is four lines, and they
// are four lines every product would otherwise write once per screen.
type Disclosure struct {
	click engine.Clickable
	open  bool
}

// Open shows the content.
func (d *Disclosure) Open() { d.open = true }

// Close hides it.
func (d *Disclosure) Close() { d.open = false }

// Showing reports whether the content is on screen.
func (d *Disclosure) Showing() bool { return d.open }

// Toggle shows the content when it is hidden and hides it when it is shown.
func (d *Disclosure) Toggle() { d.open = !d.open }

// Changed applies the press on the summary -- opening what was closed, closing
// what was open -- and reports that it did, once.
//
// It is not a passive reading. Layout does not call it, so a section nobody asks
// about never opens.
func (d *Disclosure) Changed(c ayra.Context) bool {
	if !d.click.Clicked(c.Context) {
		return false
	}
	d.open = !d.open
	return true
}

// CollapsibleProps is a summary that reveals more.
type CollapsibleProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Summary is the line that is always visible.
	Summary string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p CollapsibleProps) PartNames() []string { return []string{"root"} }

// Layout draws the summary and, when open, the content under it.
func (p CollapsibleProps) Layout(c ayra.Context, state *Disclosure, content ayra.Widget) ayra.Dimensions {
	return withRoot(c, "collapsible", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state, content)
	})
}

// layout is Layout without the root part around it.
func (p CollapsibleProps) layout(c ayra.Context, state *Disclosure, content ayra.Widget) ayra.Dimensions {
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
			return layout.Inset{Top: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return content(c.With(gtx))
			})
		}),
	)
}

// summaryRow draws the pressable line: the text, and the caret at its end.
//
// The caret is Kyse's, CaretDown, and the collapsible's does not turn: Kyse
// writes the same caret whether the section is open or closed.
func summaryRow(c ayra.Context, state *Disclosure, summary string) ayra.Dimensions {
	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		return layout.Inset{Top: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return drawText(inner.With(gtx), summary, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, semibold())
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawInner(inner.With(gtx), innerIcon("collapsible", "CaretDown"), innerPixels(inner, innerSide), inner.Theme.Colours.MutedForeground, 0)
				}),
			)
		})
	})
}

// AccordionProps is a set of sections where the open one is the only one.
//
// One at a time is the difference from a column of collapsibles, and it is the
// reason this is a type: keeping the others closed is a loop every screen would
// otherwise write, and the one that forgets produces a page that grows until it
// cannot be read.
type AccordionProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Sections are the summaries, in order.
	Sections []string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p AccordionProps) PartNames() []string { return []string{"root"} }

// Accordion is the state half: which section is open, and the press on each.
type Accordion struct {
	open   int
	clicks []engine.Clickable
}

// Open shows one section and closes the rest. A negative index closes them all.
func (a *Accordion) Open(index int) { a.open = index }

// Showing is the open section, or -1 when none is.
func (a *Accordion) Showing() int { return a.open }

// Layout draws the sections, asking content for the body of the open one.
func (p AccordionProps) Layout(c ayra.Context, state *Accordion, content func(int) ayra.Widget) ayra.Dimensions {
	return withRoot(c, "accordion", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state, content)
	})
}

// layout is Layout without the root part around it.
func (p AccordionProps) layout(c ayra.Context, state *Accordion, content func(int) ayra.Widget) ayra.Dimensions {
	for len(state.clicks) < len(p.Sections) {
		state.clicks = append(state.clicks, engine.Clickable{})
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
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return drawText(inner.With(gtx), summary, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 1, text.Start, semibold())
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							// Half a turn when open: the stylesheet's rotate-180 on the
							// last svg of an open section.
							quarters := 0
							if state.open == index {
								quarters = 2
							}
							return drawInner(inner.With(gtx), innerIcon("accordion", "CaretDown"), innerPixels(inner, innerSide), inner.Theme.Colours.MutedForeground, quarters)
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
			return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return body(c.With(gtx))
			})
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}
