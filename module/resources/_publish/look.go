package main

import (
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/theme"
)

// scheme answers the palette to open with.
//
// The flag decides here rather than the system doing it, and that is a
// starting point rather than an answer: reading the operating system's setting
// is what a finished application does, and it is one call per platform. Until
// then a person can at least see both.
func scheme(dark bool) theme.Scheme {
	if dark {
		return theme.Dark
	}
	return theme.Light
}

// faces are the typefaces this application's text is shaped from.
//
// They are carried rather than looked up, and that is not a preference. A
// binary for the browser has no filesystem to find a font in and would draw
// nothing at all; on a phone the installed faces are the platform's and differ
// between two devices of the same make. Carrying them is what makes a screen
// look the same everywhere it opens.
//
// Replace this with your own faces when you have them -- `aru font:add`
// vendors one into a project, and its licence comes with it.
func faces() []text.FontFace {
	return gofont.Collection()
}
