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
	"github.com/arandu-io/ayra/theme"
)

// Dialog is the state half: whether it is open, and the two ways it closes.
type Dialog struct {
	open bool
	// dismissed records a close the person made themselves, so a screen asking
	// after the draw is told about it once.
	dismissed bool

	// focused records that the keyboard was taken when the dialog opened, so it
	// is taken once and not on every frame: a field inside the dialog that is
	// given the focus has to be allowed to keep it.
	focused bool

	// outside is the barrier under the scrim and inside is the panel. Two
	// separate targets, because "was this press outside the dialog" is a
	// question only the two of them together can answer: the panel is drawn
	// over the barrier, so a press it takes is one the barrier never sees.
	outside gesture.Click
	inside  gesture.Click

	// scroll is the column the content is laid out in, so a panel taller than
	// the window scrolls instead of losing its last lines -- and the button on
	// them -- off the edge.
	scroll layout.List
}

// close marks the dialog closed by the person rather than by the screen.
func (d *Dialog) close() {
	d.open = false
	d.focused = false
	d.dismissed = true
}

// Open shows the dialog.
func (d *Dialog) Open() {
	if !d.open {
		d.focused = false
		d.scroll.Position = layout.Position{}
	}
	d.open = true
}

// Close hides it.
func (d *Dialog) Close() {
	d.open = false
	d.focused = false
}

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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
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

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DialogProps) PartNames() []string { return []string{"root"} }

// Layout draws the dialog over the screen and returns the room it took, which
// is all of it.
//
// A closed dialog draws nothing and takes no room, so a screen can call this
// unconditionally -- which is what keeps the call beside the rest of the screen
// rather than inside a branch that somebody has to remember to write.
func (p DialogProps) Layout(c ayra.Context, state *Dialog, content ayra.Widget) ayra.Dimensions {
	return p.layout(c, state, content)
}

// layout draws the dialog. Its root is the panel and not the whole overlay: what
// a caller restyles on a dialog is the box, and the scrim behind it is the
// window's.
func (p DialogProps) layout(c ayra.Context, state *Dialog, content ayra.Widget) ayra.Dimensions {
	if !state.open {
		return ayra.Dimensions{}
	}

	// In a window the dialog covers the window, not the box it was called from:
	// it is recorded and drawn over the screen once the screen is done. A
	// context with no window gives it the room it was given.
	return inWindow(c, func(c ayra.Context, window image.Rectangle) ayra.Dimensions {
		return p.place(c, state, content, window)
	})
}

// inWindow runs draw over the window when there is one to draw over, and
// answers what the caller is owed: nothing, since an overlay takes no room in
// the box that opened it, or what draw answered when it drew in place.
//
// draw is handed a context whose room is the area the screen was given, whose
// origin is that area's corner, and the window as a rectangle in those
// coordinates -- the strips under the system bars are negative.
func inWindow(c ayra.Context, draw func(ayra.Context, image.Rectangle) ayra.Dimensions) ayra.Dimensions {
	if c.Overlay == nil {
		return draw(c, image.Rectangle{Max: c.Constraints.Max})
	}

	safe, window := c.Overlay.Safe, c.Overlay.Window
	inner := c
	inner.Constraints = layout.Exact(safe.Size())

	recording := op.Record(c.Ops)
	at := op.Offset(safe.Min).Push(c.Ops)
	draw(inner, window.Sub(safe.Min))
	at.Pop()
	c.Overlay.Add(recording.Stop())
	return ayra.Dimensions{}
}

// place draws the scrim over the window and the panel in the room the context
// has.
func (p DialogProps) place(c ayra.Context, state *Dialog, content ayra.Widget, window image.Rectangle) ayra.Dimensions {
	state.overlay(c, window, p.Modal, p.Modal || p.Dismissible)

	width := p.Width
	if width == 0 {
		width = 420
	}

	dims := layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		// Kyse keeps the panel 16 points from every edge of the window
		// (max-w and max-h of calc(100% - 2rem)). Without it the panel of a
		// 420-wide dialog on a phone touched both edges and the corners ran off
		// the screen.
		margin := c.Dp(dialogMargin)
		if maximum := min(c.Dp(width), gtx.Constraints.Max.X-2*margin); maximum > 0 {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, maximum)
		}
		if maximum := gtx.Constraints.Max.Y - 2*margin; maximum > 0 {
			gtx.Constraints.Max.Y = maximum
		}
		gtx.Constraints.Min = image.Point{}
		panel := c.With(gtx)

		// The panel takes the presses that land on it, so the barrier behind
		// does not read them as a press outside.
		measure := op.Record(panel.Ops)
		root := panel.Theme.Resolve("dialog", "root", p.VariantName("default"), theme.Style{
			Background: theme.Colour(panel.Theme.Colours.Popover),
			Border:     theme.Colour(panel.Theme.Colours.Border),
			// Kyse draws the panel with rounded-xl: the radius of the theme and
			// four points more. It was the radius of a control, and a panel
			// that size with the corners of a button reads as square.
			Radius:  theme.Pt(panel.Theme.Metrics.Radius + 4),
			Padding: theme.Uniform(20),
		}, p.ComponentProps)
		dims := Surface(panel, root, func(c ayra.Context) ayra.Dimensions {
			return state.scrolled(c, content)
		})
		drawn := measure.Stop()

		defer clip.Rect{Max: dims.Size}.Push(panel.Ops).Pop()
		state.inside.Add(panel.Ops)
		event.Op(panel.Ops, &state.inside)
		drawn.Add(panel.Ops)

		return dims
	})

	// Read after the content is drawn, so that something inside the panel that
	// answers the back key -- a menu open over the dialog -- is asked first and
	// the dialog closes only on the press nothing above it took.
	state.dismiss(c, p.Modal, p.Dismissible)
	return dims
}

