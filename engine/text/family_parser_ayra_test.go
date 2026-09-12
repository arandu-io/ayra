package text

import (
	"slices"
	"strings"
	"testing"
)

// A list of one name is that name.
func TestOneNameIsTheWholeList(t *testing.T) {
	var p parser

	got, err := p.parse("serif")
	if err != nil {
		t.Fatalf("serif did not parse: %v", err)
	}
	if want := []string{"serif"}; !slices.Equal(got, want) {
		t.Errorf("the list is %q and %q was expected", got, want)
	}
}

// Names are separated by commas, and the space around a comma is not part of a
// name.
//
// Every spelling below is the same list. A reader writes the spacing that looks
// right to them and a stylesheet copied from the browser half of the product
// brings its own, so a parser that kept the spaces would answer a different
// family for each of these and find none of them installed.
func TestSpacingAroundCommasIsNotPartOfTheName(t *testing.T) {
	want := []string{"Iracema Sans", "serif"}

	for _, input := range []string{
		"Iracema Sans,serif",
		"Iracema Sans, serif",
		"Iracema Sans ,serif",
		"Iracema Sans  ,   serif",
		"  Iracema Sans , serif  ",
		"\tIracema Sans,\nserif\n",
		`"Iracema Sans"   ,   "serif"`,
		`'Iracema Sans',serif`,
	} {
		t.Run(input, func(t *testing.T) {
			var p parser
			got, err := p.parse(input)
			if err != nil {
				t.Fatalf("the list did not parse: %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("the list is %q and %q was expected", got, want)
			}
		})
	}
}

// A name is kept as written, including the letters a comma-delimited syntax has
// no opinion about.
//
// The names a device carries are not ASCII, and the parser is not the place to
// decide otherwise: it delimits, and what falls between the delimiters is
// handed on byte for byte. A parser that folded accents here would ask the font
// map for a family nobody installed.
func TestANameIsKeptAsWritten(t *testing.T) {
	want := []string{"Açaí Display", "Ñandú Serif", "日本語ゴシック", "serif"}

	for _, input := range []string{
		"Açaí Display, Ñandú Serif, 日本語ゴシック, serif",
		`"Açaí Display", 'Ñandú Serif', "日本語ゴシック", serif`,
	} {
		t.Run(input, func(t *testing.T) {
			var p parser
			got, err := p.parse(input)
			if err != nil {
				t.Fatalf("the list did not parse: %v", err)
			}
			if !slices.Equal(got, want) {
				t.Errorf("the list is %q and %q was expected", got, want)
			}
		})
	}
}

// Quotes are what lets a name contain the character that separates names, and
// they change nothing else about it.
//
// This is the reason the syntax has quoting at all: a family with a comma in it
// is unreachable without them, because bare text breaks at the first one. What
// quoting does not do is mark a name as literal -- a quoted generic is still
// the generic, because the parser answers names and the font map is what knows
// which of them are generic.
func TestQuotesLetANameContainACommaAndNothingElse(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "double quotes hold a comma",
			input: `"Bigelow, Holmes", serif`,
			want:  []string{"Bigelow, Holmes", "serif"},
		},
		{
			name:  "single quotes hold a comma",
			input: `'Bigelow, Holmes', serif`,
			want:  []string{"Bigelow, Holmes", "serif"},
		},
		{
			name:  "a name that is only commas",
			input: `",,,", serif`,
			want:  []string{",,,", "serif"},
		},
		{
			name:  "a bare comma is a separator and never a name",
			input: `Bigelow, Holmes, serif`,
			want:  []string{"Bigelow", "Holmes", "serif"},
		},
		{
			name:  "the generic families are names like any other",
			input: `fantasy, math, emoji, serif, sans-serif, cursive, monospace`,
			want: []string{
				"fantasy", "math", "emoji", "serif",
				"sans-serif", "cursive", "monospace",
			},
		},
		{
			name:  "quoting a generic does not stop it being one",
			input: `"monospace"`,
			want:  []string{"monospace"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p parser
			got, err := p.parse(c.input)
			if err != nil {
				t.Fatalf("the list did not parse: %v", err)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("the list is %q and %q was expected", got, c.want)
			}
		})
	}
}

