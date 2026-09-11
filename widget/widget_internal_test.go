package widget

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoControlIsHandedACopyOfItsOwnState is the guard for a defect this
// package has now written three times.
//
// A control's state is a value the caller holds across frames, and the whole
// point of holding it is that the control remembers. Constructing one from a
// copy -- Button{click: something} -- produces a fresh control every frame: it
// never sees the release of its own press, so it can be held down and never
// fire. Nothing about it looks wrong, and the button simply does nothing.
//
// It was written in the tabs, then in the group of buttons, then in the pager,
// each time by somebody who had just read the comment warning about it. That is
// what makes it a check rather than a note.
func TestNoControlIsHandedACopyOfItsOwnState(t *testing.T) {
	// Composite literals of the state types, which is the only way the copy can
	// be spelled: a caller with a pointer to one already has the real thing.
	copies := []string{
		"Button{click:",
		"Toggle{click:",
		"Input{editor:",
		"Tabs{clicks:",
	}

	for _, path := range sources(t) {
		body := read(t, path)
		for _, spelling := range copies {
			if strings.Contains(body, spelling) {
				t.Errorf("%s constructs a control from a copy of its state (%s); one made per frame never sees the release of its own press",
					filepath.Base(path), spelling)
			}
		}
	}
}

// TestEveryControlDrawsThroughTheOnePlaceThatDrawsText keeps a second way to
// put a string on screen from appearing.
//
// Two of them is two sets of defaults, and the pair drifts: one wraps and one
// does not, one takes the theme's size and one takes a number somebody typed.
// The failure is a screen where two labels that should match do not, by a
// point, which nobody can see and everybody can feel.
func TestEveryControlDrawsThroughTheOnePlaceThatDrawsText(t *testing.T) {
	for _, path := range sources(t) {
		if filepath.Base(path) == "text.go" {
			continue // where drawText itself lives
		}
		body := read(t, path)
		if strings.Contains(body, "engine.Label{") {
			t.Errorf("%s draws text directly instead of through drawText", filepath.Base(path))
		}
	}
}

// TestEveryPropsIsAValueAndEveryStateIsAPointer fixes the convention that makes
// the two halves tell themselves apart.
//
// Props are computed fresh every frame and carry nothing forward; state is held
// by the caller and is the only thing that does. A Layout taking its state by
// value is a control that cannot remember, and it compiles.
//
// The rule is read off the parameter rather than matched against a list of
// state types. A list was what this test had, and it named eleven of them: every
// control written after it was added was unguarded, and the test went on passing
// for each one. A guard that has to be edited when the thing it guards grows is
// a guard that stops guarding without saying so.
func TestEveryPropsIsAValueAndEveryStateIsAPointer(t *testing.T) {
	// The parameter and its type, with the pointer star captured when there is
	// one.
	holder := regexp.MustCompile(`\bstate (\*?)([A-Za-z_][A-Za-z0-9_]*)`)

	guarded := 0
	for _, path := range sources(t) {
		for _, line := range strings.Split(read(t, path), "\n") {
			if !strings.HasPrefix(line, "func (p ") || !strings.Contains(line, ") Layout(") {
				continue
			}

			arguments := line[strings.Index(line, ") Layout(")+len(") Layout("):]
			found := holder.FindStringSubmatch(arguments)
			if found == nil {
				// A control with nothing to remember takes no state, and there
				// are several: a badge, a separator, a heading.
				continue
			}

			guarded++
			if found[1] != "*" {
				t.Errorf("%s takes its state by value, so the control cannot remember anything: %s", filepath.Base(path), strings.TrimSpace(line))
			}
		}
	}

	// A rule that matched nothing would pass in silence, which is the failure
	// the list version had.
	if guarded < 20 {
		t.Errorf("only %d controls hold state, and this package has many more; the pattern stopped matching", guarded)
	}
}

