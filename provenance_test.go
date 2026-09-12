package ayra_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// How much of this repository is still somebody else's, counted rather than
// claimed.
//
// The engine under engine/ began as a copy, and it is being rewritten. The
// question that matters during that work -- how far along is it -- has exactly
// one honest answer, and it is not a label at the top of a file saying which
// state somebody believed that file was in. A label is written by whoever edits
// the file and verified by nobody, and the failure it invites is the quiet one:
// a file rewritten badly, marked as ours, and counted as ours by everything
// downstream.
//
// So this counts. It reads the original at the version the copy was taken from,
// normalises both sides, and reports how many files are still the same work.
// The number goes down as the rewrite lands, it is printed on every run, and it
// cannot go up.

// original is the version engine/ was copied from. It is a constant because the
// comparison is meaningless against a moving target: a file that stopped
// matching because the other side moved is not a file that was rewritten here.
const original = "gioui.org@v0.10.2"

// stillTheirs is the number of files that are still the original, and it is a
// ceiling. It may fall -- that is the work -- and a rise fails, which is what
// stops the decision from being undone on an afternoon when something does not
// compile.
const stillTheirs = 74

// comparable is how many files of the copy carry code and have a counterpart to
// compare against. It is asserted so that a walk which stopped matching reports
// a smaller universe instead of a smaller count, which would read as progress.
const comparable = 129

// A file with no code in it is not counted, and leaving it in was a fault in
// this measurement rather than a detail of it.
//
// Seventeen files under engine/ are a package clause and nothing else: the
// documentation of a package, or a declaration that exists only to carry a
// build constraint. Normalising strips comments, so both sides of such a file
// reduce to the same few characters no matter what is written in it -- one of
// them had every one of its two hundred lines replaced and still counted as
// untouched, and would have counted that way if it were rewritten ten times.
//
// Counting them made the number say that work nobody can do is work not yet
// done. They are left out, and the number that remains is one that reaching
// zero is possible for.
func hasCode(source string) bool {
	stripped := strings.TrimSpace(normalise(source))
	return !packageOnly.MatchString(stripped)
}

var packageOnly = regexp.MustCompile(`^package[A-Za-z_][A-Za-z0-9_]*$`)

// TestHowMuchOfThisIsStillSomebodyElses is the number, and the ratchet.
func TestHowMuchOfThisIsStillSomebodyElses(t *testing.T) {
	source := originalTree(t)
	root := moduleRoot(t)
	engine := filepath.Join(root, "engine")

	var same, seen int
	var theirs []string

	err := filepath.WalkDir(engine, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipped(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		relative, err := filepath.Rel(engine, path)
		if err != nil {
			return err
		}
		counterpart := filepath.Join(source, relative)
		if _, err := os.Stat(counterpart); err != nil {
			// Written here, or moved: nothing to compare against, and that
			// counts as ours by construction rather than by assertion.
			return nil
		}

		ours, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		original, err := os.ReadFile(counterpart)
		if err != nil {
			return err
		}
		if !hasCode(string(ours)) && !hasCode(string(original)) {
			// Nothing to measure on either side. See hasCode.
			return nil
		}
		seen++

		if normalise(string(ours)) == normalise(string(original)) {
			same++
			theirs = append(theirs, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if seen != comparable {
		t.Fatalf("%d files had a counterpart to compare against and %d were expected; "+
			"a walk that matches fewer files reports a smaller count, which reads like progress and is not",
			seen, comparable)
	}

	t.Logf("%d of %d files under engine/ are still the original", same, seen)

	if same > stillTheirs {
		t.Errorf("%d files are still the original, up from %d. The rewrite only moves forward: %v",
			same, stillTheirs, added(theirs, same-stillTheirs))
	}
	if same < stillTheirs {
		t.Errorf("%d files are still the original, down from %d -- which is the work. "+
			"Lower the constant in this file to %d, in the same commit that earned it",
			same, stillTheirs, same)
	}
}

// normalise reduces a file to the work in it.
//
// Whitespace goes because formatting is not authorship. Comments go for the
// reason this whole file exists: a rewrite that only replaces the prose above
// each function has changed nothing that runs, and a comparison that counted it
// as ours would be the label problem again with extra steps. Import paths are
// unified because moving a package into this module's namespace was a rename,
// not a rewrite.
func normalise(source string) string {
	source = blockComment.ReplaceAllString(source, "")
	source = lineComment.ReplaceAllString(source, "")
	source = importPath.ReplaceAllString(source, "X")
	return whitespace.ReplaceAllString(source, "")
}

var (
	blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	lineComment  = regexp.MustCompile(`(?m)//.*$`)
	importPath   = regexp.MustCompile(`github\.com/arandu-io/ayra/engine|gioui\.org`)
	whitespace   = regexp.MustCompile(`\s+`)
)

// added names a few of the files that regressed, because a count alone does not
// say where to look.
func added(files []string, count int) []string {
	if count > 5 {
		count = 5
	}
	if count > len(files) {
		count = len(files)
	}
	return files[:count]
}

// originalTree locates the version the copy was taken from.
//
// It is absent on a machine that never downloaded it, and the two situations
// that produces are not the same. Locally, skipping is right: somebody running
// the suite to check a control should not need a module they do not import. In
// CI it is a failure, because a gate that skips is a gate that does not exist,
// and the workflow fetches it for exactly this reason.
func originalTree(t *testing.T) string {
	t.Helper()

	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("the module cache could not be located: %v", err)
	}
	tree := filepath.Join(strings.TrimSpace(string(out)), original)

	if _, err := os.Stat(tree); err != nil {
		if ci, _ := strconv.ParseBool(os.Getenv("CI")); ci {
			t.Fatalf("%s is not in the module cache, so nothing was compared. "+
				"The workflow fetches it before the suite runs; a gate that skips is a gate that does not exist",
				original)
		}
		t.Skipf("%s is not in the module cache; run `go mod download %s` to count locally", original, original)
	}
	return tree
}
