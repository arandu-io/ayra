// The package is imported here rather than joined, so these tests see exactly
// what a control sees.
//
// One name reads backwards in this file. Inside the package, font is the
// shaping library it imports; here it is the package under test, and every
// font.X below is one of ours.

package font_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/font"
)

// TestWeightsAreOrderedAndAHundredApart fixes the scale the nine names sit on.
//
// The number stored is the CSS weight less 400, and both halves of that carry
// weight of their own: the order is what lets a face be chosen by how far it is
// from the one asked for, and the offset is what makes the zero value ordinary
// text. A name moved by hand -- two of them swapped, a gap of fifty -- compiles,
// draws, and draws in the wrong weight.
func TestWeightsAreOrderedAndAHundredApart(t *testing.T) {
	scale := []struct {
		weight font.Weight
		css    int
	}{
		{font.Thin, 100},
		{font.ExtraLight, 200},
		{font.Light, 300},
		{font.Normal, 400},
		{font.Medium, 500},
		{font.SemiBold, 600},
		{font.Bold, 700},
		{font.ExtraBold, 800},
		{font.Black, 900},
	}

	for i, step := range scale {
		if css := int(step.weight) + 400; css != step.css {
			t.Errorf("%v is CSS weight %d, want %d", step.weight, css, step.css)
		}
		if i == 0 {
			continue
		}

		lighter := scale[i-1]
		if step.weight <= lighter.weight {
			t.Errorf("%v is not heavier than %v", step.weight, lighter.weight)
		}
		if gap := step.weight - lighter.weight; gap != 100 {
			t.Errorf("%v to %v is a gap of %d, want 100", lighter.weight, step.weight, gap)
		}
	}
}

// TestTheZeroFontIsTheOrdinaryOne fixes what a description nobody filled in
// asks for.
//
// Most of the descriptions in the product are a field or two set on a zero
// value -- the weight alone, the family alone -- and the rest is whatever zero
// means. The zero value has to be the face a reader expects and not a face at
// the end of the scale: if it stopped meaning upright, regular, no family asked
// for, every one of those sites would change face and none of them would say
// so.
func TestTheZeroFontIsTheOrdinaryOne(t *testing.T) {
	var zero font.Font

	if zero.Weight != font.Normal {
		t.Errorf("the zero weight is %v, want %v", zero.Weight, font.Normal)
	}
	if zero.Style != font.Regular {
		t.Errorf("the zero style is %v, want %v", zero.Style, font.Regular)
	}
	if zero.Typeface != "" {
		t.Errorf("the zero typeface is %q, want no family asked for", zero.Typeface)
	}
	if spelled := (font.Font{Typeface: "", Style: font.Regular, Weight: font.Normal}); zero != spelled {
		t.Errorf("the zero font is %v, want the ordinary face spelled out as %v", zero, spelled)
	}
}

// TestEqualDescriptionsAreOneCacheKey fixes that a description is a value.
//
// Shaped text is kept under a key the description is part of, so two ways of
// asking for the same face have to be one entry. A field that stopped comparing
// -- a list of families held as a slice rather than as the string it is now --
// would still describe the same font and would miss the cache on every frame,
// which is a font that reshapes sixty times a second instead of once.
func TestEqualDescriptionsAreOneCacheKey(t *testing.T) {
	asked := font.Font{Typeface: "Go Mono", Style: font.Italic, Weight: font.SemiBold}
	again := font.Font{Typeface: "Go Mono", Style: font.Italic, Weight: font.SemiBold}

	if asked != again {
		t.Fatalf("two descriptions of the same face differ: %v and %v", asked, again)
	}

	shaped := map[font.Font]int{}
	shaped[asked]++
	shaped[again]++
	if len(shaped) != 1 {
		t.Errorf("one face was shaped under %d keys, want 1", len(shaped))
	}

	heavier := asked
	heavier.Weight = font.Bold
	shaped[heavier]++
	if len(shaped) != 2 {
		t.Errorf("%v and %v share a key", asked, heavier)
	}
}

// TestEveryNamedValueNamesItself fixes the names a diagnostic is read by.
//
// The tables are keyed by the constant, so two names that were given the same
// number do not reach the assertions below: a duplicate key in a map literal of
// constants is refused where it is written. What runs here is the other half --
// that each value answers with its own name, and that no two answer alike.
func TestEveryNamedValueNamesItself(t *testing.T) {
	weights := map[font.Weight]string{
		font.Thin:       "Thin",
		font.ExtraLight: "ExtraLight",
		font.Light:      "Light",
		font.Normal:     "Normal",
		font.Medium:     "Medium",
		font.SemiBold:   "SemiBold",
		font.Bold:       "Bold",
		font.ExtraBold:  "ExtraBold",
		font.Black:      "Black",
	}
	styles := map[font.Style]string{
		font.Regular: "Regular",
		font.Italic:  "Italic",
	}

	said := map[string]bool{}
	for weight, name := range weights {
		got := weight.String()
		if got != name {
			t.Errorf("weight %d is named %q, want %q", int(weight), got, name)
		}
		if said[got] {
			t.Errorf("weight %d answers %q, which another weight already answers", int(weight), got)
		}
		said[got] = true
	}

	said = map[string]bool{}
	for style, name := range styles {
		got := style.String()
		if got != name {
			t.Errorf("style %d is named %q, want %q", int(style), got, name)
		}
		if said[got] {
			t.Errorf("style %d answers %q, which another style already answers", int(style), got)
		}
		said[got] = true
	}
}

// TestAnUnnamedValueNamesItsNumber keeps a diagnostic from becoming the crash.
//
// Both types are integers, and values outside the named set are ordinary: a
// variable face carries weights between the names -- 450 is a real one, and it
// is Weight(50) here -- and a description built from a file carries whatever the
// file declared. The name is asked for at the point where something has already
// gone wrong, in a log line or a test failure, so a String method that stops the
// process takes the window with it and the report that would have said which
// face is never printed.
func TestAnUnnamedValueNamesItsNumber(t *testing.T) {
	unnamed := []struct {
		got  string
		want string
	}{
		{font.Weight(50).String(), "Weight(50)"},
		{font.Weight(-350).String(), "Weight(-350)"},
		{font.Style(7).String(), "Style(7)"},
	}

	for _, value := range unnamed {
		if value.got != value.want {
			t.Errorf("an unnamed value is named %q, want %q", value.got, value.want)
		}
	}
}
