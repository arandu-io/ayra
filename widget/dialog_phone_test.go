package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/widget"
)

// A phone is 390 points wide, the keyboard takes more than half of what is
// tall, and the back key is a button the person can press at any time. These
// tests fix what a dialog and a drawer do under those three.

// phone is a panel the size of a phone's window.
func phone(height int) *panel {
	p := newPanel()
	p.size = image.Pt(390, height)
	return p
}

// TestTheBackKeyClosesTheDialogAndIsTaken is Android's back button: the
// dialog on top closes, and the press is reported as taken, because one that
// is not falls through to the activity and closes the application.
func TestTheBackKeyClosesTheDialogAndIsTaken(t *testing.T) {
	p, d := newPanel(), new(dialogScreen)
	d.dialog.Open()
	p.draw(d.draw)

	p.router.Queue(key.Event{Name: key.NameBack, State: key.Press})
	p.draw(d.draw)

	if d.dialog.Showing() {
		t.Error("the back key left the dialog open")
	}
	if !d.dialog.Dismissed() {
		t.Error("the close was not reported as the person's")
	}
}

// TestTheBackKeyClosesTheDrawer is the same press on a sheet.
func TestTheBackKeyClosesTheDrawer(t *testing.T) {
	p, d := newPanel(), new(drawerScreen)
	d.drawer.Open()
	p.draw(d.draw)

	p.router.Queue(key.Event{Name: key.NameBack, State: key.Press})
	p.draw(d.draw)

	if d.drawer.Showing() {
		t.Error("the back key left the drawer open")
	}
}

// answerScreen is a modal dialog that may not be dismissed.
type answerScreen struct {
	dialog widget.Dialog
	button widget.Button
}

func (a *answerScreen) draw(c ayra.Context) ayra.Dimensions {
	a.button.Clicked(c)
	c.Constraints.Min = c.Constraints.Max
	return widget.DialogProps{Modal: true, Width: 200}.Layout(c, &a.dialog, func(c ayra.Context) ayra.Dimensions {
		return widget.ButtonProps{Label: "Remover"}.Layout(c, &a.button)
	})
}

// TestTheBackKeyDoesNotWalkAwayFromAQuestionThatMustBeAnswered keeps the
// dialog open, and still takes the press: one left over closes the application.
func TestTheBackKeyDoesNotWalkAwayFromAQuestionThatMustBeAnswered(t *testing.T) {
	p, a := newPanel(), new(answerScreen)
	a.dialog.Open()
	p.draw(a.draw)

	p.router.Queue(key.Event{Name: key.NameBack, State: key.Press})
	p.draw(a.draw)

	if !a.dialog.Showing() {
		t.Error("the back key closed a dialog that cannot be dismissed")
	}
}

// wideDialog is a dialog asking for 420 points, the width it takes by default.
type wideDialog struct {
	dialog widget.Dialog
}

func (d *wideDialog) draw(c ayra.Context) ayra.Dimensions {
	c.Constraints.Min = c.Constraints.Max
	return widget.DialogProps{Modal: true, Dismissible: true}.Layout(c, &d.dialog, func(c ayra.Context) ayra.Dimensions {
		c.Constraints.Min.X = c.Constraints.Max.X
		return widget.ButtonProps{Label: "Remover"}.Layout(c, new(widget.Button))
	})
}

// TestTheDialogKeepsSixteenPointsFromTheEdgesOfAPhone is Kyse's
// max-width of calc(100% - 2rem): on a 390-wide window the panel was 380 wide
// and its corners ran to the edge, where a press 10 points in was a press on
// the panel and not one outside it.
func TestTheDialogKeepsSixteenPointsFromTheEdgesOfAPhone(t *testing.T) {
	p, d := phone(700), new(wideDialog)
	d.dialog.Open()
	p.draw(d.draw)

	// Inside the margin, level with the middle of the panel.
	p.tap(image.Pt(10, 350), d.draw)

	if d.dialog.Showing() {
		t.Error("a press 10 points from the edge landed on the panel: the dialog has no margin")
	}
}

// tallDialog holds more lines than a phone's window has room for, and a button
// after the last of them.
type tallDialog struct {
	dialog  widget.Dialog
	buttons [20]widget.Button
	pressed int
	height  int
}

func (d *tallDialog) draw(c ayra.Context) ayra.Dimensions {
	for i := range d.buttons {
		if d.buttons[i].Clicked(c) && i == len(d.buttons)-1 {
			d.pressed++
		}
	}
	c.Constraints.Min = c.Constraints.Max
	return widget.DialogProps{Modal: true, Dismissible: true}.Layout(c, &d.dialog, func(c ayra.Context) ayra.Dimensions {
		children := make([]layout.FlexChild, 0, len(d.buttons))
		for i := range d.buttons {
			i := i
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min.X = inner.Constraints.Max.X
				return widget.ButtonProps{Label: "Linha"}.Layout(inner, &d.buttons[i])
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
	})
}

// TestADialogTallerThanTheWindowScrollsToItsLastButton is the dialog with the
// keyboard up: the panel was as tall as its content and the window cut off what
// did not fit, so the button at the end could not be reached by any means.
func TestADialogTallerThanTheWindowScrollsToItsLastButton(t *testing.T) {
	p, d := phone(468), new(tallDialog)
	d.dialog.Open()
	p.draw(d.draw)

	// The wheel over the middle of the panel.
	p.router.Queue(pointer.Event{
		Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(195, 234), Scroll: f32.Pt(0, 4000),
	})
	p.draw(d.draw)
	p.draw(d.draw)

	// The button is somewhere in the panel now: press along its middle until it
	// answers.
	for y := 20; y < 448 && d.pressed == 0; y += 4 {
		p.tap(image.Pt(195, y), d.draw)
	}
	if d.pressed == 0 {
		t.Error("the last button of a dialog taller than the window cannot be reached")
	}
	if !d.dialog.Showing() {
		t.Error("scrolling the dialog closed it")
	}
}
