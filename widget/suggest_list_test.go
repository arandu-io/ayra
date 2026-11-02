package widget_test

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/widget"
)

var places = []string{"Arandu", "Ayra", "Joaju", "Kyse", "Hesape", "Aru", "Maré alta"}

// typeIn types and draws once more, which is the frame the list opens in: the
// text is read while the field is laid out, after the list has decided.
func typeIn(p *panel, screen func(ayra.Context) ayra.Dimensions, text string) {
	typeText(p, screen, text)
	p.draw(screen)
}

func pressKey(p *panel, screen func(ayra.Context) ayra.Dimensions, name key.Name) {
	p.router.Queue(key.Event{Name: name, State: key.Press}, key.Event{Name: name, State: key.Release})
	p.draw(screen)
	p.draw(screen)
}

// TestEachOfTwoSuggestionFieldsTakesTheFocusItWasTappedFor is the form with two
// suggestion fields. The keys a list answers to were registered over the whole
// window by the second field, and the tap on the first was taken by that
// registration: the first field could never be typed into.
func TestEachOfTwoSuggestionFieldsTakesTheFocusItWasTappedFor(t *testing.T) {
	p := newPanel()
	var first, second widget.Autocomplete
	screen := func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(16).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			c := c.With(gtx)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.AutocompleteProps{Entries: places}.Layout(c.With(gtx), &first)
				}),
				layout.Rigid(layout.Spacer{Height: 12}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.AutocompleteProps{Entries: places}.Layout(c.With(gtx), &second)
				}),
			)
		})
	}
	p.draw(screen)

	tap(p, screen, 100, 32)
	typeText(p, screen, "a")
	if first.Text() != "a" || second.Text() != "" {
		t.Fatalf("a tap on the first field typed %q into it and %q into the second", first.Text(), second.Text())
	}
}

// suggestions is a field of its own with a few entries, drawn at a given depth
// in a window with a button beneath it.
type suggestions struct {
	field   widget.Autocomplete
	button  widget.Button
	top     int
	empty   string
	pressed bool
}

func (s *suggestions) layout(c ayra.Context) ayra.Dimensions {
	if s.button.Clicked(c) {
		s.pressed = true
	}
	return layout.Inset{Top: unit.Dp(s.top), Left: 16, Right: 16}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		c := c.With(gtx)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.AutocompleteProps{Entries: places, Empty: s.empty}.Layout(c.With(gtx), &s.field)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.ButtonProps{Label: "Salvar"}.Layout(c.With(gtx), &s.button)
			}),
		)
	})
}

// TestTheArrowKeysMoveThroughTheListAndReturnChoosesTheRow keeps the keyboard
// path of a suggestion field. The keys were asked for on the field's state, which
// never holds the focus, so no arrow ever arrived.
func TestTheArrowKeysMoveThroughTheListAndReturnChoosesTheRow(t *testing.T) {
	p := newPanel()
	s := &suggestions{top: 20}
	p.draw(s.layout)
	tap(p, s.layout, 100, 20+16)
	typeIn(p, s.layout, "a")
	if !s.field.Showing() {
		t.Fatal("typing did not open the list")
	}

	// Return with no row under the cursor is the field's own.
	pressKey(p, s.layout, key.NameReturn)
	if got := s.field.Chosen(); got != -1 {
		t.Fatalf("return chose entry %d with the cursor on none", got)
	}

	pressKey(p, s.layout, key.NameDownArrow)
	pressKey(p, s.layout, key.NameDownArrow)
	pressKey(p, s.layout, key.NameReturn)
	// "a" matches Arandu, Ayra, Joaju, Hesape, Aru, Maré alta: the second is Ayra.
	if got := s.field.Chosen(); got != 1 {
		t.Fatalf("down, down and return chose entry %d, want 1 (Ayra)", got)
	}
	if s.field.Text() != "Ayra" || s.field.Showing() {
		t.Errorf("the field holds %q with the list showing=%v after a choice", s.field.Text(), s.field.Showing())
	}
}

// TestEscapeClosesTheListAndItStaysClosed: the list was reopened by the next
// frame, because a text that matches is what opens it.
func TestEscapeClosesTheListAndItStaysClosed(t *testing.T) {
	p := newPanel()
	s := &suggestions{top: 20}
	p.draw(s.layout)
	tap(p, s.layout, 100, 36)
	typeIn(p, s.layout, "a")
	pressKey(p, s.layout, key.NameEscape)
	if s.field.Showing() {
		t.Fatal("escape did not close the list")
	}
	p.draw(s.layout)
	p.draw(s.layout)
	if s.field.Showing() {
		t.Fatal("the list opened again with the text unchanged")
	}
	typeIn(p, s.layout, "r")
	if !s.field.Showing() {
		t.Error("typing did not bring the list back")
	}
}

// TestTheListSaysThereIsNothingTheFirstTimeNothingMatches: the message for no
// match could only be shown by a list that was already open, so a first query
// with no answer drew nothing at all.
func TestTheListSaysThereIsNothingTheFirstTimeNothingMatches(t *testing.T) {
	p := newPanel()
	s := &suggestions{top: 20, empty: "Nada encontrado."}
	p.draw(s.layout)
	tap(p, s.layout, 100, 36)
	typeIn(p, s.layout, "zzz")
	if !s.field.Showing() {
		t.Error("the list stayed shut for a query that matched nothing, with a message to say so")
	}
}

// TestARowOverTheNextControlIsTheRowThatIsPressed: the list is drawn last, so
// the press lands on it and not on the button the screen put under it.
func TestARowOverTheNextControlIsTheRowThatIsPressed(t *testing.T) {
	p := newPanel()
	s := &suggestions{top: 20}
	p.draw(s.layout)
	tap(p, s.layout, 100, 36)
	typeIn(p, s.layout, "a")

	// The field is 20..~58 dp; the button is under it. The first row of the list
	// is at the top of the list, over the button.
	tap(p, s.layout, 100, 36+30+20)
	if s.pressed {
		t.Error("the press went through the list to the button under it")
	}
	if got := s.field.Chosen(); got < 0 {
		t.Error("the press on a row chose nothing")
	}
}

// TestAListLowInTheWindowStaysInsideIt: a field with less room under it than
// a few rows goes above itself instead of running off the bottom of the window.
func TestAListLowInTheWindowStaysInsideIt(t *testing.T) {
	p := newPanel()
	p.size = image.Pt(400, 400)
	s := &suggestions{top: 300}
	p.draw(s.layout)
	tap(p, s.layout, 100, 300+16)
	typeIn(p, s.layout, "a")

	// The row nearest the field, above it, is pressable there; below it, there
	// was no window left to press in.
	tap(p, s.layout, 100, 300-30)
	if got := s.field.Chosen(); got < 0 {
		t.Error("the list was not above the field with no room beneath it")
	}
}
