// Package shell owns the window and the frame: what opens, what the loop does,
// and which screen is on top.
//
// It is the only package that talks to the operating system. Everything above
// it takes a context and answers dimensions, which is what makes a screen
// testable without a display and drawable twice in one frame.
package shell

import (
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/arandu-io/ayra/engine/app"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
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

	// Scheme is the palette to open with. With Mode [theme.System] it is the one
	// the window keeps until the system says which it is in, and the one it
	// stays in on a platform that cannot say.
	Scheme theme.Scheme

	// Mode says where the scheme comes from. Zero is [theme.Fixed]: the window
	// draws Scheme until a screen asks for another, as before. [theme.System]
	// follows the operating system -- the window opens in its scheme and
	// changes with it while the application runs, without being reopened.
	//
	// A choice made with [ayra.Context.ChangeScheme] wins over the system while
	// it stands, and [ayra.Context.FollowSystem] hands the decision back.
	Mode theme.Mode

	// Locale is the language of the calendar's words: month and weekday names,
	// the first day of the week and the labels of its controls. Zero draws
	// English. It is laid over the theme Theme builds, so a stylesheet-made
	// theme does not have to carry it.
	Locale theme.Locale

	// OnLifecycle is told when the application goes to the background, comes
	// back to the front and is destroyed. It runs on the goroutine that draws,
	// between two frames, so it must not block: it starts or stops what runs for
	// the screen -- a socket, a video -- and returns. A window opens in the
	// foreground, so the first call is a return from the background, and a
	// notice that repeats the stage the application is already in is not made.
	OnLifecycle func(ayra.Lifecycle)

	// ReduceMotion answers whether the person has asked for less movement, and
	// is asked once per frame, so it must be cheap and must not block. Nil takes
	// the operating system's own setting, [SystemReduceMotion]: an application
	// needs this only to decide for itself, with a switch in its own settings.
	ReduceMotion func() bool

	// Theme builds the theme for a scheme. Zero takes [theme.New], the Basecoat
	// palette; a product that has its own -- made from its stylesheet by
	// theme/css, with its look and its variants registered -- hands the function
	// that returns it, so the window and everything drawn in it read the
	// product's tokens instead of the default ones.
	Theme func(theme.Scheme) theme.Theme

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

	// A typeface the scale names and the collection lacks does not fail when
	// text is shaped: it falls back, and a product whose text is in the wrong
	// face finds out by looking. Said here, it is a mistake with a name. A
	// collection left empty is the system's, which cannot be checked.
	if len(cfg.Fonts) > 0 {
		if missing := cfg.theme(cfg.Scheme).Type.Missing(cfg.Fonts); len(missing) > 0 {
			return &MissingFaceError{Faces: missing}
		}
	}

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

// theme answers the theme the window draws with.
func (c Config) theme(scheme theme.Scheme) theme.Theme {
	th := theme.New(scheme)
	if c.Theme != nil {
		th = c.Theme(scheme)
	}
	if !c.Locale.IsZero() {
		th.Locale = c.Locale
	}
	return th
}

// MissingFaceError is what Run answers when the theme's scale names a typeface
// that the configured fonts do not carry.
type MissingFaceError struct {
	// Faces are the typefaces the theme names and Fonts lacks.
	Faces []string
}

// Error names the faces.
func (e *MissingFaceError) Error() string {
	return "ayra/shell: the theme names the typefaces " + strings.Join(e.Faces, ", ") +
		", and the fonts the window was configured with do not carry them"
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
	return loop(&window, cfg, shaper, screen)
}

// host is the window as the frame loop sees it. It is [app.Window], and it is an
// interface so that the path from a platform's event to the palette on screen
// can be walked in a test with a window that is a list of events.
type host interface {
	Event() event.Event
	Option(...app.Option)
	Invalidate()
}

// loop draws until the window is destroyed.
func loop(window host, cfg Config, shaper *text.Shaper, screen Screen) error {
	state := newSchemes(cfg.Mode, cfg.Scheme)
	stage := newStages(cfg.OnLifecycle)
	palette := cfg.theme(state.scheme)

	// The bars of the system and the ground behind the window are painted with
	// the background of the palette, and the icons on them are dark on a light
	// ground and light on a dark one. They are set when a palette is first
	// drawn and again whenever it changes, and only then: an option sent every
	// frame is a configure call every frame.
	var barsFor *theme.Scheme

	// A change of scheme is asked for during a frame and applied at the start of
	// the next, so that one frame is never drawn half in each palette.
	changed := func(did bool) {
		if did {
			window.Invalidate()
		}
	}

	var keyboard keyboardWatch
	var ops op.Ops
	for {
		switch e := window.Event().(type) {
		case app.DestroyEvent:
			stage.destroyed()
			return e.Err
		case app.StageEvent:
			stage.observe(e.Stage)
		case app.ConfigEvent:
			// The system's scheme travels with the window's configuration. It
			// is recorded whatever the mode, so that handing the decision back
			// to the system finds the scheme it is in now and not the one it
			// was in when the window opened.
			changed(state.observe(e.Config.Appearance))
		case app.FrameEvent:
			if state.scheme != palette.Scheme {
				palette = cfg.theme(state.scheme)
			}
			if barsFor == nil || *barsFor != palette.Scheme {
				scheme := palette.Scheme
				barsFor = &scheme
				window.Option(
					app.StatusColor(palette.Colours.Background),
					app.NavigationColor(palette.Colours.Background),
				)
			}
			gtx := app.NewContext(&ops, e)
			paintBehindBars(gtx.Ops, e, palette.Colours.Background)
			overlay := &ayra.Overlay{
				Window: image.Rect(
					-e.Metric.Dp(e.Insets.Left), -e.Metric.Dp(e.Insets.Top),
					e.Size.X-e.Metric.Dp(e.Insets.Left), e.Size.Y-e.Metric.Dp(e.Insets.Top),
				),
				Safe: image.Rectangle{Max: gtx.Constraints.Max},
				// How much of the bottom of the window is the keyboard's.
				Keyboard: e.Metric.Dp(keyboard.observe(e.Insets.Bottom)),
			}
			c := ayra.Context{
				Overlay:      overlay,
				Context:      gtx,
				Theme:        palette,
				Shaper:       shaper,
				Invalidate:   window.Invalidate,
				Mode:         state.mode,
				ReduceMotion: cfg.reduceMotion(),
				SetScheme: func(scheme theme.Scheme) {
					changed(state.choose(scheme))
				},
				SetMode: func(mode theme.Mode) {
					changed(state.setMode(mode))
				},
			}

			// The window is filled before the screen draws, and it is filled
			// here rather than left to the screen. A screen that forgot would
			// get whatever the platform left in the buffer -- which is white
			// on most of them, so the dark scheme would draw its light text on
			// a light ground and be a screen nobody can read. Every screen
			// would have to remember, and one of them would not.
			c.Fill(palette.Colours.Background)

			screen(c)
			overlay.Flush(gtx.Ops)
			e.Frame(gtx.Ops)
		}
	}
}

// paintBehindBars paints the whole window, the strips under the system bars
// included, with the background of the palette.
//
// The context a screen is handed has the insets taken off: it starts below the
// status bar and ends above the navigation bar, and what it paints stays inside
// that. The strips outside it belong to nobody, and the surface is cleared to
// white there -- which is what a phone in the dark mode showed above and below
// an otherwise dark screen, whatever colour the bars were set to, because from
// Android 15 on the bars are transparent and the window is what shows through.
// The offset the insets applied is undone for the length of the fill.
func paintBehindBars(ops *op.Ops, e app.FrameEvent, ground color.NRGBA) {
	if e.Insets == (app.Insets{}) {
		return
	}
	defer op.Offset(image.Pt(-e.Metric.Dp(e.Insets.Left), -e.Metric.Dp(e.Insets.Top))).Push(ops).Pop()
	defer clip.Rect{Max: e.Size}.Push(ops).Pop()
	paint.ColorOp{Color: ground}.Add(ops)
	paint.PaintOp{}.Add(ops)
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
