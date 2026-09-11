package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	giowidget "github.com/arandu-io/ayra/engine/widget"
)

// Tabs is the state half: which one is selected, and the press on each.
//
// The tabs are held rather than rebuilt, because a Clickable that is recreated
// every frame is a Clickable that never sees the release of its own press.
type Tabs struct {
	selected int
	clicks   []giowidget.Clickable
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
	// Labels are the tabs, in order. What each one shows is the caller's: this
	// draws the row and says which is selected, and the screen draws the panel
	// for it. A props that owned the panels would need every one of them built
	// on every frame, including the ones nobody is looking at.
	Labels []string
}

// Layout draws the row and returns the room it took.
func (p TabsProps) Layout(c ayra.Context, state *Tabs) ayra.Dimensions {
	for len(state.clicks) < len(p.Labels) {
		state.clicks = append(state.clicks, giowidget.Clickable{})
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
			return drawText(inner.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Middle, face)
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