// TestEveryStateParameterIsCalledState is what lets the test above read the
// parameter instead of a list.
//
// A state argument under another name is invisible to it, and would be the one
// control in the package free to take its state by value.
func TestEveryStateParameterIsCalledState(t *testing.T) {
	// Every type declared in this package, so a parameter can be recognised as
	// one of ours rather than as a context or a widget function.
	declared := regexp.MustCompile(`(?m)^type ([A-Za-z_][A-Za-z0-9_]*) struct`)
	ours := map[string]bool{}
	for _, path := range sources(t) {
		for _, found := range declared.FindAllStringSubmatch(read(t, path), -1) {
			ours[found[1]] = true
		}
	}

	// A parameter of one of our types, with the name it was given.
	parameter := regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*) \*([A-Za-z_][A-Za-z0-9_]*)`)

	for _, path := range sources(t) {
		for _, line := range strings.Split(read(t, path), "\n") {
			if !strings.HasPrefix(line, "func (p ") || !strings.Contains(line, ") Layout(") {
				continue
			}

			arguments := line[strings.Index(line, ") Layout(")+len(") Layout("):]
			for _, found := range parameter.FindAllStringSubmatch(arguments, -1) {
				name, kind := found[1], found[2]
				if !ours[kind] || name == "state" {
					continue
				}
				t.Errorf("%s calls its %s parameter %q rather than state, which puts it outside the guard above: %s", filepath.Base(path), kind, name, strings.TrimSpace(line))
			}
		}
	}
}

// sources answers every non-test Go file of this package.
func sources(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	var kept []string
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") {
			kept = append(kept, path)
		}
	}
	if len(kept) == 0 {
		t.Fatal("no sources found, so this checked nothing")
	}
	return kept
}

// read answers a file's contents.
func read(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// TestAPagerAlwaysShowsBothEnds fixes the two pages a person looks for.
//
// Neither is reachable by pressing next: page one is where somebody goes to
// start again, and the last is how they learn how much there is.
func TestAPagerAlwaysShowsBothEnds(t *testing.T) {
	for _, total := range []int{1, 2, 5, 40, 400} {
		for _, current := range []int{1, total / 2, total} {
			if current < 1 {
				current = 1
			}
			pages := PaginationProps{Total: total}.visible(current, 2)
			if len(pages) == 0 {
				t.Fatalf("a pager over %d pages drew nothing", total)
			}
			if pages[0] != 1 {
				t.Errorf("%d pages at %d does not start at one: %v", total, current, pages)
			}
			if pages[len(pages)-1] != total {
				t.Errorf("%d pages at %d does not end at the last: %v", total, current, pages)
			}
		}
	}
}

// TestAPagerNeverDrawsTwoGapsTogether keeps a row reading "1 ... ... 40".
func TestAPagerNeverDrawsTwoGapsTogether(t *testing.T) {
	pages := PaginationProps{Total: 400}.visible(200, 1)

	for index := 1; index < len(pages); index++ {
		if pages[index] == 0 && pages[index-1] == 0 {
			t.Fatalf("two gaps in a row: %v", pages)
		}
	}
}

// TestANumberStaysInsideItsBounds keeps a stepper from passing a limit it
// declares.
//
// Clamped rather than refused: a control that let somebody reach a value and
// complained afterwards wasted the press, and one that silently kept counting
// past its own maximum is a form that submits a number the server rejects.
func TestANumberStaysInsideItsBounds(t *testing.T) {
	props := NumberProps{Min: 1, Max: 10}

	for start, want := range map[int]int{
		-5: 1,
		0:  1,
		5:  5,
		10: 10,
		99: 10,
	} {
		state := &Stepper{}
		state.SetValue(start)
		props.clamp(state)

		if state.Value() != want {
			t.Errorf("%d became %d, want %d", start, state.Value(), want)
		}
	}
}

// TestANumberWithNoBoundsIsNotClamped keeps the zero value from meaning
// "between nought and nought".
func TestANumberWithNoBoundsIsNotClamped(t *testing.T) {
	state := &Stepper{}
	state.SetValue(42)
	NumberProps{}.clamp(state)

	if state.Value() != 42 {
		t.Errorf("an unbounded stepper clamped %d to %d", 42, state.Value())
	}
}

// TestAMatchIsFoundWhateverTheCapitalisation fixes what a search result marks.
//
// A search that only matched the capitalisation somebody typed would mark
// nothing on most results, and a result with nothing marked reads as a result
// that does not contain what was searched for.
func TestAMatchIsFoundWhateverTheCapitalisation(t *testing.T) {
	for _, test := range []struct {
		text, match           string
		before, marked, after string
	}{
		{"Arandu draws its own", "draws", "Arandu ", "draws", " its own"},
		{"Arandu Draws its own", "draws", "Arandu ", "Draws", " its own"},
		{"ARANDU DRAWS", "draws", "ARANDU ", "DRAWS", ""},
		{"Arandu draws", "ARANDU", "", "Arandu", " draws"},
	} {
		before, marked, after := HighlightProps{Text: test.text, Match: test.match}.split()

		if before != test.before || marked != test.marked || after != test.after {
			t.Errorf("%q with %q split as %q|%q|%q, want %q|%q|%q",
				test.text, test.match, before, marked, after, test.before, test.marked, test.after)
		}
	}
}

// TestTheMarkedPartIsTheTextThatWasThere keeps the highlight from rewriting
// what it found.
//
// The marked run comes out of the original string rather than out of the query,
// so a result found case-insensitively still reads the way it was written.
func TestTheMarkedPartIsTheTextThatWasThere(t *testing.T) {
	before, marked, after := HighlightProps{Text: "Arandu DRAWS things", Match: "draws"}.split()

	if marked != "DRAWS" {
		t.Errorf("the marked part is %q; it should be what the text said", marked)
	}
	if before+marked+after != "Arandu DRAWS things" {
		t.Errorf("the three parts do not rebuild the line: %q + %q + %q", before, marked, after)
	}
}

// TestAMissingMatchLeavesTheLineWhole keeps a search with no hit from drawing a
// mark on the first letters.
func TestAMissingMatchLeavesTheLineWhole(t *testing.T) {
	for _, match := range []string{"", "absent"} {
		before, marked, after := HighlightProps{Text: "Arandu draws", Match: match}.split()

		if marked != "" || after != "" {
			t.Errorf("%q marked %q and left %q", match, marked, after)
		}
		if before != "Arandu draws" {
			t.Errorf("%q changed the line to %q", match, before)
		}
	}
}
