package widget_test

import (
	"strings"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/widget"
)

// TestAPasswordFieldDoesNotCopyItsText is the field a sign-in screen draws,
// pressed into, everything selected and copied: the dots are on the screen and
// the password must not be on the clipboard.
func TestAPasswordFieldDoesNotCopyItsText(t *testing.T) {
	p := newPanel()
	var state widget.Input
	state.SetText("hunter2-secret")
	screen := func(c ayra.Context) ayra.Dimensions {
		state.Submitted(c)
		return widget.InputProps{Kind: widget.Password}.Layout(c, &state)
	}

	p.draw(screen)
	at := f32.Pt(20, 10)
	p.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	p.draw(screen)
	for _, name := range []key.Name{"A", "C", "X"} {
		p.router.Queue(key.Event{Name: name, Modifiers: key.ModShortcut, State: key.Press})
		p.draw(screen)
		if _, content, ok := p.router.WriteClipboard(); ok {
			t.Errorf("shortcut %s on a password field wrote %q to the clipboard", name, content)
		}
	}
	if got := state.Text(); got != "hunter2-secret" {
		t.Errorf("the password field holds %q after a refused cut", got)
	}
	if got := p.router.EditorState().Snippet.Text; strings.Trim(got, "•") != "" {
		t.Errorf("the input method was told %q", got)
	}
}

// TestThePasswordEyeIsInsideTheField is the parity with Kyse's input group: the
// control that reveals what was typed sits inside the field's border, at its
// right end, and not beside the field. The field and its eye are one box as wide
// as the room they were given, a tap on the eye reveals the value and leaves the
// field alone, and a tap anywhere else on the box focuses the field.
func TestThePasswordEyeIsInsideTheField(t *testing.T) {
	var state widget.Secret
	screen := func(c ayra.Context) ayra.Dimensions {
		return widget.PasswordProps{Placeholder: "Senha"}.Layout(c, &state)
	}

	p := newPanel()
	size := p.draw(screen).Size
	p.draw(screen)
	if size.X != p.size.X {
		t.Fatalf("the password field is %d wide in a room of %d: the eye is outside it", size.X, p.size.X)
	}

	// The eye is in the last 48 points of the field, vertically centred.
	tap(p, screen, size.X-24, size.Y/2)
	if !state.Shown() {
		t.Errorf("a tap on the eye, inside the field's right end, did not reveal the value")
	}
	tap(p, screen, size.X-24, size.Y/2)
	if state.Shown() {
		t.Errorf("a second tap on the eye did not hide the value again")
	}

	// Left of the eye the tap is the field's, and what is typed arrives.
	tap(p, screen, 40, size.Y/2)
	typeText(p, screen, "ação")
	if got := state.Text(); got != "ação" {
		t.Errorf("the field holds %q after a tap beside the eye and typing, want %q", got, "ação")
	}
	if state.Shown() {
		t.Errorf("typing revealed the value")
	}
}
