package widget

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
)

func TestTheServersOptionsAreOfferedEvenWhenTheTextIsNotInTheLabel(t *testing.T) {
	lookup := LookupFunc(func(ctx context.Context, query string) ([]Option, error) {
		if !strings.EqualFold(query, "sp") {
			return nil, nil
		}
		// Matched on the code: "sp" is not in "Sao Paulo".
		return []Option{{Value: "SP", Label: "Sao Paulo"}}, nil
	})
	props := ComboboxProps{Source: lookup, Debounce: time.Millisecond}
	var state Combobox

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)

	state.field.input.SetText("sp")
	props.Layout(c, &state)

	c.Now = c.Now.Add(time.Second)
	for deadline := time.Now().Add(3 * time.Second); ; {
		props.Layout(c, &state)
		state.mu.Lock()
		got := len(state.options)
		state.mu.Unlock()
		if got == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the server's answer never arrived")
		}
		time.Sleep(2 * time.Millisecond)
	}

	// The window draws again when the answer calls Redraw.
	props.Layout(c, &state)

	if !state.field.Showing() {
		t.Error("the options were found and the list is not open: a second, local filter dropped them")
	}
}

func TestTypingAfterAChoiceUnchoosesIt(t *testing.T) {
	props := ComboboxProps{Debounce: time.Millisecond}
	var state Combobox
	state.Set(Option{Value: "SP", Label: "Sao Paulo"})

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	if _, ok := state.Selected(); !ok {
		t.Fatal("drawing the field lost the choice")
	}

	state.field.input.SetText("Sao Pa")
	props.Layout(c, &state)
	if _, ok := state.Selected(); ok {
		t.Error("the field shows one text and holds another choice")
	}
}
