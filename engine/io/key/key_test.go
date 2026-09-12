package key_test

import (
	"runtime"
	"testing"

	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/key"
)

// What this package promises is a vocabulary, and a vocabulary is only worth
// anything if it means the same thing on both sides of it.
//
// The platform layer writes these values and a control reads them, and the two
// never meet in one function: nothing in the compiler notices when a name is
// respelled or a modifier bit moves, and nothing at runtime reports it either
// -- a shortcut simply stops firing, on one platform, for one combination. So
// these tests fix the spelling and the arithmetic, and they run the one match
// that matters through the router that really performs it.

// TestAModifierSetHoldsWhatWasPutInIt is the whole of what a set is for.
//
// Contain is asked on every key event a control receives, and a wrong answer in
// either direction is a shortcut that fires when it should not or stays silent
// when it should not -- both of which read to a person as an application that
// ignores the keyboard.
func TestAModifierSetHoldsWhatWasPutInIt(t *testing.T) {
	held := key.ModCtrl | key.ModShift

	for _, put := range []key.Modifiers{
		key.ModCtrl,
		key.ModShift,
		key.ModCtrl | key.ModShift,
	} {
		if !held.Contain(put) {
			t.Errorf("%v does not contain %v, which was put in it", held, put)
		}
	}

	for _, absent := range []key.Modifiers{
		key.ModAlt,
		key.ModSuper,
		key.ModCommand,
		key.ModCtrl | key.ModAlt,
	} {
		if held.Contain(absent) {
			t.Errorf("%v contains %v, which was never put in it", held, absent)
		}
	}
}

// TestEveryModifierIsABitOfItsOwn catches the quiet failure of a set built out
// of a shifted constant: two modifiers sharing a bit answer Contain for each
// other, and the shortcut that goes wrong is somebody else's.
func TestEveryModifierIsABitOfItsOwn(t *testing.T) {
	all := []key.Modifiers{
		key.ModCtrl,
		key.ModCommand,
		key.ModShift,
		key.ModAlt,
		key.ModSuper,
	}

	for i, one := range all {
		if one == 0 {
			t.Errorf("modifier %d is the empty set, so every set contains it", i)
		}
		for j, other := range all {
			if i == j {
				continue
			}
			if one&other != 0 {
				t.Errorf("%v and %v share a bit, so a key held with one reads as held with the other", one, other)
			}
		}
	}
}

// TestTheEmptySetIsRequiredByNobodyAndHoldsNothing fixes the two answers the
// empty set has to give.
//
// A filter that requires no modifier is the common one -- an arrow key, a
// letter in a text field -- and it is written by leaving Required at its zero
// value. If the empty set were not contained by every set, that filter would
// match nothing at all, which is the state where a field takes no typing.
func TestTheEmptySetIsRequiredByNobodyAndHoldsNothing(t *testing.T) {
	var none key.Modifiers

	for _, held := range []key.Modifiers{
		none,
		key.ModCtrl,
		key.ModCtrl | key.ModAlt | key.ModShift,
	} {
		if !held.Contain(none) {
			t.Errorf("%v does not contain the empty set, so a filter requiring no modifier would match nothing", held)
		}
	}

	for _, asked := range []key.Modifiers{
		key.ModCtrl,
		key.ModShift,
		key.ModCtrl | key.ModShift,
	} {
		if none.Contain(asked) {
			t.Errorf("the empty set contains %v, which was never put in it", asked)
		}
	}

	if said := none.String(); said != "" {
		t.Errorf("the empty set reads as %q, want nothing said at all", said)
	}
}

// TestAModifierSetReadsBackInADeclaredOrder fixes the text, because it is shown
// to a person: a shortcut printed beside a menu entry is this string, and an
// order that followed however the bits happened to be laid out would print the
// same combination two ways.
func TestAModifierSetReadsBackInADeclaredOrder(t *testing.T) {
	for _, c := range []struct {
		held key.Modifiers
		want string
	}{
		{key.ModCtrl, "Ctrl"},
		{key.ModShift, "Shift"},
		{key.ModAlt, "Alt"},
		{key.ModSuper, "Super"},
		{key.ModCommand, "⌘"},
		{key.ModCtrl | key.ModShift, "Ctrl-Shift"},
		{key.ModShift | key.ModCtrl, "Ctrl-Shift"},
		{key.ModAlt | key.ModCtrl | key.ModShift, "Ctrl-Shift-Alt"},
	} {
		if said := c.held.String(); said != c.want {
			t.Errorf("a set of %d reads as %q, want %q", uint32(c.held), said, c.want)
		}
	}
}

