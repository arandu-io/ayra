package ayra_test

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// allowed is every module this one may require directly.
//
// The list is here rather than in a comment because a budget nobody can fail
// is not a budget. What it protects is the graph of whoever imports this: a
// program that draws a button should not be made to carry an authentication
// library, a database driver or a notification service to do it, and in Go
// there is no optional dependency to make that a choice.
//
// The engine's own requirements are on this list because the engine is source
// here now: they are what it needs to open a window, shape text and reach a
// GPU, and they arrived with it rather than being chosen on top of it.
var allowed = []string{
	"eliasnaur.com/font",
	"github.com/go-text/typesetting",
	"golang.org/x/exp",
	"golang.org/x/exp/shiny",
	"golang.org/x/image",
	"golang.org/x/net",
	"golang.org/x/sys",
	"golang.org/x/text",
}

// TestTheDependencyBudgetHolds fixes what this module requires directly.
//
// It reads the module file through the toolchain rather than grepping it, so
// that a require written on one line and a require written in a block are the
// same fact -- which is what a grep gets wrong the first time somebody runs go
// get.
func TestTheDependencyBudgetHolds(t *testing.T) {
	for _, module := range directRequirements(t, ".") {
		if !slices.Contains(allowed, module) {
			t.Errorf("%s is required directly and is not in the budget: everything that imports this module would carry it", module)
		}
	}
}

// TestTheBudgetNamesNothingThatIsNotRequired keeps the list from outliving what
// it describes.
//
// A name left on it after the dependency is gone reads as permission and
// protects nothing, and the next person adding a line finds a list that is
// already wrong.
func TestTheBudgetNamesNothingThatIsNotRequired(t *testing.T) {
	required := directRequirements(t, ".")

	for _, module := range allowed {
		if !slices.Contains(required, module) {
			t.Errorf("the budget allows %s, which is no longer required: a name left behind reads as permission", module)
		}
	}
}

// TestTheDrawingHalfDoesNotCarryTheServer is the boundary the two modules exist
// for.
//
// A native application draws for a server that is already running and asks it
// over the wire. Requiring the server's own packages here would put routing,
// authorization and a database driver in the graph of a program whose whole
// job is to put a button on a screen -- and would invite a screen to evaluate
// a policy locally, which is a second answer to a question the server is the
// only one entitled to answer.
func TestTheDrawingHalfDoesNotCarryTheServer(t *testing.T) {
	for _, module := range allRequirements(t, ".") {
		switch module {
		case "github.com/arandu-io/framework", "github.com/arandu-io/hesape":
			t.Errorf("%s is in this module's graph, and it is the half that runs on a device", module)
		}
	}
}

// TestTheServerHalfDoesNotCarryTheEngine is the same boundary from the other
// side.
//
// A server registers the publishing module to gain a native target. Pulling the
// drawing half in would put a GPU stack, a font shaper and nine window backends
// into a program that renders markup.
func TestTheServerHalfDoesNotCarryTheEngine(t *testing.T) {
	for _, module := range allRequirements(t, "module") {
		if module == "github.com/arandu-io/ayra" {
			t.Error("the publishing module requires the drawing half, which puts a GPU stack and nine window backends into every server that registers it")
		}
	}
}

// TestNothingIsPinnedToAPseudoVersion keeps a dependency on a commit out of a
// release.
//
// A pseudo-version is a commit somebody happened to be at. It is not a release
// anybody made, it carries no promise about what is in it, and the difference
// between two of them cannot be read from the version.
//
// The exception is a module that publishes no tags at all, where a
// pseudo-version is the only thing there is to require.
func TestNothingIsPinnedToAPseudoVersion(t *testing.T) {
	untagged := []string{
		// These publish no releases; a commit is all a require can name.
		"eliasnaur.com/font",
		"golang.org/x/exp",
		"golang.org/x/exp/shiny",
	}

	for _, dir := range []string{".", "module"} {
		for module, version := range requirements(t, dir) {
			if !pseudoVersion(version) || slices.Contains(untagged, module) {
				continue
			}
			t.Errorf("%s in %s is pinned to %s, which is a commit rather than a release", module, dir, version)
		}
	}
}

