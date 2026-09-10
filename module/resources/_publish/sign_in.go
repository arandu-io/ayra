package main

import (
	"context"
	"net/url"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/widget"
)

// signInState is what this screen remembers between frames: what is typed, and
// what was pressed.
type signInState struct {
	email    widget.Input
	password widget.Input
	submit   widget.Button

	// values are what the server sent for this page. The names are the JSON
	// names the handler renders with, so a page that comes back after a
	// refused attempt arrives with what it wants said.
	values struct {
		Email string `json:"email"`
		Error string `json:"error"`
	}
}

// layoutSignIn draws the first screen.
//
// It posts to this application's own handler, as a form. That is the handler a
// browser posts to, so the validation, the rate limiting and the session are
// the ones already written and already tested: nothing here checks a password,
// and nothing here decides who may in.
func (a *App) layoutSignIn(c ayra.Context, st status) ayra.Dimensions {
	s := &a.signIn

	// Every control is asked whether it was used, before anything is decided
	// about the answers. A control that is skipped is a control whose events
	// stay queued, and a field that missed a key is a field that ate one.
	pressed := s.submit.Clicked(c)
	fromEmail := s.email.Submitted(c)
	fromPassword := s.password.Submitted(c)

	if (pressed || fromEmail || fromPassword) && !st.Busy {
		email, password := s.email.Text(), s.password.Text()
		a.ask(c, func(ctx context.Context) (client.Page, error) {
			return a.server.Post(ctx, "/login", url.Values{
				"email":    {email},
				"password": {password},
			})
		})
	}

	return centred(c, formWidth, func(c ayra.Context) ayra.Dimensions {
		return column(c, gap,
			heading("Sign in"),
			field("Email", widget.InputProps{
				Placeholder: "you@example.com",
				Kind:        widget.Email,
				Disabled:    st.Busy,
				Invalid:     s.values.Error != "",
			}, &s.email),
			field("Password", widget.InputProps{
				Placeholder: "Your password",
				Kind:        widget.Password,
				Disabled:    st.Busy,
				Invalid:     s.values.Error != "",
			}, &s.password),
			problem(st.Failure, s.values.Error),
			func(c ayra.Context) ayra.Dimensions {
				return widget.ButtonProps{
					Label:    submitLabel(st.Busy),
					Size:     widget.Large,
					Disabled: st.Busy,
				}.Layout(c, &s.submit)
			},
		)
	})
}

// submitLabel says what the button is doing.
//
// The label changes rather than a spinner appearing beside it: on a slow
// connection the control someone pressed is the only feedback they have, and a
// control that looks untouched invites a second press.
func submitLabel(busy bool) string {
	if busy {
		return "Signing in..."
	}
	return "Sign in"
}