// dialogMargin is the room Kyse leaves between a dialog and the edge of the
// window, in points.
const dialogMargin = 16

// scrolled lays the content out in a column that scrolls when it is taller than
// the room it was given, and takes only the room it needs when it is not.
func (d *Dialog) scrolled(c ayra.Context, content ayra.Widget) ayra.Dimensions {
	return scrolled(&d.scroll, c, content)
}

// scrolled is a column that scrolls, holding one widget.
func scrolled(list *layout.List, c ayra.Context, content ayra.Widget) ayra.Dimensions {
	list.Axis = layout.Vertical
	return list.Layout(c.Context, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return content(c.With(gtx))
	})
}

// overlay is what a panel that dims the screen puts behind itself: the scrim,
// the barrier that reads a press outside the panel, and the keyboard target for
// escape.
//
// It is its own clip, closed before the panel is drawn, and that is the whole
// point of the function. A pointer handler contains the handlers declared
// inside its area -- the walk that routes a press goes from the innermost
// handler out through every one that encloses it. A panel opened inside the
// barrier's area is therefore a control inside the barrier, and a press on it
// reached the barrier too, as a press outside: the dialog closed on the tap that
// was meant for its own button, before the button answered. Drawn as a sibling,
// the panel is not enclosed by the barrier, and a press on it never reaches it.
func (d *Dialog) overlay(c ayra.Context, area image.Rectangle, modal, listening bool) {
	defer clip.Rect(area).Push(c.Ops).Pop()

	if modal {
		// The scrim, and the barrier behind it. Painting one without the other
		// is the mistake that produces a dialog somebody can click straight
		// through: it looks modal and is not.
		paint.FillShape(c.Ops, scrim(), clip.Rect(area).Op())
		d.outside.Add(c.Ops)
		event.Op(c.Ops, &d.outside)
	}
	if listening {
		// The target of the keys read in dismiss, declared inside the barrier's
		// area: a key handler outside every area is one the router never asks.
		event.Op(c.Ops, d)
	}
}

// dismiss reads what closes the dialog: escape, the back key, and a press that
// landed on the barrier rather than on the panel.
//
// A modal panel always takes the back key, even when it may not be dismissed:
// on Android an unanswered back press falls through to the activity and closes
// the application, and a question that must be answered is not something the
// back key should walk away from.
func (d *Dialog) dismiss(c ayra.Context, modal, dismissible bool) {
	// The dialog takes the keyboard when it opens, so nothing behind it reads a
	// keystroke. It takes it once: every frame would take it back from the field
	// the person just touched inside the sheet, and the field would never keep
	// what is typed. Escape is read whatever holds the focus, for the same
	// reason -- it has to close the dialog from inside a field as well.
	if modal || dismissible {
		// The panel is a stop of its own, and a modal one is the trap: Tab
		// walks what is declared from here on, which is the panel's content,
		// and never the page the scrim covers. Reading the filter is what
		// asks for it.
		for {
			if _, ok := c.Event(key.FocusFilter{Target: d, Trap: modal}); !ok {
				break
			}
		}
		if !d.focused {
			d.focused = true
			c.Execute(key.FocusCmd{Tag: d})
		}
	}
	if dismissible || modal {
		for {
			e, ok := c.Event(
				key.Filter{Name: key.NameEscape},
				key.Filter{Name: key.NameBack},
			)
			if !ok {
				break
			}
			if press, isKey := e.(key.Event); isKey && press.State == key.Press && dismissible {
				d.close()
			}
		}
	}

	// A press that reached the barrier is a press outside the panel, because
	// the panel is drawn over it and takes its own.
	for modal {
		if _, ok := d.outside.Update(c.Source); !ok {
			break
		}
		if dismissible {
			d.close()
		}
	}

	// Presses on the panel are consumed so they do not reach anything behind,
	// and so the barrier does not see them.
	for {
		if _, ok := d.inside.Update(c.Source); !ok {
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
