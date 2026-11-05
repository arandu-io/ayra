package shell

import (
	"testing"

	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/unit"
)

// TestAnUnaskedSizeOpensAtSomethingVisible fixes the numbers rather than the
// fact that there are numbers.
//
// A zero here is not a small window, it is a window with no area, and the
// platform is entitled to show nothing at all -- which reads as "the
// application did not start" to whoever ran it.
func TestAnUnaskedSizeOpensAtSomethingVisible(t *testing.T) {
	got := Config{Title: "Catalogue"}.withDefaults()

	if got.Width != 960 || got.Height != 720 {
		t.Errorf("an unasked size opened at %dx%d, want 960x720", got.Width, got.Height)
	}
	if got.Title != "Catalogue" {
		t.Errorf("the title was changed to %q", got.Title)
	}
}

// TestAnAskedSizeIsKept keeps the default from being a floor.
//
// A window asked to open small is usually a window being placed beside
// something else, and growing it would move whatever the person was lining it
// up with.
func TestAnAskedSizeIsKept(t *testing.T) {
	got := Config{Title: "Catalogue", Width: 320, Height: 240}.withDefaults()

	if got.Width != 320 || got.Height != 240 {
		t.Errorf("an asked size opened at %dx%d, want 320x240", got.Width, got.Height)
	}
}

// TestOneAskedDimensionDoesNotCarryTheOther fixes each axis on its own.
//
// A width given with no height is the common shape of the mistake, and filling
// both from the pair would silently widen the window the caller did ask about.
func TestOneAskedDimensionDoesNotCarryTheOther(t *testing.T) {
	wide := Config{Title: "Catalogue", Width: 320}.withDefaults()
	if wide.Width != 320 || wide.Height != 720 {
		t.Errorf("a width alone opened at %dx%d, want 320x720", wide.Width, wide.Height)
	}

	tall := Config{Title: "Catalogue", Height: 240}.withDefaults()
	if tall.Width != 960 || tall.Height != 240 {
		t.Errorf("a height alone opened at %dx%d, want 960x240", tall.Width, tall.Height)
	}
}

// TestANegativeSizeIsTreatedAsUnasked keeps a subtraction from opening a window
// the platform has to interpret.
func TestANegativeSizeIsTreatedAsUnasked(t *testing.T) {
	got := Config{Title: "Catalogue", Width: -1, Height: -1}.withDefaults()

	if got.Width != 960 || got.Height != 720 {
		t.Errorf("a negative size opened at %dx%d, want 960x720", got.Width, got.Height)
	}
}

// open applies the options a window opens with to an empty configuration, as
// the platform does.
func open(c Config) app.Config {
	var cnf app.Config
	for _, o := range c.windowOptions() {
		o(unit.Metric{PxPerDp: 1, PxPerSp: 1}, &cnf)
	}
	return cnf
}

// TestAWindowIsWindowedUnlessFullscreenIsAsked fixes both answers.
func TestAWindowIsWindowedUnlessFullscreenIsAsked(t *testing.T) {
	plain := open(Config{Title: "Kit", Width: 1280, Height: 800})
	if plain.Mode != app.Windowed {
		t.Errorf("mode %v, want windowed", plain.Mode)
	}

	full := open(Config{Title: "Kit", Width: 1280, Height: 800, Fullscreen: true})
	if full.Mode != app.Fullscreen {
		t.Errorf("mode %v, want fullscreen: the size option puts the window back to windowed, so the mode must come after it", full.Mode)
	}
	if full.Title != "Kit" || full.Size.X != 1280 || full.Size.Y != 800 {
		t.Errorf("title %q size %v: fullscreen must keep both, the size is what leaving fullscreen goes back to", full.Title, full.Size)
	}
}
