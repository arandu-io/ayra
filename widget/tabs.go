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

	// row is the strip the tabs are laid out in, which scrolls sideways when
	// they are wider than the room, and reveal says that the selected one is to
	// be brought into view on the next frame.
	row    layout.List
	reveal bool
}

// Selected is the index showing.
func (t *Tabs) Selected() int { return t.selected }

// Select shows one, for the state a screen arrives with.
func (t *Tabs) Select(index int) { t.selected, t.reveal = index, true }

// Changed reports that somebody chose a different tab, and consumes that.
func (t *Tabs) Changed(c ayra.Context) bool {
	changed := false
	for index := range t.clicks {
		if t.clicks[index].Clicked(c.Context) && t.selected != index {
			t.selected, t.reveal, changed = index, true, true
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

	// The tabs are a strip that scrolls: more of them, or longer words, than the
	// room has were cut off at its edge, and the ones past it could not be
	// reached by any means. A strip is as wide as the room and the tabs take what
	// they need.
	state.row.Axis = layout.Horizontal
	if state.reveal {
		state.row.ScrollTo(state.selected)
		state.reveal = false
	}
	room := c
	room.Constraints.Min = image.Point{}
	dims := state.row.Layout(room.Context, len(p.Labels), func(gtx layout.Context, index int) layout.Dimensions {
		return p.tab(c.With(gtx), state, index, p.Labels[index])
	})

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

		// A tab is at least 44 points high: a finger is not a pointer, and the
		// text and its padding are 33.
		dims := layout.Inset{Left: 12, Right: 12}.Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = c.Dp(touchTarget)
			var icon icons.Icon
			if index < len(p.Icons) {
				icon = p.Icons[index]
			}
			// Centred in the height, whatever the tab holds.
			words := layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return drawText(inner.With(gtx), label, unit.Sp(c.Theme.Type.Body), ink, 1, text.Middle, face)
			})
			if icon.IsZero() {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, words)
			}
			side := c.Dp(unit.Dp(c.Theme.Type.Body))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					DrawIcon(inner.With(gtx), icon, side, ink)
					return layout.Dimensions{Size: image.Pt(side, side)}
				}),
				layout.Rigid(layout.Spacer{Width: 6}.Layout),
				words,
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
