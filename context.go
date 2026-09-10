// Package ayra draws an Arandu application as a native one.
//
// A screen is a function: it takes what it needs to draw and answers how much
// room it took. Nothing here renders markup, loads a stylesheet or runs a
// script, and nothing here decides anything -- the server a screen draws for
// is the one that authorizes, validates and answers, and this side asks it the
// same way a browser would.
package ayra

import (
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"

	"github.com/arandu-io/ayra/theme"
)

// Context is what a screen is handed, and everything it is allowed to read.
//
// It carries the theme by value rather than pointing at a current one, so two
// schemes can be drawn side by side -- which is what a catalogue does, and
// what a person comparing them needs.
//
// The embedded layout context is the frame: the room available, the pixel
// density, and the list the drawing is appended to. It is embedded rather than
// wrapped because a control passes it down a hundred times per frame, and a
// wrapper would be a hundred conversions to say nothing.
type Context struct {
	layout.Context

	// Theme is the palette, the lengths and the type scale.
	Theme theme.Theme

	// Shaper turns a string into glyphs. It is here rather than looked up
	// because shaping is the one thing on this path that is expensive enough
	// to want reused, and a control that made its own would rebuild the font
	// atlas on every frame.
	Shaper *text.Shaper

	// Invalidate is what the window was given to ask for another frame with.
	//
	// It is set by whoever opened the window and read by [Context.Redraw],
	// which is what a screen calls. A context built outside a window -- in a
	// test, in a tool that renders one frame -- leaves it nil, and Redraw then
	// does nothing, which is the right answer when there is no window to
	// redraw.
	Invalidate func()
}

// Redraw asks for another frame, and is what work finishing outside one calls.
//
// A screen is drawn in response to input. Anything that finishes while nobody
// is touching the device -- an answer from the server, a timer, a file that
// finished loading -- changes what should be on screen and nothing draws it:
// the result sits in memory, correct and invisible, until the next stray
// click. This is what makes it appear.
//
// It is safe to call from another goroutine, and that is where the work
// belongs. The frame loop must not wait on a network: a screen that blocks in
// its draw stops answering the window, and the platform reports an application
// that has stopped responding.
func (c Context) Redraw() {
	if c.Invalidate != nil {
		c.Invalidate()
	}
}

// Dimensions is how much room something took.
type Dimensions = layout.Dimensions

// Widget is anything that draws itself and says how much room it took.
//
// It takes the Ayra context rather than the engine's, so a control cannot be
// written that draws without a theme -- which is the mistake that produces a
// screen where one control is themed and the next is not.
type Widget func(Context) Dimensions

// With returns a copy of the context drawing under different constraints.
//
// The theme and the shaper travel; only the frame changes. It exists so that a
// layout can hand a child less room without a control having to know how the
// engine spells that.
func (c Context) With(gtx layout.Context) Context {
	c.Context = gtx
	return c
}
