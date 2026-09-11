package widget

import (
	"os"
	"path/filepath"
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
		if strings.Contains(body, "giowidget.Label{") {
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
func TestEveryPropsIsAValueAndEveryStateIsAPointer(t *testing.T) {
	for _, path := range sources(t) {
		for _, line := range strings.Split(read(t, path), "\n") {
			if !strings.HasPrefix(line, "func (p ") || !strings.Contains(line, ") Layout(") {
				continue
			}
			// The state argument, when there is one, comes after the context.
			arguments := line[strings.Index(line, ") Layout(")+len(") Layout("):]
			for _, name := range []string{"state Button", "state Toggle", "state Input", "state Tabs", "state Dialog", "state Pages", "state Group", "state Crumbs", "state Accordion", "state Disclosure", "state Secret"} {
				if strings.Contains(arguments, name) {
					t.Errorf("%s takes its state by value, so the control cannot remember anything: %s", filepath.Base(path), strings.TrimSpace(line))
				}
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
