package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/widget"
)

// A form on a phone, and the keyboard coming up under the field at its bottom.
//
// The window gives the screen the room the keyboard leaves, the list in it draws
// only the rows that fit, and the field the person tapped is not one of them. A
// focus whose control is not in the frame is given up, with the keyboard that was
// raised for it, so the keyboard flickered and every key it sent went nowhere.
// These tests are that form, drawn in both rooms in the order a window draws
// them, and they type through the input method's edit event as the phone does.

// form is a screen that scrolls: a page of fields below a heading, tall enough to
// fit a window of 800 and not one of 400.
type form struct {
	list   layout.List
	name   widget.Input
	email  widget.Input
	secret widget.Secret
	cpf    widget.Masked
	phone  widget.Masked
	notes  widget.Input
}

func (f *form) layout(c ayra.Context) ayra.Dimensions {
	f.list.Axis = layout.Vertical
	rows := []ayra.Widget{
		func(c ayra.Context) ayra.Dimensions { return layout.Spacer{Height: 300}.Layout(c.Context) },
		func(c ayra.Context) ayra.Dimensions { return widget.InputProps{Placeholder: "Nome"}.Layout(c, &f.name) },
		func(c ayra.Context) ayra.Dimensions {
			return widget.InputProps{Placeholder: "E-mail", Kind: widget.Email}.Layout(c, &f.email)
		},
		func(c ayra.Context) ayra.Dimensions {
			return widget.PasswordProps{Placeholder: "Senha"}.Layout(c, &f.secret)
		},
		func(c ayra.Context) ayra.Dimensions {
			return widget.MaskedProps{Mask: brazil.CPF, Kind: widget.Number}.Layout(c, &f.cpf)
		},
		func(c ayra.Context) ayra.Dimensions {
			return widget.MaskedProps{Mask: brazil.Phone, Kind: widget.Telephone}.Layout(c, &f.phone)
		},
		func(c ayra.Context) ayra.Dimensions {
			return widget.InputProps{Placeholder: "Observações"}.Layout(c, &f.notes)
		},
	}
	return f.list.Layout(c.Context, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		return layout.Inset{Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return rows[i](c.With(gtx)) })
	})
}

// keyboardUp draws the frames a window draws when the keyboard comes up: the
// window asks for the field to be revealed as its room shrinks, draws the screen
// in the smaller room and settles the reveal after each frame. It fails when the
// keyboard is closed on the way.
func keyboardUp(t *testing.T, p *panel, f *form, height int) {
	t.Helper()
	p.size.Y = height
	room := image.Rect(0, 0, p.size.X, height)
	p.router.BeginReveal(room, true)
	for i := 0; i < input.RevealFrames; i++ {
		p.draw(f.layout)
		if got := p.router.TextInputState(); got == input.TextInputClose {
			t.Fatalf("frame %d with the keyboard up closed it", i+1)
		}
		if !p.router.ContinueReveal(room) {
			break
		}
	}
	p.draw(f.layout)
}

// TestAFieldBelowTheKeyboardIsBroughtAboveItAndTakesTyping taps the last field of
// a form that fitted its window, raises the keyboard, and types with accents.
func TestAFieldBelowTheKeyboardIsBroughtAboveItAndTakesTyping(t *testing.T) {
	p := newPanel()
	p.size = image.Pt(400, 800)
	var f form

	p.draw(f.layout)
	p.draw(f.layout)
	if f.list.Position.First != 0 || f.list.Position.Count != 7 {
		t.Fatalf("the form was meant to fit the window: %+v", f.list.Position)
	}

	// The phone field is the fifth row, drawn low in the window.
	tap(p, f.layout, 200, 508)
	p.router.TextInputState()
	p.draw(f.layout)

	keyboardUp(t, p, &f, 400)

	if f.list.Position.First == 0 && f.list.Position.Offset == 0 {
		t.Errorf("the form did not scroll: %+v", f.list.Position)
	}
	typeText(p, f.layout, "11987654321")
	if got := f.phone.Text(); got != "(11) 98765-4321" {
		t.Errorf("the field holds %q after typing with the keyboard up, want %q", got, "(11) 98765-4321")
	}
	if got := p.router.TextInputState(); got == input.TextInputClose {
		t.Error("typing closed the keyboard")
	}
}
