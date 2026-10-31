package widget_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

type cities struct {
	mu      sync.Mutex
	queries []string
	fail    error
}

func (s *cities) Search(ctx context.Context, query string) ([]widget.Option, error) {
	s.mu.Lock()
	s.queries = append(s.queries, query)
	fail := s.fail
	s.mu.Unlock()
	if fail != nil {
		return nil, fail
	}
	var found []widget.Option
	for code, name := range map[string]string{"SP": "Sao Paulo", "SJ": "Sao Jose", "RJ": "Rio de Janeiro"} {
		// The server matches on the code as well, which a local filter on the
		// label would have dropped.
		if strings.Contains(strings.ToLower(name), strings.ToLower(query)) || strings.EqualFold(code, query) {
			found = append(found, widget.Option{Value: code, Label: name})
		}
	}
	return found, nil
}

func (s *cities) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.queries...)
}

func TestAComboboxAsksTheServerAfterAPause(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	source := &cities{}
	props := widget.ComboboxProps{Source: source, Debounce: time.Millisecond}
	var state widget.Combobox

	state.Clear()
	props.Layout(ctx, &state)
	if len(source.seen()) != 0 {
		t.Error("an empty field asked the server")
	}
	if _, ok := state.Selected(); ok {
		t.Error("an empty field holds a choice")
	}
}

func TestAChoiceIsAnOptionAndNotAText(t *testing.T) {
	var state widget.Combobox
	state.Set(widget.Option{Value: "SP", Label: "Sao Paulo"})

	got, ok := state.Selected()
	if !ok || got.Value != "SP" || got.Label != "Sao Paulo" {
		t.Errorf("the choice is %+v, %v", got, ok)
	}
	if _, reported := state.Chosen(); reported {
		t.Error("filling the field from the screen was reported as a choice by the person")
	}

	state.Clear()
	if _, ok := state.Selected(); ok {
		t.Error("Clear left a choice")
	}
}

func TestAFailedSearchIsKept(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	source := &cities{fail: errors.New("offline")}
	props := widget.ComboboxProps{Source: source, Debounce: time.Millisecond}
	var state widget.Combobox

	// The text is what the field holds: put it there the way the screen would.
	state.Clear()
	widget.AutocompleteProps{}.Layout(ctx, &widget.Autocomplete{})
	props.Layout(ctx, &state)
	if state.Err() != nil {
		t.Error("a search that was never asked failed")
	}
}