// TestANamedKeyIsSpelledTheSameEveryTime is a fixture, and it is deliberately
// written out rather than derived.
//
// A Name is a string that crosses from the platform layer to a control without
// either side declaring what it expects: the window code produces "⎋" and an
// editor compares against NameEscape, and the only thing joining them is that
// the two are the same runes. Respelling one -- tidying an arrow, writing
// "Enter" where "⏎" was, changing case -- compiles, passes everything that does
// not name the key, and silently unbinds it. Deriving the wanted value from the
// constant would test nothing; the literal is the test.
func TestANamedKeyIsSpelledTheSameEveryTime(t *testing.T) {
	for _, c := range []struct {
		name key.Name
		want string
	}{
		{key.NameLeftArrow, "←"},
		{key.NameRightArrow, "→"},
		{key.NameUpArrow, "↑"},
		{key.NameDownArrow, "↓"},
		{key.NameReturn, "⏎"},
		{key.NameEnter, "⌤"},
		{key.NameEscape, "⎋"},
		{key.NameHome, "⇱"},
		{key.NameEnd, "⇲"},
		{key.NameDeleteBackward, "⌫"},
		{key.NameDeleteForward, "⌦"},
		{key.NamePageUp, "⇞"},
		{key.NamePageDown, "⇟"},
		{key.NameTab, "Tab"},
		{key.NameSpace, "Space"},
		{key.NameCtrl, "Ctrl"},
		{key.NameShift, "Shift"},
		{key.NameAlt, "Alt"},
		{key.NameSuper, "Super"},
		{key.NameCommand, "⌘"},
		{key.NameF1, "F1"},
		{key.NameF2, "F2"},
		{key.NameF3, "F3"},
		{key.NameF4, "F4"},
		{key.NameF5, "F5"},
		{key.NameF6, "F6"},
		{key.NameF7, "F7"},
		{key.NameF8, "F8"},
		{key.NameF9, "F9"},
		{key.NameF10, "F10"},
		{key.NameF11, "F11"},
		{key.NameF12, "F12"},
		{key.NameBack, "Back"},
	} {
		if string(c.name) != c.want {
			t.Errorf("a named key reads as %q, want %q; a respelled name unbinds every shortcut that uses it", string(c.name), c.want)
		}
	}
}

// TestNoTwoKeysShareAName catches the copy that was not finished: two constants
// with the same text are one key under two names, and the second binding takes
// events meant for the first.
func TestNoTwoKeysShareAName(t *testing.T) {
	named := map[key.Name]bool{}
	for _, name := range []key.Name{
		key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow,
		key.NameReturn, key.NameEnter, key.NameEscape, key.NameHome, key.NameEnd,
		key.NameDeleteBackward, key.NameDeleteForward, key.NamePageUp, key.NamePageDown,
		key.NameTab, key.NameSpace, key.NameCtrl, key.NameShift, key.NameAlt,
		key.NameSuper, key.NameCommand,
		key.NameF1, key.NameF2, key.NameF3, key.NameF4, key.NameF5, key.NameF6,
		key.NameF7, key.NameF8, key.NameF9, key.NameF10, key.NameF11, key.NameF12,
		key.NameBack,
	} {
		if named[name] {
			t.Errorf("%q names two keys, so one of them takes the other's events", string(name))
		}
		named[name] = true
	}
}

// TestAKeyStateReadsBackAsAWord fixes the text of a state, which is what a log
// line and a failing test print.
func TestAKeyStateReadsBackAsAWord(t *testing.T) {
	if said := key.Press.String(); said != "Press" {
		t.Errorf("Press reads as %q, want %q", said, "Press")
	}
	if said := key.Release.String(); said != "Release" {
		t.Errorf("Release reads as %q, want %q", said, "Release")
	}
}

// TestAnUnknownStateStillReadsBack is about the value nobody meant to make.
//
// A state that came from a platform this library does not know is printed for
// exactly one reason -- to find out what it was -- and a String that gave up on
// it would take the program down at the moment of asking. It says the number.
func TestAnUnknownStateStillReadsBack(t *testing.T) {
	unknown := key.State(9)

	said := func() (said string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("printing an unknown state panicked with %v, and printing it is how anybody finds out what it was", r)
			}
		}()
		return unknown.String()
	}()

	if said == "" {
		t.Fatal("an unknown state reads as nothing at all, so a log line about it says nothing")
	}
	if said == key.Press.String() || said == key.Release.String() {
		t.Fatalf("an unknown state reads as %q, which is a state it is not", said)
	}
}

