package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/widget"
)

// lines is a column of buttons, the last of which counts its presses.
type lines struct {
	buttons [16]widget.Button
	last    int
}

func (l *lines) content(c ayra.Context) ayra.Dimensions {
	for i := range l.buttons {
		if l.buttons[i].Clicked(c) && i == len(l.buttons)-1 {
			l.last++
		}
	}
	children := make([]layout.FlexChild, 0, len(l.buttons))
	for i := range l.buttons {
		i := i
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min.X = inner.Constraints.Max.X
			return widget.ButtonProps{Label: "Linha"}.Layout(inner, &l.buttons[i])
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}

type scrollingDrawer struct {
	drawer widget.Dialog
	body   lines
	scroll bool
}

func (d *scrollingDrawer) draw(c ayra.Context) ayra.Dimensions {
	return widget.DrawerProps{Side: widget.Bottom, Size: 250, Modal: true, Dismissible: true, Scroll: d.scroll}.Layout(c, &d.drawer, d.body.content)
}

// TestADrawerTallerThanItsPanelScrollsWhenAskedTo: a form of sixteen lines in a
// sheet of 250 points had its last lines under the foot of the panel.
func TestADrawerTallerThanItsPanelScrollsWhenAskedTo(t *testing.T) {
	p, d := phone(600), &scrollingDrawer{scroll: true}
	d.drawer.Open()
	p.draw(d.draw)

	p.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(195, 450), Scroll: f32.Pt(0, 4000)})
	p.draw(d.draw)
	p.draw(d.draw)

	for y := 360; y < 580 && d.body.last == 0; y += 4 {
		p.tap(image.Pt(195, y), d.draw)
	}
	if d.body.last == 0 {
		t.Error("the last line of a scrolling drawer cannot be reached")
	}
}

// pinned is a sheet that fills its panel: a spacer that takes what is left, and a
// button at the foot, which is what a sheet of filters with Apply at the bottom is.
type pinned struct {
	drawer  widget.Dialog
	apply   widget.Button
	pressed int
}

func (s *pinned) draw(c ayra.Context) ayra.Dimensions {
	if s.apply.Clicked(c) {
		s.pressed++
	}
	return widget.DrawerProps{Side: widget.Bottom, Size: 250, Modal: true, Dismissible: true}.Layout(c, &s.drawer,
		func(c ayra.Context) ayra.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min.X = inner.Constraints.Max.X
					return widget.ButtonProps{Label: "Aplicar"}.Layout(inner, &s.apply)
				}),
			)
		})
}

// TestADrawerThatFillsItsPanelKeepsItsFooterAtTheFoot: without the prop the content
// is given the panel's height, as it always was, and the button the spacer pushed
// to the foot is where it was drawn.
func TestADrawerThatFillsItsPanelKeepsItsFooterAtTheFoot(t *testing.T) {
	p, s := phone(600), new(pinned)
	s.drawer.Open()
	p.draw(s.draw)

	// The panel is the lower 250 points, inset by 20: the button is in the last
	// 36 points above the inset.
	p.tap(image.Pt(195, 600-20-12), s.draw)

	if s.pressed != 1 {
		t.Errorf("the footer of a sheet that fills its panel answered %d times to a press at the foot, want 1", s.pressed)
	}
}
