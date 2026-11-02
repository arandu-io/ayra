package widget_test

import (
	"image"
	"strconv"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// windowed draws a frame the way the window does: the screen is written offset by
// the status bar, the layer knows the whole window, and the router has the last
// frame's places to answer with. Presses are in the window's own coordinates.
func (p *panel) windowed(status int, screen func(ayra.Context) ayra.Dimensions) {
	p.ops.Reset()
	op.Offset(image.Pt(0, status)).Add(p.ops)
	room := image.Pt(p.size.X, p.size.Y-status)
	gtx := layout.Context{Ops: p.ops, Source: p.router.Source(), Metric: unitMetric(), Constraints: layout.Exact(room)}
	overlay := &ayra.Overlay{Window: image.Rect(0, -status, p.size.X, p.size.Y-status), Safe: image.Rectangle{Max: room}}
	screen(ayra.Context{Context: gtx, Theme: theme.New(theme.Light), Shaper: p.shaper, Overlay: overlay})
	overlay.Flush(p.ops)
	p.router.Frame(p.ops)
}

func (p *panel) press(at image.Point, screen func(ayra.Context) ayra.Dimensions) {
	for _, e := range []pointer.Event{pointerPress(at), pointerRelease(at)} {
		p.router.Queue(e)
		p.windowed(24, screen)
	}
	p.windowed(24, screen)
}

// placed is a menu whose control sits at a chosen height and a chosen side.
type placed struct {
	menu    widget.Menu
	trigger widget.Button
	top     int
	right   bool
	entries int
	end     bool
}

func (m *placed) draw(c ayra.Context) ayra.Dimensions {
	entries := make([]string, m.entries)
	for i := range entries {
		entries[i] = "Entrada " + strconv.Itoa(i)
	}
	c.Constraints.Min = image.Point{}
	return layout.Inset{Top: unitDp(m.top), Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				w := 0
				if m.right {
					w = gtx.Constraints.Max.X
				}
				return layout.Dimensions{Size: image.Pt(w, 0)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.MenuProps{Entries: entries, Width: 180, End: m.end}.Layout(c.With(gtx), &m.menu, func(c ayra.Context) ayra.Dimensions {
					return widget.ButtonProps{Label: "Ações"}.Layout(c, &m.trigger)
				})
			}),
		)
	})
}

func unitDp(v int) unit.Dp { return unit.Dp(v) }

func settleMenu(p *panel, m *placed) {
	for range 3 {
		p.windowed(24, m.draw)
	}
}

// TestAMenuNearTheBottomOpensAboveItsControl is Kyse's flip: the list under a
// control at the foot of the window ran off the screen, where nothing in it could
// be pressed.
func TestAMenuNearTheBottomOpensAboveItsControl(t *testing.T) {
	p, m := phone(600), &placed{top: 470, entries: 3}
	m.menu.Open()
	settleMenu(p, m)

	// Just over the control, where the last entry of a list above it is. The
	// control is at 494 in the window.
	p.press(image.Pt(60, 494-4-4-16), m.draw)

	if got := m.menu.Chosen(); got != 2 {
		t.Errorf("a press on the last entry over the control chose %d, want 2: the list did not open above", got)
	}
}

// TestAMenuWithRoomBelowStaysBelow keeps the other half.
func TestAMenuWithRoomBelowStaysBelow(t *testing.T) {
	p, m := phone(600), &placed{top: 40, entries: 3}
	m.menu.Open()
	settleMenu(p, m)

	// Under the control, at 40+24=64 to 100 in the window.
	p.press(image.Pt(60, 64+36+4+4+16), m.draw)

	if got := m.menu.Chosen(); got != 0 {
		t.Errorf("a press on the first entry under the control chose %d, want 0", got)
	}
}

// TestAListLongerThanTheRoomIsHeldToItAndScrolls: forty entries ran past the
// bottom of the window with no way to reach the last.
func TestAListLongerThanTheRoomIsHeldToItAndScrolls(t *testing.T) {
	p, m := phone(600), &placed{top: 40, entries: 40}
	m.menu.Open()
	settleMenu(p, m)

	p.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(100, 300), Scroll: f32.Pt(0, 40000)})
	settleMenu(p, m)

	// The foot of the window is the foot of the list now.
	p.press(image.Pt(60, 600-8), m.draw)

	if got := m.menu.Chosen(); got != 39 {
		t.Errorf("a press at the foot of the window, with the list scrolled to its end, chose %d, want 39", got)
	}
}

// TestAMenuAtTheRightEdgeStaysInsideTheWindowWithoutAsking: the list is kept off
// the sides of the window whether or not the caller said End.
func TestAMenuAtTheRightEdgeStaysInsideTheWindowWithoutAsking(t *testing.T) {
	p, m := phone(600), &placed{top: 40, entries: 3, right: true}
	m.menu.Open()
	settleMenu(p, m)

	// 150 points left of the control: inside a 180-wide list that was kept in the
	// window, nowhere near one that started at the control.
	p.press(image.Pt(390-16-150, 64+36+4+4+16), m.draw)

	if got := m.menu.Chosen(); got != 0 {
		t.Errorf("the list was not kept inside the window: chose %d", got)
	}
}