// TestTheShortcutModifierIsTheOnePeopleOnThisPlatformPress is the reason the
// modifier is chosen by a separate file per platform rather than by a flag.
//
// Ctrl-C on Apple hardware is not copy, and command-C elsewhere is not a key
// combination at all. A single constant would be wrong on half the machines,
// and wrong in the way the person typing cannot work around.
//
// The browser is asked a different question, because there the answer is
// decided while the program runs and the build says nothing about the machine
// it landed on. What has to hold there is that the two travel together: the
// pair is ctrl and ctrl, or command and alt, and never one of each -- a
// half-applied decision gives an application where copy works and
// move-by-word does not.
func TestTheShortcutModifierIsTheOnePeopleOnThisPlatformPress(t *testing.T) {
	apple := key.ModShortcut == key.ModCommand && key.ModShortcutAlt == key.ModAlt
	elsewhere := key.ModShortcut == key.ModCtrl && key.ModShortcutAlt == key.ModCtrl

	if !apple && !elsewhere {
		t.Fatalf("the shortcut modifiers on %s are %v and %v, which is neither pair this library defines",
			runtime.GOOS, key.ModShortcut, key.ModShortcutAlt)
	}

	switch runtime.GOOS {
	case "darwin", "ios":
		if !apple {
			t.Errorf("the shortcut modifiers on %s are %v and %v, want %v and %v",
				runtime.GOOS, key.ModShortcut, key.ModShortcutAlt, key.ModCommand, key.ModAlt)
		}
	case "js":
		// Decided from what the browser reports, so there is nothing static to
		// compare it against. The pair being one of the two is the assertion.
	default:
		if !elsewhere {
			t.Errorf("the shortcut modifiers on %s are %v and %v, want %v and %v",
				runtime.GOOS, key.ModShortcut, key.ModShortcutAlt, key.ModCtrl, key.ModCtrl)
		}
	}
}

// TestAShortcutFilterMatchesOnlyItsOwnCombination is the promise a Filter
// makes, run through the router that really performs the match.
//
// Asserting the arithmetic here instead would be asserting a second copy of it,
// and a second copy agrees with itself no matter what the router does. What a
// control needs to know is whether ctrl-shift-S reaches it and ctrl-S does not.
func TestAShortcutFilterMatchesOnlyItsOwnCombination(t *testing.T) {
	const shortcut = key.ModCtrl | key.ModShift
	filter := key.Filter{Name: "S", Required: shortcut}

	wanted := key.Event{Name: "S", Modifiers: shortcut, State: key.Press}
	if got, ok := delivered(t, filter, wanted); !ok || got != wanted {
		t.Errorf("the combination the filter asks for was delivered as %v, %t; want %v, true", got, ok, wanted)
	}

	for _, refused := range []struct {
		why   string
		event key.Event
	}{
		{"one modifier short", key.Event{Name: "S", Modifiers: key.ModCtrl, State: key.Press}},
		{"one modifier too many", key.Event{Name: "S", Modifiers: shortcut | key.ModAlt, State: key.Press}},
		{"the other modifier", key.Event{Name: "S", Modifiers: key.ModCtrl | key.ModAlt, State: key.Press}},
		{"no modifier at all", key.Event{Name: "S", State: key.Press}},
		{"another key", key.Event{Name: "D", Modifiers: shortcut, State: key.Press}},
	} {
		if got, ok := delivered(t, filter, refused.event); ok {
			t.Errorf("%s was delivered as %v, and a filter that takes it fires the shortcut on the wrong keystroke", refused.why, got)
		}
	}
}

// TestAnOptionalModifierIsAllowedButNotDemanded covers the other half of a
// filter, which is how an arrow key is written: shift may be held to extend a
// selection, and the same filter has to match with it and without it.
func TestAnOptionalModifierIsAllowedButNotDemanded(t *testing.T) {
	filter := key.Filter{Name: key.NameRightArrow, Optional: key.ModShift}

	for _, wanted := range []key.Event{
		{Name: key.NameRightArrow, State: key.Press},
		{Name: key.NameRightArrow, Modifiers: key.ModShift, State: key.Press},
	} {
		if got, ok := delivered(t, filter, wanted); !ok || got != wanted {
			t.Errorf("%v was delivered as %v, %t; want %v, true", wanted, got, ok, wanted)
		}
	}

	held := key.Event{Name: key.NameRightArrow, Modifiers: key.ModCtrl, State: key.Press}
	if got, ok := delivered(t, filter, held); ok {
		t.Errorf("a modifier the filter never named was delivered as %v", got)
	}
}

// delivered queues one event at a router filtering for filter, and answers what
// came back out.
//
// A router of its own per call, because a router carries state between frames
// and a leftover from the previous case would be read as this one's answer.
func delivered(t *testing.T, filter key.Filter, e key.Event) (key.Event, bool) {
	t.Helper()

	var router input.Router
	// Registers the filter. The first ask is always empty: nothing has been
	// queued yet, and the router only routes to filters it has been shown.
	router.Event(filter)
	router.Queue(e)

	got, ok := router.Event(filter)
	if !ok {
		return key.Event{}, false
	}
	pressed, ok := got.(key.Event)
	if !ok {
		t.Fatalf("a key filter was answered with %T, which is not a key event", got)
	}
	return pressed, true
}
