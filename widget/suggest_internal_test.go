package widget

import (
	"strings"
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// commands is the list these tests filter, chosen so that one query matches
// several entries and another matches none.
var commands = []string{"Open file", "Open recent", "Save", "Save as", "Quit"}

// TestAnEmptyQueryMatchesEverything is what makes a list of commands a list of
// commands before anything is typed.
func TestAnEmptyQueryMatchesEverything(t *testing.T) {
	for _, query := range []string{"", "   ", "\t"} {
		found := matches(commands, query)
		if len(found) != len(commands) {
			t.Errorf("the query %q matched %d of %d entries", query, len(found), len(commands))
		}
	}
}

// TestMatchingDoesNotDependOnCapitalisation keeps a list from going empty
// because somebody typed the first letter in upper case.
func TestMatchingDoesNotDependOnCapitalisation(t *testing.T) {
	lower := matches(commands, "open")
	upper := matches(commands, "OPEN")
	mixed := matches(commands, "OpEn")

	if len(lower) != 2 {
		t.Fatalf("open matched %d entries, want 2", len(lower))
	}
	if len(upper) != len(lower) || len(mixed) != len(lower) {
		t.Errorf("the same query in three cases matched %d, %d and %d entries", len(lower), len(upper), len(mixed))
	}
}

// TestTheOrderIsTheOneTheCallerGave fixes what a score nobody can see would
// cost: the entry under the cursor when return is pressed would not be the one
// that was under it when the key went down.
func TestTheOrderIsTheOneTheCallerGave(t *testing.T) {
	found := matches(commands, "save")

	if len(found) != 2 {
		t.Fatalf("save matched %d entries, want 2", len(found))
	}
	if found[0] >= found[1] {
		t.Errorf("the matches came back out of order: %v", found)
	}
	if commands[found[0]] != "Save" {
		t.Errorf("the first match is %q, and the caller listed Save first", commands[found[0]])
	}
}

// TestAQueryThatMatchesNothingMatchesNothing keeps a list from falling back to
// everything when it narrows to zero, which is the moment somebody presses
// return.
func TestAQueryThatMatchesNothingMatchesNothing(t *testing.T) {
	if found := matches(commands, "zzz"); len(found) != 0 {
		t.Errorf("a query nothing matches matched %d entries", len(found))
	}
}

// TestTheCursorWrapsAtBothEnds is how somebody reaches the last entry without
// holding a key down.
func TestTheCursorWrapsAtBothEnds(t *testing.T) {
	if at := step(0, -1, 5); at != 4 {
		t.Errorf("stepping back from the first reached %d, want the last", at)
	}
	if at := step(4, 1, 5); at != 0 {
		t.Errorf("stepping on from the last reached %d, want the first", at)
	}
	if at := step(2, 1, 5); at != 3 {
		t.Errorf("stepping on from the third reached %d", at)
	}
}

// TestTheCursorOnAnEmptyListIsNowhere keeps a remainder from dividing by zero
// when the query narrows to nothing.
func TestTheCursorOnAnEmptyListIsNowhere(t *testing.T) {
	if at := step(0, 1, 0); at != -1 {
		t.Errorf("stepping through nothing reached %d, want nowhere", at)
	}
	if at := step(-1, -1, 0); at != -1 {
		t.Errorf("stepping back through nothing reached %d, want nowhere", at)
	}
}

// TestAStepFromNowhereLandsAtTheNearEnd keeps the first arrow key from being
// swallowed: with nothing under the cursor, down means the first entry and up
// means the last.
func TestAStepFromNowhereLandsAtTheNearEnd(t *testing.T) {
	if at := step(-1, 1, 5); at != 0 {
		t.Errorf("the first press of down reached %d, want the first entry", at)
	}
	if at := step(-1, -1, 5); at != 4 {
		t.Errorf("the first press of up reached %d, want the last entry", at)
	}
}

// TestEveryStepLandsOnAnEntryThatExists is the invariant the two remainders are
// there for.
func TestEveryStepLandsOnAnEntryThatExists(t *testing.T) {
	for length := 1; length <= 6; length++ {
		for at := 0; at < length; at++ {
			for _, by := range []int{-3, -1, 1, 2, 7} {
				landed := step(at, by, length)
				if landed < 0 || landed >= length {
					t.Errorf("from %d by %d of %d landed at %d", at, by, length, landed)
				}
			}
		}
	}
}

// TestAnEmptyFieldSuggestsNothing fixes the one difference between this control
// and the palette: every entry matching an empty query is what a list of
// commands wants and what a text field does not. Without it the panel is open
// from the moment the screen is drawn.
func TestAnEmptyFieldSuggestsNothing(t *testing.T) {
	var state Autocomplete

	c, _ := field(t, theme.Light, 400)
	AutocompleteProps{Entries: commands}.Layout(c, &state)

	if state.Showing() {
		t.Error("a field nobody has typed in is showing suggestions")
	}
}

// TestTypingOpensTheSuggestionsAndChoosingClosesThem is the whole cycle.
func TestTypingOpensTheSuggestionsAndChoosingClosesThem(t *testing.T) {
	var state Autocomplete
	props := AutocompleteProps{Entries: commands}

	state.input.SetText("op")

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	if !state.Showing() {
		t.Fatal("typing something that matches opened nothing")
	}

	state.choose(1, commands[1])

	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if state.Showing() {
		t.Error("the suggestions are still open over the form")
	}
	if state.Text() != commands[1] {
		t.Errorf("the field holds %q, want the entry that was chosen", state.Text())
	}
}

// TestAChosenSuggestionIsReportedOnceAndConsumed keeps one press from being
// acted on every frame for as long as the text stays in the field.
func TestAChosenSuggestionIsReportedOnceAndConsumed(t *testing.T) {
	var state Autocomplete

	if at := state.Chosen(); at != -1 {
		t.Errorf("nothing was chosen and the control answered %d", at)
	}

	state.choose(3, commands[3])

	if at := state.Chosen(); at != 3 {
		t.Errorf("the control answered %d, want the entry that was chosen", at)
	}
	if at := state.Chosen(); at != -1 {
		t.Errorf("the choice was reported twice as %d, and whatever it fires fires twice", at)
	}
}

// TestFillingTheFieldInDoesNotOpenThePanel keeps a screen that arrives with a
// value from drawing a list of suggestions for text nobody typed.
func TestFillingTheFieldInDoesNotOpenThePanel(t *testing.T) {
	var state Autocomplete
	props := AutocompleteProps{Entries: commands}

	state.input.SetText("op")
	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	if !state.Showing() {
		t.Fatal("typing opened nothing, so this test proves nothing")
	}

	state.SetText("Open file")

	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if state.Showing() {
		t.Error("filling the field in opened a panel over whatever is under it")
	}
}

// TestADisabledFieldSuggestsNothing keeps a list of suggestions from being
// drawn over a form whose presses are all refused.
func TestADisabledFieldSuggestsNothing(t *testing.T) {
	var state Autocomplete

	state.input.SetText("op")
	c, _ := field(t, theme.Light, 400)
	AutocompleteProps{Entries: commands}.Layout(c, &state)
	if !state.Showing() {
		t.Fatal("typing opened nothing, so this test proves nothing")
	}

	c, _ = field(t, theme.Light, 400)
	AutocompleteProps{Entries: commands, Disabled: true}.Layout(c, &state)
	if state.Showing() {
		t.Error("a disabled field is suggesting entries")
	}
}

// TestThePaletteOpensOnTheFirstEntryWithNothingTyped keeps return from doing
// nothing on the first keystroke of a palette that was just opened.
func TestThePaletteOpensOnTheFirstEntryWithNothingTyped(t *testing.T) {
	var state Palette
	state.Open()

	if state.at != 0 {
		t.Errorf("the palette opened with the cursor at %d, want the first entry", state.at)
	}
	if state.Query() != "" {
		t.Errorf("the palette opened holding %q from last time", state.Query())
	}
	if !state.Showing() {
		t.Error("the palette did not open")
	}
}

// TestTheCursorComesBackInsideAListThatNarrowed fixes what happens on every
// keystroke that filters: the list shortens under the cursor, and left alone
// return runs whatever ends up at that position next.
func TestTheCursorComesBackInsideAListThatNarrowed(t *testing.T) {
	var state Palette
	state.Open()
	state.at = 4

	c, _ := field(t, theme.Light, 400)
	state.input.SetText("save")
	PaletteProps{Commands: commands}.Layout(c, &state)

	found := matches(commands, "save")
	if state.at < 0 || state.at >= len(found) {
		t.Errorf("the cursor is at %d of %d matches", state.at, len(found))
	}
}

// TestRunningACommandTakesThePaletteDownAndReportsItOnce keeps a command from
// being run again on every frame after it was chosen.
func TestRunningACommandTakesThePaletteDownAndReportsItOnce(t *testing.T) {
	var state Palette
	state.Open()

	if at := state.Chosen(); at != -1 {
		t.Errorf("nothing was run and the palette answered %d", at)
	}

	state.run(2)

	if state.Showing() {
		t.Error("the palette is still up over the application")
	}
	if at := state.Chosen(); at != 2 {
		t.Errorf("the palette answered %d, want the command that was run", at)
	}
	if at := state.Chosen(); at != -1 {
		t.Errorf("the command was reported twice as %d, and it runs twice", at)
	}
}

// TestAClosedPaletteDrawsNothingAtAll keeps a palette that is down from taking
// room in the screen under it.
func TestAClosedPaletteDrawsNothingAtAll(t *testing.T) {
	var state Palette

	c, _ := field(t, theme.Light, 400)
	dims := PaletteProps{Commands: commands}.Layout(c, &state)

	if dims.Size.X != 0 || dims.Size.Y != 0 {
		t.Errorf("a closed palette took %v", dims.Size)
	}
}

// TestThePaletteSaysWhatToTypeBeforeAnythingIsTyped keeps an empty field with
// no hint in it, which reads as a search that has not loaded.
func TestThePaletteSaysWhatToTypeBeforeAnythingIsTyped(t *testing.T) {
	if hint := (PaletteProps{}).placeholder(); strings.TrimSpace(hint) == "" {
		t.Error("a palette with no placeholder of its own draws an empty field")
	}
	if hint := (PaletteProps{Placeholder: "Ir para"}).placeholder(); hint != "Ir para" {
		t.Errorf("the palette drew %q over the one the caller gave", hint)
	}
}

// TestTypingAgainAfterAChoiceReopensTheList keeps the fix for the reopening
// panel from becoming a panel that never opens again.
func TestTypingAgainAfterAChoiceReopensTheList(t *testing.T) {
	var state Autocomplete
	props := AutocompleteProps{Entries: commands}

	state.choose(0, commands[0])

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	if state.Showing() {
		t.Fatal("the list reopened over the entry that was just chosen")
	}

	// What typing one more character leaves behind.
	state.input.SetText(commands[0] + "x")
	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if state.Showing() {
		t.Error("a query that matches nothing opened the list")
	}

	state.input.SetText("Save")
	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if !state.Showing() {
		t.Error("typing after a choice never suggests anything again")
	}
}

// TestAFieldFilledWithNothingStillSuggestsNothing keeps the empty string from
// being mistaken for "nothing was put here", which would make an empty field
// list every entry.
func TestAFieldFilledWithNothingStillSuggestsNothing(t *testing.T) {
	var state Autocomplete

	state.SetText("")

	c, _ := field(t, theme.Light, 400)
	AutocompleteProps{Entries: commands}.Layout(c, &state)
	if state.Showing() {
		t.Error("a field cleared by the screen is suggesting every entry")
	}
}

// TestTypingAwayAndBackReopensTheList is what the record of having been filled
// in is for, as opposed to the text alone.
//
// After choosing "Save" the list stays shut for that text. Typing on to
// "Save as" and then deleting back to "Save" is somebody searching again, and
// the list has to come back -- but the text is once more exactly what was put
// in the field, so the text alone cannot tell the two apart.
func TestTypingAwayAndBackReopensTheList(t *testing.T) {
	var state Autocomplete
	props := AutocompleteProps{Entries: commands}

	state.choose(2, commands[2]) // Save

	c, _ := field(t, theme.Light, 400)
	props.Layout(c, &state)
	if state.Showing() {
		t.Fatal("the list reopened over the entry that was just chosen")
	}

	state.input.SetText("Save a")
	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if !state.Showing() {
		t.Fatal("typing on from a chosen entry suggested nothing")
	}

	state.input.SetText("Save")
	c, _ = field(t, theme.Light, 400)
	props.Layout(c, &state)
	if !state.Showing() {
		t.Error("deleting back to the chosen text shut the list, so searching again is impossible")
	}
}

// TestAnEmptyQueryIsTheOnlyThingThatMatchesEverything keeps the match from
// being a test somebody can pass by accident.
//
// A substring search answers true for the empty string on its own, so the
// branch that lets an empty query through has to be shown to be the one
// deciding it -- otherwise a rewrite that dropped the branch would look right.
func TestAnEmptyQueryIsTheOnlyThingThatMatchesEverything(t *testing.T) {
	for _, query := range []string{"a", "s", "o", "e"} {
		if found := matches(commands, query); len(found) == len(commands) {
			t.Errorf("the query %q matched every entry, so nothing is being filtered", query)
		}
	}
	if found := matches(commands, ""); len(found) != len(commands) {
		t.Errorf("an empty query matched %d of %d", len(found), len(commands))
	}
	if found := matches(nil, ""); len(found) != 0 {
		t.Errorf("an empty query over no entries matched %d", len(found))
	}

	// A query of nothing but spaces is an empty query: it is what the field
	// holds mid-word, and narrowing to nothing there would empty the list
	// under the cursor between two keystrokes.
	if found := matches(commands, "   "); len(found) != len(commands) {
		t.Errorf("a query of spaces matched %d of %d", len(found), len(commands))
	}
}

// TestAClosedPaletteDoesNoWork fixes what the early return is worth.
//
// A dialog that is down draws nothing by itself, so without the return the
// screen looks right -- and every frame still grows one press target per
// command and registers the keys the list answers to, on a control nobody can
// see.
func TestAClosedPaletteDoesNoWork(t *testing.T) {
	var state Palette

	for range 3 {
		c, _ := field(t, theme.Light, 400)
		PaletteProps{Commands: commands}.Layout(c, &state)
	}

	if len(state.options) != 0 {
		t.Errorf("a palette that was never opened holds %d press targets", len(state.options))
	}

	state.Open()
	c, _ := field(t, theme.Light, 400)
	PaletteProps{Commands: commands}.Layout(c, &state)
	if len(state.options) != len(commands) {
		t.Errorf("an open palette holds %d press targets for %d commands", len(state.options), len(commands))
	}
}
