package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/pointer"
)

// TestADialogClosedByAPressOutsideOpensAgainAtTheFirstTry is the tap on the
// scrim and then the button that opens the dialog again. The dialog closed on
// the press and the release of that same tap stayed queued for the barrier, so
// the next open read it as a second press outside and closed at once: the
// person had to press the button twice.
func TestADialogClosedByAPressOutsideOpensAgainAtTheFirstTry(t *testing.T) {
	for _, src := range []pointer.Source{pointer.Touch, pointer.Mouse} {
		p, d := newPanel(), new(dialogScreen)
		d.dialog.Open()
		p.draw(d.draw)

		outside := f32.Pt(4, 4)
		p.router.Queue(pointer.Event{Kind: pointer.Press, Source: src, Buttons: buttonsFor(src), PointerID: 1, Position: outside})
		p.draw(d.draw)
		p.router.Queue(pointer.Event{Kind: pointer.Release, Source: src, PointerID: 1, Position: outside})
		p.draw(d.draw)
		if d.dialog.Showing() {
			t.Fatalf("%v: a tap outside left the dialog open", src)
		}
		d.dialog.Dismissed()

		d.dialog.Open()
		p.draw(d.draw)
		p.draw(d.draw)
		if !d.dialog.Showing() {
			t.Errorf("%v: the dialog opened again closed at once", src)
		}
	}
}

// TestAPressThatStartsOutsideAndEndsOnThePanelDoesNotClose is the drag that
// begins on the scrim: only a whole tap outside dismisses.
func TestAPressThatStartsOutsideAndEndsOnThePanelDoesNotClose(t *testing.T) {
	p, d := newPanel(), new(dialogScreen)
	d.dialog.Open()
	p.draw(d.draw)

	p.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, PointerID: 1, Position: f32.Pt(4, 4)})
	p.draw(d.draw)
	if !d.dialog.Showing() {
		t.Fatal("the press alone, before the release, closed the dialog")
	}
}

func buttonsFor(src pointer.Source) pointer.Buttons {
	if src == pointer.Mouse {
		return pointer.ButtonPrimary
	}
	return 0
}
