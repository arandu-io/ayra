package widget

import (
	"image"

	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op/clip"
)

// Decorations owns the interaction state of the project's titlebar.
//
// The project draws the titlebar so its controls share the behavior and visual
// language of the rest of the application on every supported platform.
type Decorations struct {
	// Maximized controls the look and behaviour of the maximize button. The
	// caller keeps it synchronized with the window state reported by the app.
	Maximized bool

	clicks map[system.Action]*Clickable
}

// LayoutMove lays out content and marks its area as a window drag handle.
func (d *Decorations) LayoutMove(gtx layout.Context, content layout.Widget) layout.Dimensions {
	dimensions := content(gtx)
	defer clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops).Pop()
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
	return dimensions
}

// Clickable returns the stable clickable associated with one window action.
func (d *Decorations) Clickable(action system.Action) *Clickable {
	if action == 0 || action&(action-1) != 0 {
		panic("window decoration requires one action")
	}
	if d.clicks == nil {
		d.clicks = make(map[system.Action]*Clickable)
	}
	if click := d.clicks[action]; click != nil {
		return click
	}

	click := new(Clickable)
	d.clicks[action] = click
	return click
}

// Update returns every window action activated since the previous update.
func (d *Decorations) Update(gtx layout.Context) system.Action {
	var activated system.Action
	for action, click := range d.clicks {
		if click.Clicked(gtx) {
			activated |= d.actionForState(action)
		}
	}
	return activated
}

func (d *Decorations) actionForState(action system.Action) system.Action {
	switch {
	case action == system.ActionMaximize && d.Maximized:
		return system.ActionUnmaximize
	case action == system.ActionUnmaximize && !d.Maximized:
		return system.ActionMaximize
	default:
		return action
	}
}
