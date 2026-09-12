package ayra_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// How many platforms announce a control to a screen reader, guarded.
//
// The README tells a reader, on its first screen, that of the nine platform
// backends beneath this library exactly one walks a semantic tree, and that a
// test guards the count. This is that test. It existed as a sentence for two
// days before it existed as code, which is the reason it is written the way it
// is: a number a document states and nothing measures is a number that drifts
// away from the truth without anybody noticing.
//
// A backend is counted as announcing when it reads the callbacks that expose
// the tree. Asking for the callbacks is the only way to reach the tree, so a
// backend that does not name one of them cannot be walking it, and a backend
// that names one is at minimum reading it.

// semanticCallbacks are the four methods a backend calls to reach the tree. A
// backend that names none of them announces nothing.
var semanticCallbacks = []string{
	"SemanticRoot",
	"LookupSemantic",
	"AppendSemanticDiffs",
	"SemanticAt",
}

// announcingBackends is the number of platform backends that reach the tree,
// and platformBackends is how many there are. Both are stated in README.md and
// in SECURITY.md, so this test owns them: a change in either direction fails,
// because a rise is good news that those two documents have to carry too.
const (
	announcingBackends = 1
	platformBackends   = 9
)

func TestTheSemanticBackendCountIsWhatTheDocumentsSay(t *testing.T) {
	root := moduleRoot(t)
	backends := filepath.Join(root, "engine", "app")

	entries, err := os.ReadDir(backends)
	if err != nil {
		t.Fatal(err)
	}

	// A backend is one os_<platform>.go. The suffixed build variants of a
	// platform are counted with it, because a platform announces or does not.
	platform := regexp.MustCompile(`^os_([a-z0-9]+)\.go$`)

	announcing := map[string]bool{}
	for _, entry := range entries {
		found := platform.FindStringSubmatch(entry.Name())
		if found == nil {
			continue
		}
		name := found[1]
		announcing[name] = false

		source, err := os.ReadFile(filepath.Join(backends, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, callback := range semanticCallbacks {
			if strings.Contains(string(source), callback) {
				announcing[name] = true
				break
			}
		}
	}

	// A walk that matched nothing would report zero and read like a
	// regression, which is the failure this whole file exists to prevent.
	if len(announcing) != platformBackends {
		t.Fatalf("found %d platform backends under engine/app and expected %d; "+
			"if a platform was added or removed, this constant and the two documents that state it move together",
			len(announcing), platformBackends)
	}

	var reached, silent []string
	for name, yes := range announcing {
		if yes {
			reached = append(reached, name)
		} else {
			silent = append(silent, name)
		}
	}

	if len(reached) < announcingBackends {
		t.Errorf("%d of %d backends reach the semantic tree, down from %d: %v went silent",
			len(reached), len(announcing), announcingBackends, silent)
	}
	if len(reached) > announcingBackends {
		t.Errorf("%d of %d backends now reach the semantic tree, up from %d: %v. "+
			"Raise the constant, and say so in README.md and SECURITY.md, which both state the old number",
			len(reached), len(announcing), announcingBackends, reached)
	}
}
