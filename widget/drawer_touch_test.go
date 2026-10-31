package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/widget"
)

// tap is a finger going down and coming up at one point: the source Android
// and iOS deliver, which has no hover before the press and no buttons.
func (p *panel) tap(at image.Point, screen func(ayra.Context) ayra.Dimensions) {
	pos := f32.Pt(float32(at.X), float32(at.Y))
	p.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: pos})
	p.draw(screen)
	p.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: pos})
	p.draw(screen)
}

// drawerScreen is a modal, dismissible drawer from the bottom with one button
// in it, the shape of the data table's sheet of filters.
type drawerScreen struct {
	drawer  widget.Dialog
	button  widget.Button
	pressed int
}

func (d *drawerScreen) draw(c ayra.Context) ayra.Dimensions {
	if d.button.Clicked(c) {
		d.pressed++
	}
	return widget.DrawerProps{Side: widget.Bottom, Size: 200, Modal: true, Dismissible: true}.Layout(c, &d.drawer,
		func(c ayra.Context) ayra.Dimensions {
			return widget.ButtonProps{Label: "Aplicar"}.Layout(c, &d.button)
		})
}

// TestATouchInsideTheDrawerReachesItsButton fixes the sheet of filters: the
// "Aplicar" in a drawer answers a tap, and the tap does not count as one
// outside the panel.
func TestATouchInsideTheDrawerReachesItsButton(t *testing.T) {
	p, d := newPanel(), new(drawerScreen)
	d.drawer.Open()
	p.draw(d.draw)

	// The panel is the lower half of the 400 by 400 area, inset by 20; the
	// button is at its top left corner.
	p.tap(image.Pt(60, 230), d.draw)

	if d.pressed != 1 {
		t.Errorf("the button inside the drawer answered %d times to a tap, want 1", d.pressed)
	}
	if !d.drawer.Showing() {
		t.Error("a tap on the drawer's own button closed the drawer: it counted as outside")
	}
	if d.drawer.Dismissed() {
		t.Error("a tap inside the drawer was recorded as the person dismissing it")
	}
}

// TestATouchOnTheEmptyPartOfThePanelDoesNotClose is the inside that is not a
// control.
func TestATouchOnTheEmptyPartOfThePanelDoesNotClose(t *testing.T) {
	p, d := newPanel(), new(drawerScreen)
	d.drawer.Open()
	p.draw(d.draw)

	p.tap(image.Pt(300, 380), d.draw)

	if !d.drawer.Showing() {
		t.Error("a tap on the panel's empty part closed the drawer")
	}
}

// TestATouchOnTheScrimClosesTheDrawer is the press outside.
func TestATouchOnTheScrimClosesTheDrawer(t *testing.T) {
	p, d := newPanel(), new(drawerScreen)
	d.drawer.Open()
	p.draw(d.draw)

	p.tap(image.Pt(200, 50), d.draw)

	if d.drawer.Showing() {
		t.Error("a tap on the scrim left the drawer open")
	}
	if !d.drawer.Dismissed() {
		t.Error("the close was not reported as the person's")
	}
	if d.pressed != 0 {
		t.Errorf("a tap outside reached the button inside: %d", d.pressed)
	}
}

// dialogScreen is a modal, dismissible dialog with one button in it.
type dialogScreen struct {
	dialog  widget.Dialog
	button  widget.Button
	pressed int
}

func (d *dialogScreen) draw(c ayra.Context) ayra.Dimensions {
	if d.button.Clicked(c) {
		d.pressed++
	}
	// A window hands a screen its whole area as the least it may take, and the
	// dialog centres itself in that.
	c.Constraints.Min = c.Constraints.Max
	return widget.DialogProps{Modal: true, Dismissible: true, Width: 200}.Layout(c, &d.dialog,
		func(c ayra.Context) ayra.Dimensions {
			return widget.ButtonProps{Label: "Aplicar"}.Layout(c, &d.button)
		})
}

// TestATouchInsideTheDialogReachesItsButton is the drawer's test for the other
// panel that dims the screen: the dialog is centred, so its button is in the
// middle of the area.
func TestATouchInsideTheDialogReachesItsButton(t *testing.T) {
	p, d := newPanel(), new(dialogScreen)
	d.dialog.Open()
	p.draw(d.draw)

	p.tap(image.Pt(200, 200), d.draw)

	if d.pressed != 1 {
		t.Errorf("the button inside the dialog answered %d times to a tap, want 1", d.pressed)
	}
	if !d.dialog.Showing() {
		t.Error("a tap on the dialog's own button closed the dialog: it counted as outside")
	}
}

// TestATouchOnTheDialogScrimClosesIt is the press outside the dialog.
func TestATouchOnTheDialogScrimClosesIt(t *testing.T) {
	p, d := newPanel(), new(dialogScreen)
	d.dialog.Open()
	p.draw(d.draw)

	p.tap(image.Pt(10, 10), d.draw)

	if d.dialog.Showing() {
		t.Error("a tap on the scrim left the dialog open")
	}
}

// fieldDrawer is a dismissible drawer with a text field in it: the sheet of
// filters has several.
type fieldDrawer struct {
	drawer widget.Dialog
	input  widget.Input
}

func (d *fieldDrawer) draw(c ayra.Context) ayra.Dimensions {
	return widget.DrawerProps{Side: widget.Bottom, Size: 200, Modal: true, Dismissible: true}.Layout(c, &d.drawer,
		func(c ayra.Context) ayra.Dimensions {
			return widget.InputProps{}.Layout(c, &d.input)
		})
}

// TestADismissibleDrawerLetsItsFieldKeepTheKeyboard fixes the other half of a
// dismissible panel: it took the keyboard on every frame, so the field a person
// touched inside it lost the focus at once and never received what was typed.
func TestADismissibleDrawerLetsItsFieldKeepTheKeyboard(t *testing.T) {
	p, d := newPanel(), new(fieldDrawer)
	d.drawer.Open()
	p.draw(d.draw)

	p.tap(image.Pt(60, 235), d.draw)
	p.draw(d.draw)
	p.router.Queue(key.EditEvent{Text: "abc"})
	p.draw(d.draw)
	p.draw(d.draw)

	if got := d.input.Text(); got != "abc" {
		t.Errorf("the field inside the drawer holds %q after typing, want \"abc\"", got)
	}
	if !d.drawer.Showing() {
		t.Error("touching the field closed the drawer")
	}
}

// TestEscapeClosesADismissibleDrawerFromInsideAField keeps the key working now
// that the panel does not hold the focus itself.
func TestEscapeClosesADismissibleDrawerFromInsideAField(t *testing.T) {
	p, d := newPanel(), new(fieldDrawer)
	d.drawer.Open()
	p.draw(d.draw)
	p.tap(image.Pt(60, 235), d.draw)
	p.draw(d.draw)

	p.router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	p.draw(d.draw)
	p.draw(d.draw)

	if d.drawer.Showing() {
		t.Error("escape in a field left the drawer open")
	}
}
