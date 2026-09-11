// Command catalogue draws every control this library has, in both palettes.
//
// It is a module of its own, and the module file is the point of it. A program
// carries its whole graph behind it, so a command kept inside the library would
// put everything it needs to run -- the window it opens, the faces it shapes
// text with, the data it invents to fill a table -- into the requirements of
// every project that imported a button. Separate, installing the library
// installs the library.
//
// What it is for is comparison. A control drawn on its own looks correct;
// sixty drawn beside each other is where a border a point thicker than its
// neighbour, a label set in the wrong weight, or a colour that reads on one
// palette and vanishes on the other becomes visible. That is also why the two
// palettes are switched while it runs rather than chosen when it opens: the
// eye compares what it can put side by side in memory, and quitting between
// them loses the comparison.
package main

import (
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/shell"
	"github.com/arandu-io/ayra/theme"
)

func main() {
	// One scheme, read by both: the window paints its ground from it and the
	// catalogue starts its switch on it. Written twice, the application opens
	// on one palette and repaints in the other before anybody has touched it.
	opening := theme.Light

	// The faces are carried rather than taken from the machine. A control is
	// measured from the glyphs it draws, so a catalogue shaped with whatever a
	// particular desktop happens to have installed is a catalogue whose spacing
	// is that desktop's -- and the point of it is to show what the library
	// draws, not what one computer had to hand.
	shell.Exit(shell.Run(shell.Config{
		Title:  "Controls",
		Width:  1200,
		Height: 820,
		Scheme: opening,
		Fonts:  gofont.Collection(),
	}, New(opening).Layout))
}
