package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// A catalogue is a list of what it happens to contain, and that is the whole
// problem with one.
//
// A control added to the library and not added here draws nowhere, and nothing
// on the screen says so: what is shown still looks like every control there is,
// because there is no gap to see. The catalogue would go on being trusted while
// being wrong, which is worse than having none -- somebody comparing two
// controls would conclude the second does not exist.
//
// So the library's own source is read, and the two lists are compared. It is
// read rather than reflected over because a props type is a type and not a
// value: nothing at run time can be asked what the package declares.

// declaration matches an exported props type.
//
// The anchor is what makes it a declaration rather than a mention: the same
// words appear in a doc comment on nearly every one of them, and a pattern
// without it counts each control twice and invents a few besides.
var declaration = regexp.MustCompile(`(?m)^type ([A-Z][A-Za-z0-9]*Props) struct`)

// TestEveryControlIsInTheCatalogue is the gate.
func TestEveryControlIsInTheCatalogue(t *testing.T) {
	drawn := controls()

	for _, control := range declared(t) {
		if !slices.Contains(drawn, control) {
			t.Errorf("the library has %s and the catalogue does not draw it: a control missing from a catalogue leaves no gap anybody can see", control)
		}
	}
}

// TestTheCatalogueNamesNothingTheLibraryDoesNotHave is the same rule from the
// other side.
//
// A name left behind after a control is renamed reads as coverage and draws
// nothing: the entry beside it is still on the screen under the old heading,
// and the gate above passes because the new name is nowhere to be compared.
func TestTheCatalogueNamesNothingTheLibraryDoesNotHave(t *testing.T) {
	exists := declared(t)

	for _, control := range controls() {
		if !slices.Contains(exists, control) {
			t.Errorf("the catalogue draws %s, which the library does not declare", control)
		}
	}
}

// TestNoControlIsDrawnTwice keeps a duplicate from standing in for a missing
// one.
//
// Two entries for one control satisfy the count and leave another control
// undrawn, and the two gates above would both pass on it.
func TestNoControlIsDrawnTwice(t *testing.T) {
	seen := map[string]bool{}

	for _, control := range controls() {
		if seen[control] {
			t.Errorf("%s is drawn twice, and a duplicate is a control that is not drawn at all", control)
		}
		seen[control] = true
	}
}

// TestEveryEntryCanBeDrawn fixes that an entry is more than a name.
//
// The gates above compare strings, and a string is satisfied by an entry whose
// demonstration is nil -- which is a heading, a summary and an empty card.
func TestEveryEntryCanBeDrawn(t *testing.T) {
	for _, section := range sections() {
		if len(section.Entries) == 0 {
			t.Errorf("the %s page has no controls on it", section.Name)
		}

		for _, entry := range section.Entries {
			if entry.Summary == "" {
				t.Errorf("%s has no summary, so its card says only what its type is called", entry.Control)
			}
			if entry.Build == nil {
				t.Fatalf("%s has nothing to build, and would draw an empty card under its own name", entry.Control)
			}
			if entry.Build() == nil {
				t.Errorf("%s builds nothing", entry.Control)
			}
		}
	}
}

// declared answers every exported props type the library declares, read from
// its source.
func declared(t *testing.T) []string {
	t.Helper()

	source := filepath.Join("..", "widget")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatalf("the library's controls could not be read from %s: %v", source, err)
	}

	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		body, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range declaration.FindAllStringSubmatch(string(body), -1) {
			found = append(found, match[1])
		}
	}

	// A read that found nothing would pass every gate above in silence, and a
	// gate that passes when it read nothing is not a gate. The floor is well
	// under the count so that it fails on a broken read rather than on the next
	// control somebody removes.
	if len(found) < 40 {
		t.Fatalf("only %d controls were read from %s, and the library has many more", len(found), source)
	}

	slices.Sort(found)
	return found
}
