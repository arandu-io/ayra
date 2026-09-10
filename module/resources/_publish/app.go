// Package native is this application's native target: the window it opens, the
// server it draws for, and which screen is showing.
//
// It is yours from the moment it was published. Nothing upstream reads it and
// nothing regenerates it, so a screen added here stays added and a decision
// changed here stays changed.
//
// Every file here is one package, deliberately. A screen in a package of its
// own would have to be imported by its full path, and that path begins with
// the module name of this project -- which whoever published these files could
// not know. One package needs no such import.
package native

import (
	"context"
	"sync"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/shell"
	"github.com/arandu-io/ayra/theme"
)

// Config is what this application declares about its native target.
type Config struct {
	// Server is the address of the Arandu server this draws for. There is no
	// default: a native binary that guessed would ship pointing at whichever
	// machine it was built on.
	Server string

	// Title is what the window is called.
	Title string

	// Width and Height are the opening size, in points. Zero takes the
	// window's own default.
	Width, Height int

	// Scheme is the palette to open with.
	Scheme theme.Scheme

	// Fonts are the faces text is shaped from. A binary for the browser has to
	// carry its own, because there is no filesystem there to find any.
	Fonts []text.FontFace
}

// Run opens the window and draws until it closes. It is the whole of a native
// main:
//
//	func main() {
//		shell.Exit(native.Run(native.Config{
//			Server: "https://example.com",
//			Title:  "My Application",
//		}))
//	}
func Run(cfg Config) error {
	server, err := client.New(cfg.Server)
	if err != nil {
		return err
	}

	app := &App{server: server}

	return shell.Run(shell.Config{
		Title:  cfg.Title,
		Width:  cfg.Width,
		Height: cfg.Height,
		Scheme: cfg.Scheme,
		Fonts:  cfg.Fonts,
	}, app.Layout)
}

// App is what is on screen and what it is talking to.
//
// One value for the whole application, because a native process has one window
// and one session: the server it signed in to is the server every screen draws
// from, and a second one would be a second identity nobody asked for.
type App struct {
	server *client.Client

	// screen is what is showing, and signIn and home are the screens' own
	// state: what is typed, what is focused, what was pressed. It lives here
	// because a native screen is drawn many times a second and its state has
	// to outlive the frame -- unlike a page, which is rebuilt from the request
	// every time.
	//
	// Everything in this group is touched only while drawing, and drawing
	// happens on one goroutine. That is what makes it safe without a lock, and
	// it is why the work below hands its result back instead of writing here.
	screen Screen
	signIn signInState
	home   homeState

	// The three fields below are shared with the goroutine doing the work, so
	// all three are guarded.
	mu      sync.Mutex
	pending bool
	answer  *answer
	failure error
}

// answer is what a finished request left for the next frame to apply.
//
// The request runs on its own goroutine and does not touch the screen, the
// fields or the controls: applying it there would race the drawing, and the
// race would be a caret that jumps or a control that reads two states in one
// frame. It leaves the page here instead, and the frame applies it.
type answer struct {
	page client.Page
}

// Screen names what is showing.
type Screen int

// The screens this application has. Add one here and in [App.Layout].
const (
	// SignIn is the first screen, because the server authorizes every page:
	// until there is a session, there is nothing to draw.
	SignIn Screen = iota
	// Home is what a signed-in person sees.
	Home
)

// Layout draws whichever screen is showing.
func (a *App) Layout(c ayra.Context) ayra.Dimensions {
	st := a.apply(c)

	// A switch and not a table, because a switch is what the compiler
	// complains about when a screen is added and forgotten.
	switch a.screen {
	case Home:
		return a.layoutHome(c, st)
	default:
		return a.layoutSignIn(c, st)
	}
}

// status is what the screens are told about work in flight.
type status struct {
	// Busy is true while something is being asked of the server. A screen
	// draws its controls as unavailable rather than letting a second request
	// go out on top of the first.
	Busy bool
	// Failure is what the last attempt answered with, and nil when it worked.
	Failure error
}

// apply takes whatever finished since the last frame and reads the shared
// state once.
//
// Once, and not per control: reading it three times in one frame can see three
// different answers, and a screen that says "signing in" beside a button that
// is already enabled is a screen nobody can explain.
func (a *App) apply(c ayra.Context) status {
	a.mu.Lock()
	finished := a.answer
	a.answer = nil
	st := status{Busy: a.pending, Failure: a.failure}
	a.mu.Unlock()

	if finished != nil {
		if err := a.show(finished.page); err != nil {
			st.Failure = err

			a.mu.Lock()
			a.failure = err
			a.mu.Unlock()
		}
	}
	return st
}

// ask runs one request to the server and redraws when it answers.
//
// The work happens on its own goroutine because the frame loop must not wait
// on a network: a screen that blocks in its draw stops answering the window,
// and the platform reports an application that has stopped responding. The
// redraw at the end is what makes the answer visible -- nobody is touching the
// device while it is in flight, so nothing else would draw it.
func (a *App) ask(c ayra.Context, work func(context.Context) (client.Page, error)) {
	a.mu.Lock()
	if a.pending {
		// A second press while the first is still out. Ignoring it is what the
		// disabled control already says; obeying it would sign in twice.
		a.mu.Unlock()
		return
	}
	a.pending = true
	a.failure = nil
	a.mu.Unlock()

	redraw := c.Redraw
	go func() {
		page, err := work(context.Background())

		a.mu.Lock()
		a.pending = false
		a.failure = err
		if err == nil {
			a.answer = &answer{page: page}
		}
		a.mu.Unlock()

		redraw()
	}()
}

// show picks the screen for a page the server answered with.
//
// The server names the page and this maps that name to a screen. The mapping
// is here rather than at each call site because one address can answer with
// more than one page: a form that failed validation answers with the form, and
// signing out answers with the page signing out leads to.
func (a *App) show(page client.Page) error {
	switch page.View {
	case "auth.login":
		if a.screen != SignIn {
			// Arriving here from somewhere else is a session ending. What was
			// typed belonged to the session that is over.
			a.signIn = signInState{}
		}
		a.screen = SignIn
		return page.Into(&a.signIn.values)
	default:
		a.screen = Home
		return page.Into(&a.home.values)
	}
}
