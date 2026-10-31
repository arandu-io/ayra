package widget

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A part a widget publishes has to be a part it resolves, and the other way
// round.
//
// The web components carry this promise as a test too: PartNames is compared
// with the data-part attributes the component renders, in both directions. A
// name published and not drawn is a caller writing Parts["title"] and seeing
// nothing change, with no error anywhere; a name drawn and not published is a
// part nobody knows exists. Here the drawing is a call to Theme.Resolve, so the
// two lists compared are PartNames and the literals passed to Resolve.

var (
	publishes = regexp.MustCompile(`(?m)^func \(p ([A-Za-z]+)Props\) PartNames\(\) \[\]string \{\s*return \[\]string\{([^}]*)\}`)
	resolves  = regexp.MustCompile(`Resolve\("([a-z]+)", "([a-z-]+)"`)
	// resolvesByName is the form a widget uses when it is drawn under two names:
	// the input, which is a text area as well.
	resolvesByName = regexp.MustCompile(`Resolve\(name, "([a-z-]+)"`)
)

func TestEveryPublishedPartIsResolvedAndEveryResolvedPartIsPublished(t *testing.T) {
	published := map[string]map[string]bool{}
	resolved := map[string]map[string]bool{}
	var byName []string
	var all strings.Builder

	for _, path := range sources(t) {
		body := read(t, path)
		all.WriteString(body)

		for _, found := range publishes.FindAllStringSubmatch(body, -1) {
			name := strings.ToLower(found[1])
			published[name] = map[string]bool{}
			for _, part := range strings.Split(found[2], ",") {
				if part = strings.Trim(strings.TrimSpace(part), `"`); part != "" {
					published[name][part] = true
				}
			}
		}
		for _, found := range resolves.FindAllStringSubmatch(body, -1) {
			if resolved[found[1]] == nil {
				resolved[found[1]] = map[string]bool{}
			}
			resolved[found[1]][found[2]] = true
		}
		if filepath.Base(path) == "input.go" {
			for _, found := range resolvesByName.FindAllStringSubmatch(body, -1) {
				byName = append(byName, found[1])
			}
		}
	}

	if len(published) < 10 {
		t.Fatalf("only %d widgets publish their parts; the pattern stopped matching", len(published))
	}

	// The input resolves its parts under the name it is drawn as.
	for _, name := range []string{"input", "textarea"} {
		resolved[name] = map[string]bool{}
		for _, part := range byName {
			resolved[name][part] = true
		}
	}

	for name, parts := range published {
		for part := range parts {
			if !resolved[name][part] {
				t.Errorf("%s publishes the part %q and never resolves it: a caller who styles it would see nothing change", name, part)
			}
		}
		if !parts["root"] {
			t.Errorf("%s publishes no root part, and every widget has one", name)
		}
	}
	for name, parts := range resolved {
		for part := range parts {
			if !published[name][part] {
				t.Errorf("%s resolves the part %q and does not publish it in PartNames", name, part)
			}
		}
	}
}

// TestNoControlIsHandedACopyOfAClickable extends the guard on state copies to
// the exported press state.
func TestNoControlIsHandedACopyOfAClickable(t *testing.T) {
	for _, path := range sources(t) {
		if strings.Contains(read(t, path), "Clickable{click:") {
			t.Errorf("%s constructs a Clickable from a copy of its state", filepath.Base(path))
		}
	}
}
