package mask_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTheCopyIsTheOriginal keeps mask.go, patterns.go and doc.go the same bytes
// as hesape/mask, which is where the rule lives.
//
// A formatting rule written twice is a rule that disagrees with itself the first
// time one copy is edited. This module cannot require hesape -- the drawing half
// carries no server -- so the rule is copied, and this is what makes the copy a
// copy and not a fork: when the original is in the workspace beside this
// repository, any difference fails.
//
// Without the workspace there is nothing to compare, and the test says so
// rather than passing: a skipped comparison is a different thing from a
// matching one.
func TestTheCopyIsTheOriginal(t *testing.T) {
	original := filepath.Join("..", "..", "hesape", "mask")
	if _, err := os.Stat(original); err != nil {
		t.Skipf("hesape is not in the workspace beside this repository (%v): the copy was not compared", err)
	}

	for _, name := range []string{"mask.go", "patterns.go", "doc.go"} {
		want, err := os.ReadFile(filepath.Join(original, name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(want) != string(got) {
			t.Errorf("mask/%s is not hesape/mask/%s: change the rule there, and copy it here", name, name)
		}
	}
}
