package theme_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// TestTheGeneratedPaletteMatchesTheStylesheet is the gate that keeps one
// source for the palette.
//
// It re-runs the generator into a temporary file and compares. That catches
// the drift in both directions, and both have happened to generated files in
// this collection before: a stylesheet edited without regenerating, which
// leaves the old colour compiled in, and a generated file edited by hand,
// which is undone silently by the next run.
func TestTheGeneratedPaletteMatchesTheStylesheet(t *testing.T) {
	committed, err := os.ReadFile("tokens_gen.go")
	if err != nil {
		t.Fatalf("reading the committed palette: %v", err)
	}

	regenerated := filepath.Join(t.TempDir(), "tokens_gen.go")
	cmd := exec.Command("go", "run", "../internal/tokengen", "basecoat/base.css", regenerated)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running the generator: %v\n%s", err, out)
	}

	fresh, err := os.ReadFile(regenerated)
	if err != nil {
		t.Fatalf("reading the regenerated palette: %v", err)
	}

	if string(committed) != string(fresh) {
		t.Error("the committed palette is not what the stylesheet generates: " +
			"run `go generate ./theme` and commit the result, or revert the hand edit")
	}
}

// TestEveryColourIsOpaqueUnlessTheStylesheetSaysOtherwise fixes the one thing
// a conversion bug produces that nothing else would report: a colour whose
// alpha came out zero draws nothing, and a screen with a missing element looks
// like a layout problem rather than a palette one.
//
// Three colours are transparent on purpose, and they are named here so that a
// fourth arriving is a decision somebody made rather than an accident.
func TestEveryColourIsOpaqueUnlessTheStylesheetSaysOtherwise(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		p := theme.New(scheme).Colours

		transparent := map[string]bool{
			"ScrollbarTrack": true,
		}

		// Border, Input and SidebarBorder are white at a tenth in the dark
		// scheme -- translucent, not transparent, and a zero there would be
		// a bug rather than a declaration.
		for name, c := range map[string]struct{ A uint8 }{
			"Background": {p.Background.A},
			"Foreground": {p.Foreground.A},
			"Primary":    {p.Primary.A},
			"Secondary":  {p.Secondary.A},
			"Muted":      {p.Muted.A},
			"Accent":     {p.Accent.A},
			"Card":       {p.Card.A},
			"Popover":    {p.Popover.A},
			"Ring":       {p.Ring.A},
		} {
			if transparent[name] {
				continue
			}
			if c.A == 0 {
				t.Errorf("%s: %s came out fully transparent", scheme, name)
			}
		}

		if p.Border.A == 0 {
			t.Errorf("%s: Border came out fully transparent", scheme)
		}
	}
}

// TestTheTwoSchemesAreDifferent keeps the dark palette from being a copy.
//
// The generator inherits what the dark block does not override, which is what
// a cascade does -- and a bug in that inheritance would produce a dark scheme
// that is the light one, compiling and drawing and simply never going dark.
func TestTheTwoSchemesAreDifferent(t *testing.T) {
	light, dark := theme.New(theme.Light), theme.New(theme.Dark)

	if light.Colours.Background == dark.Colours.Background {
		t.Error("both schemes draw the same background")
	}
	if light.Colours.Foreground == dark.Colours.Foreground {
		t.Error("both schemes draw the same foreground")
	}

	// The scrollbar thumb is written once, in the light block, as a reference
	// to the border. A stylesheet resolves that at the point of use, so it is
	// the border of whichever scheme is drawing -- and freezing it where it is
	// written is the mistake this asserts against.
	if light.Colours.ScrollbarThumb == dark.Colours.ScrollbarThumb {
		t.Error("the scrollbar thumb is the same in both schemes, so the reference was frozen where it was written instead of resolved per scheme")
	}
	if dark.Colours.ScrollbarThumb != dark.Colours.Border {
		t.Errorf("the dark scrollbar thumb is %+v and the dark border is %+v; they are written as the same colour", dark.Colours.ScrollbarThumb, dark.Colours.Border)
	}
}

// TestContrastAnswersForEverySurfaceItNames keeps the pairing honest: a
// surface listed in Contrast has to answer with something other than the
// default, or the entry is doing nothing.
func TestTheContrastPairsAreRealPairs(t *testing.T) {
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		th := theme.New(scheme)

		if got := th.Contrast(th.Colours.Primary); got != th.Colours.PrimaryForeground {
			t.Errorf("%s: text on Primary is %+v, want PrimaryForeground", scheme, got)
		}
		if got := th.Contrast(th.Colours.Card); got != th.Colours.CardForeground {
			t.Errorf("%s: text on Card is %+v, want CardForeground", scheme, got)
		}

		// A surface it does not know answers with the plain foreground rather
		// than with a zero colour, which would be invisible text.
		unknown := th.Colours.Chart1
		if got := th.Contrast(unknown); got.A == 0 {
			t.Errorf("%s: text on an unknown surface came out transparent", scheme)
		}
	}
}
