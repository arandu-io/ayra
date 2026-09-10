package shell_test

import (
	"strings"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/shell"
)

// TestAWindowWithNoTitleIsRefused fixes the one thing a default would hide.
//
// Defaulting to the binary's name, or to "Untitled", produces a window that
// opens and is wrong -- and it is wrong in somebody's dock, on a screenshot,
// in a bug report. Refusing costs one line to fix and the message says which
// line.
func TestAWindowWithNoTitleIsRefused(t *testing.T) {
	err := shell.Run(shell.Config{}, func(ayra.Context) ayra.Dimensions { return ayra.Dimensions{} })

	if err == nil {
		t.Fatal("a window with no title opened")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("the refusal does not say what is missing: %v", err)
	}
}

// TestRunRefusesBeforeTouchingThePlatform keeps the refusal above the window.
//
// A check that ran after the platform was asked for a window would report the
// missing title on a machine with a display and the missing GPU on one
// without, for the same mistake -- and the second message sends whoever reads
// it looking at the wrong thing.
func TestRunRefusesBeforeTouchingThePlatform(t *testing.T) {
	drawn := false

	err := shell.Run(shell.Config{}, func(ayra.Context) ayra.Dimensions {
		drawn = true
		return ayra.Dimensions{}
	})

	if err == nil {
		t.Fatal("the refusal did not happen")
	}
	if drawn {
		t.Error("the screen was drawn before the configuration was checked")
	}
}
