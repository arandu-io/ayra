// Package shell owns the window and the frame: what opens, what the loop does,
// and which screen is on top.
//
// It is the only package that talks to the operating system. Everything above
// it takes a context and answers dimensions, which is what makes a screen
// testable without a display and drawable twice in one frame.
package shell

import (
	"os"

	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// Config is what an application declares about its window.
type Config struct {
	// Title is what the window manager shows. Empty is refused rather than
	// defaulted: a window called "Untitled" in somebody's dock is a bug that
	// ships, and the name is one line to write.
	Title string

	// Width and Height are the window's opening size, in points. Zero takes a
	// size that fits a form on a laptop.
	Width, Height int

	// Scheme is the palette to open with.
	Scheme theme.Scheme

	// Fonts is the collection text is shaped from. Zero uses whatever the
	// system offers, which is nothing on a target with no filesystem -- so an
	// application that ships to the browser has to carry its faces.
	Fonts []text.FontFace
}

// Screen draws one page.
//
// It is the same shape as any other widget, which is deliberate: a screen is
// not a special kind of thing, it is the widget the window happens to be
// showing. That is what lets a catalogue draw six of them side by side.
type Screen func(ayra.Context) ayra.Dimensions

// Run opens the window and draws until it closes. It is the last line of a
// main, and on the platforms that need the first thread for their own loop it
// does not return at all: when the window closes, the process ends.
//
// The error it answers is therefore only the one it can answer before a window
// exists -- a configuration this package refuses. A window that fails to open
// on a machine with no GPU, and a window a person closes, both end the process,
// the first after writing why to standard error.
func Run(cfg Config, screen Screen) error {
	if cfg.Title == "" {
		return errNoTitle
	}
	cfg = cfg.withDefaults()

	shaper := text.NewShaper(text.WithCollection(cfg.Fonts))

	go func() {
		// A window that failed says so and exits non-zero; one that was closed
		// exits clean. Neither returns here, because the loop below never
		// gives this goroutine another turn.
		Exit(draw(cfg, shaper, screen))
		os.Exit(0)
	}()

	// The platform's loop has to run on the thread the process started on, and
	// it does not return. Everything above runs on another goroutine because
	// this line is where the main one is spent.
	app.Main()
	return nil
}

// withDefaults answers cfg with an opening size that fits a form on a laptop
// wherever one was not asked for.
//
// Zero is a size nobody chose, and a window opened at it is invisible; the
// numbers are here rather than at the call site so that a screenshot taken by
// two different applications is the same size.
func (c Config) withDefaults() Config {
	if c.Width <= 0 {
		c.Width = 960
	}
	if c.Height <= 0 {
		c.Height = 720
	}
	return c
}

// draw opens the window and runs the frame loop until it is destroyed.
//
// It answers what the window closed with, which is nil when a person closed it
// and the platform's failure when it could never be shown.
func draw(cfg Config, shaper *text.Shaper, screen Screen) error {
	var window app.Window
	window.Option(
		app.Title(cfg.Title),
		app.Size(unit.Dp(cfg.Width), unit.Dp(cfg.Height)),
	)

	palette := theme.New(cfg.Scheme)

	var ops op.Ops
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			c := ayra.Context{
				Context:    gtx,
				Theme:      palette,
				Shaper:     shaper,
				Invalidate: window.Invalidate,
			}

			// The window is filled before the screen draws, and it is filled
			// here rather than left to the screen. A screen that forgot would
			// get whatever the platform left in the buffer -- which is white
			// on most of them, so the dark scheme would draw its light text on
			// a light ground and be a screen nobody can read. Every screen
			// would have to remember, and one of them would not.
			c.Fill(palette.Colours.Background)

			screen(c)
			e.Frame(gtx.Ops)
		}
	}
}

// errNoTitle is what an unnamed window answers with.
var errNoTitle = errNoTitleType{}

type errNoTitleType struct{}

func (errNoTitleType) Error() string {
	return "shell: the window has no title, and a window with no title is what a person sees in their dock"
}

// Exit ends the process with the code a failed window deserves.
//
// It is here rather than left to the caller because the answer is the same
// every time and getting it wrong is silent: a main that returns after a
// window failed to open exits zero, and whatever started it believes the
// application ran.
func Exit(err error) {
	if err == nil {
		return
	}
	os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(1)
}
