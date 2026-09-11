package widget

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/unit"
)

// Dialog is the state half: whether it is open, and the two ways it closes.
type Dialog struct {
	open bool
	// dismissed records a close the person made themselves, so a screen asking
	// after the draw is told about it once.
	dismissed bool

	// outside is the barrier under the scrim and inside is the panel. Two
	// separate targets, because "was this press outside the dialog" is a
	// question only the two of them together can answer: the panel is drawn
	// over the barrier, so a press it takes is one the barrier never sees.
	outside gesture.Click
	inside  gesture.Click
}

// close marks the dialog closed by the person rather than by the screen.
func (d *Dialog) close() {
	d.open = false
	d.dismissed = true
}

// Open shows the dialog.
func (d *Dialog) Open() { d.open = true }

// Close hides it.
func (d *Dialog) Close() { d.open = false }

// Showing reports whether it is on screen.
func (d *Dialog) Showing() bool { return d.open }

// Dismissed reports that the person closed it themselves -- with the escape key
// or by pressing outside it -- and consumes that.
//
// It is separate from Close because the two mean different things to a screen:
// a dialog closed by its own button has an answer, and one dismissed has none.
// A screen that treated them alike would save a form nobody confirmed.
func (d *Dialog) Dismissed() bool {
	was := d.dismissed
	d.dismissed = false
	return was
}

// DialogProps is what a dialog is drawn from.
type DialogProps struct {
	// Width is how wide the panel may get. Zero takes a width a form reads
	// comfortably at.
	Width unit.Dp

	// Modal draws the scrim and swallows what is behind it. A dialog that is
	// not modal is a panel, and a panel does not belong on top of a screen.
	//
	// It is a field rather than an assumption because the scrim is the
	// expensive half: it covers the application, and something that covers the
	// application should be asked for rather than inherited.
	Modal bool

	// Dismissible lets escape close it, and a press outside close it when Modal
	// is set as well: the press outside is read from the barrier the scrim puts
	// up, and without one there is nothing to read. A dialog asking
	// something that cannot be left unanswered sets this false and gives the
	// person a button instead.
	Dismissible bool
}

// Layout draws the dialog over the screen and returns the room it took, which
// is all of it.
//
// A closed dialog draws nothing and takes no room, so a screen can call this
// unconditionally -- which is what keeps the call beside the rest of the screen
// rather than inside a branch that somebody has to remember to write.
func (p DialogProps) Layout(c ayra.Context, state *Dialog, content ayra.Widget) ayra.Dimensions {
	if !state.open {
		return ayra.Dimensions{}
	}

	area := c.Constraints.Max
	defer clip.Rect{Max: area}.Push(c.Ops).Pop()

	if p.Modal {
		// The scrim, and the barrier behind it. Painting one without the other
		// is the mistake that produces a dialog somebody can click straight
		// through: it looks modal and is not.
		paint.FillShape(c.Ops, scrim(), clip.Rect{Max: area}.Op())
		state.outside.Add(c.Ops)
		event.Op(c.Ops, &state.outside)
	}

	if p.Dismissible {
		p.readDismissal(c, state)
	}

	width := p.Width
	if width == 0 {
		width = 420
	}

	return layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		if maximum := c.Dp(width); gtx.Constraints.Max.X > maximum {
			gtx.Constraints.Max.X = maximum
		}
		gtx.Constraints.Min = image.Point{}
		panel := c.With(gtx)

		// The panel takes the presses that land on it, so the barrier behind
		// does not read them as a press outside.
		measure := op.Record(panel.Ops)
		dims := surface(panel, panel.Theme.Colours.Popover, panel.Theme.Colours.Border, controlRadius(panel), func(c ayra.Context) ayra.Dimensions {
			return layout.UniformInset(20).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return content(c.With(gtx))
			})
		})
		drawn := measure.Stop()

		defer clip.Rect{Max: dims.Size}.Push(panel.Ops).Pop()
		state.inside.Add(panel.Ops)
		event.Op(panel.Ops, &state.inside)
		drawn.Add(panel.Ops)

		return dims
	})
}

// readDismissal closes the dialog on escape, or on a press that landed on the
// barrier rather than on the panel.
func (p DialogProps) readDismissal(c ayra.Context, state *Dialog) {
	// The dialog takes the keyboard while it is up, which is both halves of
	// being modal: escape reaches it, and nothing behind it reads a keystroke.
	c.Execute(key.FocusCmd{Tag: state})
	event.Op(c.Ops, state)

	for {
		e, ok := c.Event(
			key.Filter{Focus: state, Name: key.NameEscape},
		)
		if !ok {
			break
		}
		if press, isKey := e.(key.Event); isKey && press.State == key.Press {
			state.close()
		}
	}

	// A press that reached the barrier is a press outside the panel, because
	// the panel is drawn over it and takes its own.
	for {
		if _, ok := state.outside.Update(c.Source); !ok {
			break
		}
		state.close()
	}

	// Presses on the panel are consumed so they do not reach anything behind,
	// and so the barrier does not see them.
	for {
		if _, ok := state.inside.Update(c.Source); !ok {
			break
		}
	}
}

// scrim is the wash over what the dialog covers.
//
// Dark in both schemes rather than derived from the palette: what it does is
// take light away from everything behind it, and a light scrim on a light
// screen dims nothing while still swallowing every press -- which is a screen
// that stopped responding for no visible reason.
func scrim() color.NRGBA {
	return color.NRGBA{A: 140}
}

// registerTarget makes a gesture the thing a press at this point reaches.
//
// Two calls that always go together, and a pair that is easy to write half of:
// adding the gesture without the tag registers an area nothing is listening
// for, which swallows the press and reports nothing.
func registerTarget(c ayra.Context, tag any) {
	event.Op(c.Ops, tag)
}
