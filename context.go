// Package ayra draws an Arandu application as a native one.
//
// A screen is a function: it takes what it needs to draw and answers how much
// room it took. Nothing here renders markup, loads a stylesheet or runs a
// script, and nothing here decides anything -- the server a screen draws for
// is the one that authorises, validates and answers, and this side asks it the
// same way a browser would.
package ayra

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
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

	// SetScheme is what a control that changes the palette -- the light and dark
	// toggle -- calls. The window applies it from the next frame, and the whole
	// application is drawn in the other scheme with no work from the screens.
	//
	// It is nil outside a window, which is what a test or a tool that renders
	// one frame leaves it, and [Context.ChangeScheme] then does nothing: there is
	// no window whose palette could change.
	SetScheme func(theme.Scheme)

	// Mode is where the scheme being drawn comes from: [theme.System] while the
	// window follows the operating system, [theme.Fixed] when a person or the
	// application chose one. A control that offers Automático, Claro and Escuro
	// reads it to mark the current choice.
	Mode theme.Mode

	// ReduceMotion is true while the person has asked the system for less
	// movement. A control that animates -- the skeleton's pulse, the indeterminate
	// bar's trip -- draws its resting picture and asks for no more frames while it
	// is set. It is false outside a window, and a window takes it from the operating
	// system unless the application said otherwise: see shell.Config.ReduceMotion
	// and shell.SystemReduceMotion.
	ReduceMotion bool

	// SetMode is what [Context.FollowSystem] calls. The window applies it from
	// the next frame, like SetScheme. It is nil outside a window.
	SetMode func(theme.Mode)

	// Overlay is the layer drawn over the whole window, after the screen. A
	// dialog or a drawer put in it covers the window whatever box it was called
	// from: the bars of the application and the strips under the system bars
	// included. It is nil outside a window, which is what a test or a tool that
	// renders one panel leaves it, and what draws over the window then draws
	// over the room it was given.
	Overlay *Overlay
}

// KeyboardOpen reports whether the software keyboard is up. A bar across the foot
// of the screen -- the tabs of an application -- is not drawn while it is: it
// stays over the keyboard and takes the room the field being typed into needs,
// as no Android application's does. It is false outside a window.
func (c Context) KeyboardOpen() bool { return c.Overlay != nil && c.Overlay.Keyboard > 0 }

// Overlay is what is drawn over the window once the screen is done.
//
// A control cannot cover the window from inside a box: it is laid out in the
// room its parent gave it, and a dialog opened from a card was a dialog the
// size of the card, with the application bar and the tab bar outside its scrim.
// So it records what it draws and hands it here, and the window plays the
// recordings back at the end of the frame in its own coordinates, in the order
// they were handed over -- the later is on top.
type Overlay struct {
	// Window is the whole window, the strips under the system bars included, in
	// the coordinates the screen is drawn in: its origin is the corner of the
	// room the screen was given, so the part under the status bar is negative.
	Window image.Rectangle
	// Safe is the room the screen was given: the window less the system bars
	// and the keyboard. What has to be reached and read is laid out in it.
	Safe image.Rectangle
	// Keyboard is the height of the software keyboard in pixels, zero while it
	// is down. It is part of what Safe leaves out at the bottom.
	Keyboard int

	calls []op.CallOp
}

// Add hands over a recording to draw over the window. It is replayed with no
// offset: what was recorded is in the window's coordinates, which is what
// Window and Safe are written in.
func (o *Overlay) Add(call op.CallOp) { o.calls = append(o.calls, call) }

// Flush plays the recordings back, in order, and forgets them. The window calls
// it once, after the screen, at the origin the screen was drawn from.
func (o *Overlay) Flush(ops *op.Ops) {
	for _, call := range o.calls {
		call.Add(ops)
	}
	o.calls = o.calls[:0]
}

// ChangeScheme asks the window to draw in another palette, and keeps it there:
// it is a choice, and a choice wins over the system's setting for as long as it
// stands, so a window that followed the system stops following it until
// [Context.FollowSystem] is called. It is what the theme toggle does.
func (c Context) ChangeScheme(scheme theme.Scheme) {
	if c.SetScheme != nil {
		c.SetScheme(scheme)
	}
}

// FollowSystem asks the window to draw in the scheme of the operating system
// again, and to change with it, which is the web panel's Automático. It undoes
// a [Context.ChangeScheme]. On a platform that cannot tell the system's scheme
// the window keeps the one it has.
func (c Context) FollowSystem() {
	if c.SetMode != nil {
		c.SetMode(theme.System)
	}
}

// Fill paints the whole area this context was given.
//
// It is what puts a ground under a screen, and it is separate from drawing the
// screen because the two answer to different things: the ground belongs to the
// window and the palette, and the screen belongs to the application. A screen
// left to fill its own would work until one of them forgot, and the one that
// forgot would show light text on whatever the platform happened to leave in
// the buffer.
func (c Context) Fill(colour color.NRGBA) {
	defer clip.Rect{Max: c.Constraints.Max}.Push(c.Ops).Pop()
	paint.ColorOp{Color: colour}.Add(c.Ops)
	paint.PaintOp{}.Add(c.Ops)
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
