package main

import (
	"context"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/widget"
)

// homeState is what this screen remembers between frames.
type homeState struct {
	signOut widget.Button

	// values are what the server sent for this page. Replace these fields with
	// whatever your handler renders; the names are the JSON names it uses.
	values struct {
		Name string `json:"name"`
	}
}

// layoutHome draws what a signed-in person sees.
//
// It is a starting point: one greeting and a way out. What replaces it is
// whatever this application shows, drawn from the values its own handler
// already renders -- the same handler, the same values, the same authorization
// as the page a browser gets.
func (a *App) layoutHome(c ayra.Context, st status) ayra.Dimensions {
	s := &a.home

	if s.signOut.Clicked(c) && !st.Busy {
		// No special case for what to show afterwards: signing out redirects,
		// the client follows it, and the page that comes back is the one the
		// server sends a stranger to. Deciding here would be a second answer
		// to a question the server already answers.
		a.ask(c, a.signOut)
	}

	return centred(c, readingWidth, func(c ayra.Context) ayra.Dimensions {
		return column(c, gap,
			heading(greeting(s.values.Name)),
			problem(st.Failure, ""),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ButtonProps{
					Label:    "Sign out",
					Variant:  widget.Outline,
					Disabled: st.Busy,
				}.Layout(c, &s.signOut)
			},
		)
	})
}

// signOut ends the session on both sides.
//
// The server ending it is not this side ending it. Without the second half the
// cookie is still carried, and a session kept between runs is still on disk --
// so the next start signs back in to a session the server has already thrown
// away, and what comes back is a refusal nobody can explain.
//
// A method rather than a closure inside the screen, because it is the half that
// is easy to leave out and a closure inside a layout is a decision no test can
// reach without pressing a button.
func (a *App) signOut(ctx context.Context) (client.Page, error) {
	page, err := a.server.Post(ctx, "/logout", nil)
	if err != nil {
		return page, err
	}
	return page, a.server.Forget()
}

// greeting names the person when the server said who they are.
//
// A page that rendered no name gets the general form, rather than "Welcome, "
// -- which is what concatenation produces for an empty value, and what ships
// when nobody checks.
func greeting(name string) string {
	if name == "" {
		return "Welcome"
	}
	return "Welcome, " + name
}
