package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// inWindow is a frame drawn the way the window draws one: with the layer that
// covers it, and with strips under the system bars that are not part of the
// room the screen was given. The screen's origin is the corner of that room, so
// the strip under the status bar is above it.
func (p *panel) inWindow(top int, screen func(ayra.Context) ayra.Dimensions) ayra.Dimensions {
	p.ops.Reset()
	gtx := layout.Context{
		Ops:         p.ops,
		Source:      p.router.Source(),
		Metric:      unitMetric(),
		Constraints: layout.Exact(p.size),
	}
	overlay := &ayra.Overlay{
		Window: image.Rect(0, -top, p.size.X, p.size.Y),
		Safe:   image.Rectangle{Max: p.size},
	}
	c := ayra.Context{Context: gtx, Theme: theme.New(theme.Light), Shaper: p.shaper, Overlay: overlay}
	dims := screen(c)
	overlay.Flush(p.ops)
	p.router.Frame(p.ops)
	return dims
}

// card is a dialog opened from a small box in a corner of the screen, which is
// where a dialog is opened from: a row of a list, a card.
type card struct {
	dialog widget.Dialog
	inside widget.Button
	page   [2]widget.Button
	clicks [4]int
}

func (k *card) draw(c ayra.Context) ayra.Dimensions {
	for i := range k.page {
		if k.page[i].Clicked(c) {
			k.clicks[i]++
		}
	}
	if k.inside.Clicked(c) {
		k.clicks[2]++
	}
	layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widget.ButtonProps{Label: "Um"}.Layout(c.With(gtx), &k.page[0])
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widget.ButtonProps{Label: "Dois"}.Layout(c.With(gtx), &k.page[1])
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// The box the dialog is called from: 100 by 100.
			gtx.Constraints = layout.Constraints{Max: image.Pt(100, 100)}
			return widget.DialogProps{Modal: true, Dismissible: true, Width: 200}.Layout(c.With(gtx), &k.dialog,
				func(c ayra.Context) ayra.Dimensions {
					return widget.ButtonProps{Label: "Aplicar"}.Layout(c, &k.inside)
				})
		}),
	)
	return ayra.Dimensions{}
}

// TestADialogCalledFromASmallBoxCoversTheWholeWindow: the scrim covered only the
// room the dialog was given, so a dialog opened from a card left the application
// bar and the tab bar outside it -- readable, pressable, and not behind
// anything.
func TestADialogCalledFromASmallBoxCoversTheWholeWindow(t *testing.T) {
	for _, at := range []image.Point{{380, 390}, {300, 200}, {200, -20}} {
		p, k := phone(400), new(card)
		k.dialog.Open()
		p.inWindow(40, k.draw)

		p.router.Queue(
			pointerPress(at),
			pointerRelease(at),
		)
		p.inWindow(40, k.draw)
		p.inWindow(40, k.draw)

		if k.dialog.Showing() {
			t.Errorf("a press at %v, far from the box the dialog was called from, did not reach the scrim", at)
		}
	}
}

// TestTheDialogIsCentredInTheRoomTheScreenWasGiven: the panel is centred where
// the application is, and not in the box that opened it.
func TestTheDialogIsCentredInTheRoomTheScreenWasGiven(t *testing.T) {
	p, k := phone(400), new(card)
	k.dialog.Open()
	p.inWindow(40, k.draw)

	// The middle of a 390 by 400 room.
	p.tap2(image.Pt(195, 200), func(c ayra.Context) ayra.Dimensions { return k.draw(c) })

	if k.clicks[2] != 1 {
		t.Errorf("the button in the dialog answered %d times to a press in the middle of the window, want 1", k.clicks[2])
	}
	if !k.dialog.Showing() {
		t.Error("a press on the dialog's own button closed it")
	}
}

// TestTabStaysInsideAModalDialog: the keyboard cannot walk out of a modal panel
// into the page the scrim covers.
func TestTabStaysInsideAModalDialog(t *testing.T) {
	p, k := phone(400), new(card)
	k.dialog.Open()
	p.inWindow(40, k.draw)
	p.inWindow(40, k.draw)

	for range 8 {
		p.router.MoveFocus(key.FocusForward)
		p.inWindow(40, k.draw)
		p.router.Queue(
			key.Event{Name: key.NameReturn, State: key.Press},
			key.Event{Name: key.NameReturn, State: key.Release},
		)
		p.inWindow(40, k.draw)
		p.inWindow(40, k.draw)
	}
	if k.clicks[0]+k.clicks[1] != 0 {
		t.Errorf("Tab reached the page under the scrim: %d presses on its buttons", k.clicks[0]+k.clicks[1])
	}
	if k.clicks[2] == 0 {
		t.Error("Tab never reached the button inside the dialog")
	}
}

// TestShiftTabStaysInsideAModalDialogToo is the same walk backwards.
func TestShiftTabStaysInsideAModalDialogToo(t *testing.T) {
	p, k := phone(400), new(card)
	k.dialog.Open()
	p.inWindow(40, k.draw)
	p.inWindow(40, k.draw)

	for range 8 {
		p.router.MoveFocus(key.FocusBackward)
		p.inWindow(40, k.draw)
		p.router.Queue(
			key.Event{Name: key.NameReturn, State: key.Press},
			key.Event{Name: key.NameReturn, State: key.Release},
		)
		p.inWindow(40, k.draw)
		p.inWindow(40, k.draw)
	}
	if k.clicks[0]+k.clicks[1] != 0 {
		t.Errorf("Shift-Tab reached the page under the scrim: %d presses on its buttons", k.clicks[0]+k.clicks[1])
	}
}

func unitMetric() unit.Metric { return unit.Metric{PxPerDp: 1, PxPerSp: 1} }

func pointerPress(at image.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(float32(at.X), float32(at.Y))}
}

func pointerRelease(at image.Point) pointer.Event {
	return pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(float32(at.X), float32(at.Y))}
}

// tap2 is a tap through frames drawn in the window.
func (p *panel) tap2(at image.Point, screen func(ayra.Context) ayra.Dimensions) {
	p.router.Queue(pointerPress(at))
	p.inWindow(40, screen)
	p.router.Queue(pointerRelease(at))
	p.inWindow(40, screen)
	p.inWindow(40, screen)
}
