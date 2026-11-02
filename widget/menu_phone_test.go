package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/widget"
)

// rowMenu is the "..." at the end of a row: a menu at the right edge of a
// phone, with a button in the line under it that the open list lies over.
type rowMenu struct {
	menu    widget.Menu
	trigger widget.Button
	below   widget.Button
	below2  int
	end     bool
}

func (r *rowMenu) draw(c ayra.Context) ayra.Dimensions {
	if r.below.Clicked(c) {
		r.below2++
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					return widget.MenuProps{Entries: []string{"Editar", "Excluir"}, End: r.end}.Layout(inner, &r.menu,
						func(c ayra.Context) ayra.Dimensions {
							return widget.ButtonProps{Label: "Mais"}.Layout(c, &r.trigger)
						})
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min.X = inner.Constraints.Max.X
			return widget.ButtonProps{Label: "Abaixo"}.Layout(inner, &r.below)
		}),
	)
}

// TestAMenuOpenOverTheNextLineTakesThePressMeantForIt: the list was drawn in
// layout order, so the button in the line under it was drawn over the first
// entry and answered the press.
func TestAMenuOpenOverTheNextLineTakesThePressMeantForIt(t *testing.T) {
	p, r := phone(400), &rowMenu{end: true}
	r.menu.Open()
	p.draw(r.draw)

	// The first entry lies over the "Abaixo" button, a few points under the
	// trigger and against the right edge.
	p.tap(image.Pt(370, 52), r.draw)

	if got := r.menu.Chosen(); got != 0 {
		t.Errorf("a press on the first entry chose %d, want 0", got)
	}
	if r.below2 != 0 {
		t.Errorf("the button under the list answered %d presses meant for the entry", r.below2)
	}
}

// TestAMenuAtTheRightEdgeOpensInsideTheWindow is the "..." at the end of a row:
// the list started at the control's left edge and ran off the screen.
func TestAMenuAtTheRightEdgeOpensInsideTheWindow(t *testing.T) {
	p, r := phone(400), &rowMenu{end: true}
	r.menu.Open()
	p.draw(r.draw)

	// 150 points from the right edge, which is inside a 180-wide list that ends
	// where the control does, and nowhere near a list that starts at the control.
	p.tap(image.Pt(250, 52), r.draw)

	if got := r.menu.Chosen(); got != 0 {
		t.Errorf("the list does not reach 150 points left of the control: chose %d", got)
	}
}

// TestAPressOutsideTheMenuPutsItAway is the tap on the page that closes it.
func TestAPressOutsideTheMenuPutsItAway(t *testing.T) {
	p, r := phone(400), &rowMenu{end: true}
	r.menu.Open()
	p.draw(r.draw)

	p.tap(image.Pt(40, 300), r.draw)

	if r.menu.Showing() {
		t.Error("a press outside left the menu open")
	}
	if got := r.menu.Chosen(); got != -1 {
		t.Errorf("a press outside chose entry %d", got)
	}
}

// TestAPressOnThePanelBetweenEntriesDoesNotClose keeps the other half.
func TestAPressOnThePanelBetweenEntriesDoesNotClose(t *testing.T) {
	p, r := phone(400), &rowMenu{end: true}
	r.menu.Open()
	p.draw(r.draw)

	// The panel's own padding, between the last entry and the edge of the panel.
	p.tap(image.Pt(388, 50), r.draw)

	if !r.menu.Showing() {
		t.Error("a press on the panel's padding closed the menu")
	}
}

// TestTheBackKeyAndEscapePutTheMenuAway: the back button closes the list that is
// on top before it closes anything under it.
func TestTheBackKeyAndEscapePutTheMenuAway(t *testing.T) {
	for _, name := range []key.Name{key.NameBack, key.NameEscape} {
		p, r := phone(400), &rowMenu{end: true}
		r.menu.Open()
		p.draw(r.draw)

		p.router.Queue(key.Event{Name: name, State: key.Press})
		p.draw(r.draw)

		if r.menu.Showing() {
			t.Errorf("%s left the menu open", name)
		}
	}
}

// TestTheBackKeyClosesTheMenuOpenOverADialogFirst is the order of the stack: the
// press is read by what is on top, and the dialog under it stays.
func TestTheBackKeyClosesTheMenuOpenOverADialogFirst(t *testing.T) {
	p := phone(400)
	var dialog widget.Dialog
	var menu widget.Menu
	var trigger widget.Button
	dialog.Open()
	menu.Open()
	screen := func(c ayra.Context) ayra.Dimensions {
		c.Constraints.Min = c.Constraints.Max
		return widget.DialogProps{Modal: true, Dismissible: true, Width: 300}.Layout(c, &dialog, func(c ayra.Context) ayra.Dimensions {
			return widget.MenuProps{Entries: []string{"Editar"}}.Layout(c, &menu, func(c ayra.Context) ayra.Dimensions {
				return widget.ButtonProps{Label: "Mais"}.Layout(c, &trigger)
			})
		})
	}
	p.draw(screen)

	p.router.Queue(key.Event{Name: key.NameBack, State: key.Press})
	p.draw(screen)

	if menu.Showing() {
		t.Error("the back key left the menu open")
	}
	if !dialog.Showing() {
		t.Error("the back key closed the dialog under the menu with it")
	}

	p.router.Queue(key.Event{Name: key.NameBack, State: key.Press})
	p.draw(screen)
	if dialog.Showing() {
		t.Error("the second back key did not close the dialog")
	}
}