// TestPseudoVersionKnowsAllThreeShapes fixes the reading, because the gate
// above cannot be proven by editing a module file: a version nothing has ever
// published breaks the build before any test runs, so the check would be
// reported as passing on a tree that does not compile.
//
// The three shapes differ only in what sits before the timestamp, and an
// earlier version of this recognised the first and let the other two through --
// which are the ones that come from a repository that has released before, and
// therefore every dependency worth pinning.
func TestPseudoVersionKnowsAllThreeShapes(t *testing.T) {
	commits := []string{
		"v0.0.0-20260908205506-85c1c2202aba",         // no tag to build on
		"v0.26.1-0.20260101120000-abcdef123456",      // built on a release
		"v0.33.0-beta.0.20260101120000-abcdef123456", // built on a pre-release
	}
	for _, version := range commits {
		if !pseudoVersion(version) {
			t.Errorf("%s names a commit and was read as a release", version)
		}
	}

	releases := []string{
		"v0.26.0",
		"v1.0.0",
		"v0.32.0-beta.1",                        // a pre-release is a release somebody made
		"v2.0.0-rc.1",                           //
		"v0.0.0",                                //
		"v1.2.3-0.20260101",                     // truncated, and not a commit
		"v1.0.1-0.2026010112000-abcdef123456",   // thirteen digits of stamp
		"v1.0.1-0.20260101120000-zzzzzzzzzzzz",  // twelve characters, none of them a commit
		"v1.0.1-0.20260101120000-ABCDEF123456",  // the go command writes these in lower case
		"v1.0.1-0.20260101120000-abcdef12345",   // eleven digits of commit
		"v1.0.1-0.20260101120000-abcdef1234567", // thirteen
	}
	for _, version := range releases {
		if pseudoVersion(version) {
			t.Errorf("%s names a release and was read as a commit", version)
		}
	}
}

// pseudoVersion reports whether a version names a commit rather than a tag.
//
// The go command writes three shapes of these, and they differ in what sits
// before the timestamp: nothing when there was no tag to build on, and a base
// version when there was. What all three end with is the same, and is what this
// reads: fourteen digits of date and time, then twelve hexadecimal digits of
// commit.
//
// Matching only the first shape was the earlier version of this, and it let
// through exactly the ones that come from a repository that has released
// before -- which is every dependency worth pinning.
func pseudoVersion(version string) bool {
	parts := strings.Split(version, "-")
	if len(parts) < 3 {
		return false
	}

	stamp, commit := parts[len(parts)-2], parts[len(parts)-1]
	if len(stamp) < 14 || len(commit) != 12 {
		return false
	}
	return digits(stamp[len(stamp)-14:]) && hexadecimal(commit)
}

// digits reports whether every rune is 0 to 9.
func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// hexadecimal reports whether every rune is a lowercase hexadecimal digit,
// which is how the go command writes an abbreviated commit.
func hexadecimal(s string) bool {
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		isLetter := r >= 'a' && r <= 'f'
		if !isDigit && !isLetter {
			return false
		}
	}
	return true
}

// modFile is the shape of `go mod edit -json` this reads.
type modFile struct {
	Require []struct {
		Path     string
		Version  string
		Indirect bool
	}
}

// requirements answers every requirement of the module in dir, by path.
func requirements(t *testing.T, dir string) map[string]string {
	t.Helper()

	out, err := exec.Command("go", "mod", "edit", "-json", dir+"/go.mod").Output()
	if err != nil {
		t.Fatalf("the module file in %s could not be read: %v", dir, err)
	}

	var parsed modFile
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("the module file in %s could not be parsed: %v", dir, err)
	}

	versions := make(map[string]string, len(parsed.Require))
	for _, require := range parsed.Require {
		versions[require.Path] = require.Version
	}
	return versions
}

// directRequirements answers the modules in dir requires without the indirect
// mark: the ones this module asked for rather than inherited.
func directRequirements(t *testing.T, dir string) []string {
	t.Helper()

	out, err := exec.Command("go", "mod", "edit", "-json", dir+"/go.mod").Output()
	if err != nil {
		t.Fatalf("the module file in %s could not be read: %v", dir, err)
	}

	var parsed modFile
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("the module file in %s could not be parsed: %v", dir, err)
	}

	var direct []string
	for _, require := range parsed.Require {
		if !require.Indirect {
			direct = append(direct, require.Path)
		}
	}
	return direct
}

// allRequirements answers every module in dir's graph, direct or not.
func allRequirements(t *testing.T, dir string) []string {
	t.Helper()

	var all []string
	for module := range requirements(t, dir) {
		all = append(all, module)
	}
	slices.Sort(all)
	return all
}
