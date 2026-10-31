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
