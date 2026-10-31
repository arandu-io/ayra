package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/icons"
)

// Tabs is the state half: which one is selected, and the press on each.
//
// The tabs are held rather than rebuilt, because a Clickable that is recreated
// every frame is a Clickable that never sees the release of its own press.
type Tabs struct {
	selected int
	clicks   []engine.Clickable
}

// Selected is the index showing.
func (t *Tabs) Selected() int { return t.selected }

// Select shows one, for the state a screen arrives with.
func (t *Tabs) Select(index int) { t.selected = index }

// Changed reports that somebody chose a different tab, and consumes that.
func (t *Tabs) Changed(c ayra.Context) bool {
	changed := false
	for index := range t.clicks {
		if t.clicks[index].Clicked(c.Context) && t.selected != index {
			t.selected, changed = index, true
		}
	}
	return changed
}

// TabsProps is what a row of tabs is drawn from.
type TabsProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Labels are the tabs, in order. What each one shows is the caller's: this
	// draws the row and says which is selected, and the screen draws the panel
	// for it. A props that owned the panels would need every one of them built
	// on every frame, including the ones nobody is looking at.
	Labels []string
	// Icons are drawn before each label, by position. Fewer icons than labels
	// leaves the rest without one, and the zero Icon draws nothing. An icon takes
	// the colour of its tab's text, so it follows the selection and the scheme
	// with it.
	Icons []icons.Icon
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p TabsProps) PartNames() []string { return []string{"root"} }

// Layout draws the row and returns the room it took.
func (p TabsProps) Layout(c ayra.Context, state *Tabs) ayra.Dimensions {
	return withRoot(c, "tabs", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p TabsProps) layout(c ayra.Context, state *Tabs) ayra.Dimensions {
	for len(state.clicks) < len(p.Labels) {
		state.clicks = append(state.clicks, engine.Clickable{})
	}
	if state.selected >= len(p.Labels) {
		state.selected = 0
	}

	children := make([]layout.FlexChild, 0, len(p.Labels))
	for index, label := range p.Labels {
		index, label := index, label
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.tab(c.With(gtx), state, index, label)
		}))
	}

	dims := layout.Flex{Axis: layout.Horizontal}.Layout(c.Context, children...)

	// The rule under the whole row, drawn after the tabs so the selected one's
	// own marker sits on top of it rather than beside it.
	rule := image.Rect(0, dims.Size.Y-hairline(c), c.Constraints.Max.X, dims.Size.Y)
	paint.FillShape(c.Ops, c.Theme.Colours.Border, clip.Rect(rule).Op())

	return ayra.Dimensions{Size: image.Pt(c.Constraints.Max.X, dims.Size.Y)}
}

// tab draws one of them.
func (p TabsProps) tab(c ayra.Context, state *Tabs, index int, label string) ayra.Dimensions {
	ink := c.Theme.Colours.MutedForeground
	face := plain()
	if state.selected == index {
		ink, face = c.Theme.Colours.Foreground, semibold()
	}

	return state.clicks[index].Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Point{}

		dims := layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
			var icon icons.Icon
			if index < len(p.Icons) {
				icon = p.Icons[index]
			}
			if icon.IsZero() {
				return drawText(inner.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Middle, face)
			}
			side := c.Dp(unit.Dp(c.Theme.Type.Body))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					DrawIcon(inner.With(gtx), icon, side, ink)
					return layout.Dimensions{Size: image.Pt(side, side)}
				}),
				layout.Rigid(layout.Spacer{Width: 6}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return drawText(inner.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Middle, face)
				}),
			)
		})

		if state.selected == index {
			// Two points rather than one: the marker has to read as heavier
			// than the rule it sits on, or the selected tab looks like the
			// edge of the row.
			thickness := max(c.Dp(unit.Dp(2)), 2)
			marker := image.Rect(0, dims.Size.Y-thickness, dims.Size.X, dims.Size.Y)
			paint.FillShape(inner.Ops, c.Theme.Colours.Foreground, clip.Rect(marker).Op())
		}
		return dims
	})
}
