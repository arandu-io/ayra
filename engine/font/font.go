// Package font says which face to draw with, and nothing about drawing.
//
// A [Font] is the request -- which families to try, upright or italic, how
// heavy -- and a [Face] is what somebody loaded to answer it. The request is a
// value with nothing behind a pointer in it, because it is copied down every
// control on the screen and compared as part of the key that shaped text is
// kept under.
//
// One name means two things in here, and the code cannot say which. This
// package is called font and it imports a shaping library that is also called
// font. Go does not qualify a package's own identifiers, so the rule when
// reading any file of this package is that a bare name is declared here and a
// qualified one never is: [Face] is ours, font.Face is theirs, and the one
// method of ours is called Face and returns theirs. Nothing at the call site
// marks the boundary and it compiles whichever way it is read.
package font

import (
	"strconv"

	"github.com/go-text/typesetting/font"
)

// Font is a request for a face: which families to try, in which style, at which
// weight.
//
// It describes a face without being one. Nothing is loaded by filling this in,
// and the same request can be answered by a different file on another machine
// -- which is the point, because the families a device carries are not known
// when a screen is written.
type Font struct {
	// Typeface specifies the name(s) of the font faces to try. See [Typeface]
	// for details.
	Typeface Typeface
	// Style specifies the kind of text style.
	Style Style
	// Weight is the text weight.
	Weight Weight
}

// Typeface is the list of families to try, in order, for one string.
//
// The syntax is a comma-delimited list of family names. A name containing a
// comma may be quoted with either single or double quotes; within quotes, a
// literal quotation mark is escaped with `\`, and a literal backslash with
// another `\`.
//
// Here is a Typeface:
//
//	Times New Roman, Georgia, serif
//
// This is the same one:
//
//	"Times New Roman", 'Georgia', serif
//
// And these are the escapes:
//
//	"Contains a literal \" doublequote", 'Literal \' Singlequote', "\\ Literal backslash", '\\ another'
//
// It is a list and not one name because a family is a thing a device either has
// or has not: the first that is present is used, and the last entry is a generic
// family, which is the one that cannot be missing. The generic families are
// expanded to the known families that match, and they are:
//
//   - fantasy
//   - math
//   - emoji
//   - serif
//   - sans-serif
//   - cursive
//   - monospace
//
// The syntax is the one a stylesheet writes a font-family rule in, less the
// semicolon, so a face described on the browser half of the product is described
// the same way here.
type Typeface string

// Style is whether a face is upright or slanted.
//
// The set is closed at the two that a font file itself distinguishes. A slant
// this package invented would be one no file carries, and answering it would
// mean drawing a face nobody drew.
type Style int

const (
	// Regular is upright, and zero, because most text is.
	Regular Style = iota
	// Italic is the slanted face, drawn as its own face rather than leaned.
	Italic
)

// String names the style, for a log line and for a test failure.
//
// A value outside the set names its number instead of stopping the process; see
// [Weight.String], which is read in the same places and for the same reason.
func (s Style) String() string {
	switch s {
	case Regular:
		return "Regular"
	case Italic:
		return "Italic"
	}
	return "Style(" + strconv.Itoa(int(s)) + ")"
}

// Weight is how heavy a face is, as a CSS weight less 400.
//
// The offset is what makes the zero value useful: a request left as somebody
// found it asks for the regular face, and a field nobody filled in is not a
// request for the thinnest weight in the family.
//
// The nine named weights are a hundred apart and in order, so the distance
// between two of them is a number -- which is what lets the nearest weight a
// family actually carries answer for one it does not. A value between the names
// is ordinary and not a mistake: a variable face carries them, and a description
// read from a file carries whatever the file declared.
type Weight int

const (
	// Thin is CSS weight 100, the lightest of the nine.
	Thin Weight = -300
	// ExtraLight is CSS weight 200.
	ExtraLight Weight = -200
	// Light is CSS weight 300.
	Light Weight = -100
	// Normal is CSS weight 400, and zero, because it is what running text is
	// set in.
	Normal Weight = 0
	// Medium is CSS weight 500.
	Medium Weight = 100
	// SemiBold is CSS weight 600, and is what a label set against a filled
	// background holds at.
	SemiBold Weight = 200
	// Bold is CSS weight 700.
	Bold Weight = 300
	// ExtraBold is CSS weight 800.
	ExtraBold Weight = 400
	// Black is CSS weight 900, the heaviest of the nine.
	Black Weight = 500
)

// String names the weight, for a log line and for a test failure.
//
// A value outside the named nine names its number. It used to stop the process,
// and that is the wrong answer twice over: the values in between are legitimate,
// and the name is asked for exactly where something has already gone wrong -- a
// face that did not load, a line of diagnostics about the collection. A String
// method that panics there takes the window down with the report that would have
// said which face it was.
func (w Weight) String() string {
	switch w {
	case Thin:
		return "Thin"
	case ExtraLight:
		return "ExtraLight"
	case Light:
		return "Light"
	case Normal:
		return "Normal"
	case Medium:
		return "Medium"
	case SemiBold:
		return "SemiBold"
	case Bold:
		return "Bold"
	case ExtraBold:
		return "ExtraBold"
	case Black:
		return "Black"
	}
	return "Weight(" + strconv.Itoa(int(w)) + ")"
}

// Face is a loaded face, held opaquely.
//
// What is behind it depends on who loaded it and which shaper is in use, and
// nothing outside the shaper has any use for the inside -- so the interface is
// the one method that hands it over.
type Face interface {
	// Face returns the loaded face.
	//
	// It is called when a collection is registered, and what it returns is kept
	// and used by the one shaper that asked. An implementation may therefore
	// hand back a fresh wrapper on each call rather than one shared value, which
	// is what lets a single loaded file serve two shapers at once.
	//
	// The result is the shaping library's Face and not the [Face] declared
	// above; the two are told apart only by the qualifier, as the package
	// comment says.
	Face() *font.Face
}

// FontFace is a request and the face that was loaded to answer it.
//
// They travel together because a collection is handed to a shaper as a list,
// and a face on its own does not say what it is in the terms a screen asks in:
// something has to carry "this file is the bold italic of that family" from
// whoever parsed it to whoever matches against it.
type FontFace struct {
	Font Font
	Face Face
}