// Inside quotes a backslash escapes the quote that opened the name, and itself.
//
// Those two and nothing else: an escape set that grew would have to be the
// same set on both halves of the product, and every addition is a name that
// means one thing here and another in a stylesheet.
func TestInsideQuotesABackslashEscapesTheQuoteAndItself(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "an escaped double quote inside double quotes",
			input: `"He said \"serif\""`,
			want:  `He said "serif"`,
		},
		{
			name:  "an escaped single quote inside single quotes",
			input: `'it\'s a serif'`,
			want:  `it's a serif`,
		},
		{
			name:  "an escaped backslash",
			input: `"back\\slash"`,
			want:  `back\slash`,
		},
		{
			name:  "a backslash before the closing quote is the backslash",
			input: `"trailing\\"`,
			want:  `trailing\`,
		},
		{
			name:  "the other quote needs no escape",
			input: `"it's a serif"`,
			want:  `it's a serif`,
		},
		{
			name:  "and neither does it the other way round",
			input: `'He said "serif"'`,
			want:  `He said "serif"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p parser
			got, err := p.parse(c.input)
			if err != nil {
				t.Fatalf("the name did not parse: %v", err)
			}
			if want := []string{c.want}; !slices.Equal(got, want) {
				t.Errorf("the name is %q and %q was expected", got, want)
			}
		})
	}
}

// A backslash outside quotes is an ordinary character.
//
// Escaping is a property of the quoted form, because outside quotes there is
// nothing to escape: the only delimiter is the comma, and a name that needs one
// has to be quoted anyway.
func TestABackslashOutsideQuotesIsAnOrdinaryCharacter(t *testing.T) {
	var p parser

	got, err := p.parse(`C:\Windows\Fonts, serif`)
	if err != nil {
		t.Fatalf("the list did not parse: %v", err)
	}
	if want := []string{`C:\Windows\Fonts`, "serif"}; !slices.Equal(got, want) {
		t.Errorf("the list is %q and %q was expected", got, want)
	}
}

// Malformed input is refused, and refusal answers no families at all.
//
// This is the contract the shaper leans on. It parses whatever a screen put in
// the typeface field, and on an error it keeps the faces it already had; a
// half-built list handed back alongside the error would be a screen drawn in
// the first family of a rule the parser could not finish reading.
func TestMalformedInputIsRefusedAndAnswersNoFamilies(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "nothing at all", input: ""},
		{name: "only spaces", input: "   \t\r\n  "},
		{name: "a bare comma", input: ","},
		{name: "nothing but commas", input: ",,,"},
		{name: "commas and spaces", input: " , ,  , "},
		{name: "an empty double-quoted name", input: `""`},
		{name: "an empty single-quoted name", input: `''`},
		{name: "a whitespace-only quoted name", input: `"   "`},
		{name: "an empty quoted name in a list", input: `serif, "", monospace`},
		{name: "a trailing comma", input: "serif,"},
		{name: "a trailing comma with space", input: "Iracema Sans, serif, "},
		{name: "a leading comma", input: ", serif"},
		{name: "a hole in the middle", input: "Iracema Sans, , serif"},
		{name: "an unterminated double quote", input: `"serif`},
		{name: "an unterminated single quote", input: `'serif`},
		{name: "an unterminated quote after a good name", input: `Iracema Sans, "serif`},
		{name: "a quote closed by the other kind", input: `"serif'`},
		{name: "an odd number of quotes", input: `"""`},
		{name: "an illegal escape", input: `"a\b"`},
		{name: "an escape at the very end", input: `"a\`},
		{name: "a quoted name with no separator after it", input: `"serif" "sans-serif"`},
		{name: "a bare name jammed onto a quoted one", input: `"serif"sans-serif`},
		{name: "a lone backslash", input: `"\"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p parser
			got, err := p.parse(c.input)
			if err == nil {
				t.Fatalf("%q was accepted and answered %q", c.input, got)
			}
			if got != nil {
				t.Errorf("the refusal answered %q and no families were expected", got)
			}
		})
	}
}

// Hostile input is answered, never survived.
//
// The typeface field is a string a screen fills in, so this is where text from
// outside the program is interpreted, and the only two outcomes allowed are a
// list and an error. The cases below are truncations and repetitions of every
// character the syntax gives a meaning to -- the shapes that a hand-written
// state machine gets wrong -- and the assertion is simply that the function
// returns.
func TestHostileInputIsAnsweredNeverSurvived(t *testing.T) {
	seeds := []string{
		"", " ", ",", `"`, `'`, `\`, `\\`, `""`, `''`, `",'`, `\,`,
		`"\`, `'\`, `,"`, `,'`, `a`, `a,`, `,a`, `"a`, `a"`, `'a`, `a'`,
		`"a\`, `"a\\`, `"a\\\`, `a\,b`, `"",""`, `''''`, `" , "`, "\x00",
		"\xff\xfe", `"\xff"`, strings.Repeat(",", 64), strings.Repeat(`"`, 64),
		strings.Repeat(`\`, 64), strings.Repeat("a,", 64), strings.Repeat(`"a",`, 64),
	}

	var p parser
	for _, seed := range seeds {
		t.Run(seed, func(t *testing.T) {
			got, err := p.parse(seed)
			if err != nil && got != nil {
				t.Errorf("%q was refused and still answered %q", seed, got)
			}
		})
	}
}

// The families answered are valid until the next parse, and the parser is
// reusable.
//
// It is kept on the shaper and parses once per shaped run, so the slice is
// reused rather than allocated every time. A caller that needs the names to
// outlive the next call copies them, and this pins the rule that says so.
func TestTheFamiliesAreValidUntilTheNextParse(t *testing.T) {
	var p parser

	first, err := p.parse("Iracema Sans, serif")
	if err != nil {
		t.Fatalf("the first list did not parse: %v", err)
	}
	kept := slices.Clone(first)

	second, err := p.parse("monospace")
	if err != nil {
		t.Fatalf("the second list did not parse: %v", err)
	}
	if want := []string{"monospace"}; !slices.Equal(second, want) {
		t.Errorf("the second list is %q and %q was expected", second, want)
	}
	if want := []string{"Iracema Sans", "serif"}; !slices.Equal(kept, want) {
		t.Errorf("the copy is %q and %q was expected", kept, want)
	}
}

// A refused parse does not poison the parser.
//
// The shaper logs the error and carries on with the next run, which parses the
// next typeface through the same value. State left behind by the failure would
// surface as a wrong family on an unrelated screen, which is the kind of fault
// that is never reproduced from the report.
func TestARefusedParseDoesNotPoisonTheParser(t *testing.T) {
	var p parser

	if _, err := p.parse(`"serif`); err == nil {
		t.Fatal("an unterminated quote was accepted")
	}

	got, err := p.parse("Iracema Sans, serif")
	if err != nil {
		t.Fatalf("the list after the refusal did not parse: %v", err)
	}
	if want := []string{"Iracema Sans", "serif"}; !slices.Equal(got, want) {
		t.Errorf("the list is %q and %q was expected", got, want)
	}
}
