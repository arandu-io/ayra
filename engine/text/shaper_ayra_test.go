package text

import (
	"strings"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"github.com/arandu-io/ayra/engine/font/opentype"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// newTestShaper builds a shaper over two faces: one that covers Latin and one
// that covers Arabic.
//
// Two rather than one because the properties below are about what happens when
// a run has to change face, and a shaper with a single face answers every
// question with the same face and proves nothing about the second.
//
// System fonts are off. What a machine happens to have installed changes the
// glyph counts and the advances, so a test that read them would pass here and
// fail on the next machine.
func newTestShaper(t *testing.T) *Shaper {
	t.Helper()
	latin, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parsing the Latin face: %v", err)
	}
	arabic, err := opentype.Parse(nsareg.TTF)
	if err != nil {
		t.Fatalf("parsing the Arabic face: %v", err)
	}
	return NewShaper(NoSystemFonts(), WithCollection([]FontFace{{Face: latin}, {Face: arabic}}))
}

// testParams are the parameters the properties are checked under: wide enough
// that nothing wraps, so that a change in glyph count means a change in
// shaping rather than a change in where a line broke.
func testParams() Parameters {
	return Parameters{
		PxPerEm:  fixed.I(16),
		MaxWidth: 10000,
		Locale:   english,
	}
}

// drain reads a whole shaping result out of the shaper.
func drain(s *Shaper) []Glyph {
	var glyphs []Glyph
	for g, ok := s.NextGlyph(); ok; g, ok = s.NextGlyph() {
		glyphs = append(glyphs, g)
	}
	return glyphs
}

// TestShapingEmptyStringDrawsNothing checks that the empty string produces no
// text and does not panic.
//
// It is not "no glyphs": the shaper answers with one glyph carrying the line's
// ascent and descent and nothing else, because a caller that asked for an empty
// line still has to know how tall the line is -- an empty text field has a
// height, and a caller handed nothing at all would have to invent one.
//
// What the property fixes is that the glyph is empty of text: no runes
// accounted to it, no advance, and nothing for the line to be wide by.
func TestShapingEmptyStringDrawsNothing(t *testing.T) {
	for _, how := range []struct {
		name string
		lay  func(*Shaper, Parameters)
	}{
		{"LayoutString", func(s *Shaper, p Parameters) { s.LayoutString(p, "") }},
		{"Layout", func(s *Shaper, p Parameters) { s.Layout(p, strings.NewReader("")) }},
	} {
		t.Run(how.name, func(t *testing.T) {
			s := newTestShaper(t)
			how.lay(s, testParams())
			glyphs := drain(s)

			for i, g := range glyphs {
				if g.Runes != 0 {
					t.Errorf("glyph %d accounts for %d runes of an empty string", i, g.Runes)
				}
				if g.Advance != 0 {
					t.Errorf("glyph %d of an empty string advances by %v", i, g.Advance)
				}
			}
			if len(s.txt.lines) != 1 {
				t.Fatalf("an empty string laid out as %d lines, want 1", len(s.txt.lines))
			}
			if w := s.txt.lines[0].width; w != 0 {
				t.Errorf("an empty line is %v wide, want 0", w)
			}
		})
	}
}

// TestShapingAdvancesSumToLineWidth checks that the advances of a line's glyphs
// add up to the width the line reports.
//
// This is what lets a caller place a caret, measure a selection or centre a
// label without shaping the text a second time: the width the layout answers
// and the distance walked glyph by glyph have to be the same number, or the
// caret lands where the text is not.
//
// The inputs carry no trailing space on purpose. The advance of a line's final
// whitespace is zeroed unless the caller asks for it back, so a trailing space
// would be a glyph the line is deliberately not wide by, and the sum would
// disagree for a reason that is not a fault.
func TestShapingAdvancesSumToLineWidth(t *testing.T) {
	for _, input := range []string{
		"a",
		"abc",
		"the quick brown fox",
		"بِ",
		"mixed بِ scripts",
	} {
		t.Run(input, func(t *testing.T) {
			s := newTestShaper(t)
			s.LayoutString(testParams(), input)
			glyphs := drain(s)

			var sum fixed.Int26_6
			for _, g := range glyphs {
				sum += g.Advance
			}
			if len(s.txt.lines) != 1 {
				t.Fatalf("input wrapped onto %d lines; the property is stated for one", len(s.txt.lines))
			}
			if want := s.txt.lines[0].width; sum != want {
				t.Errorf("glyph advances sum to %v, line reports width %v", sum, want)
			}
		})
	}
}

// TestShapingIsDeterministic checks that the same input shaped twice answers
// the same glyphs.
//
// Twice on one shaper and once on a fresh one, because the two paths are
// different: the second call on the same shaper is answered from the layout
// cache, and the fresh shaper does the work again. A cache that answered
// something else would be a screen that changes when nothing changed, and the
// frames are drawn often enough that it would be seen as a flicker rather than
// as a fault anybody could locate.
func TestShapingIsDeterministic(t *testing.T) {
	const input = "the quick brown fox\njumped over the lazy dog"

	s := newTestShaper(t)
	s.LayoutString(testParams(), input)
	first := drain(s)
	s.LayoutString(testParams(), input)
	cached := drain(s)

	other := newTestShaper(t)
	other.LayoutString(testParams(), input)
	fresh := drain(other)

	if len(first) == 0 {
		t.Fatal("shaping produced no glyphs at all")
	}
	for _, again := range []struct {
		name   string
		glyphs []Glyph
	}{
		{"same shaper", cached},
		{"fresh shaper", fresh},
	} {
		if len(again.glyphs) != len(first) {
			t.Errorf("%s: %d glyphs, first pass had %d", again.name, len(again.glyphs), len(first))
			continue
		}
		for i := range first {
			if again.glyphs[i] != first[i] {
				t.Errorf("%s: glyph %d is %+v, first pass had %+v", again.name, i, again.glyphs[i], first[i])
			}
		}
	}
}

// TestShapingAccountsForEveryRune checks the correspondence the shaper
// promises: every rune of the input is accounted for by exactly one glyph
// cluster, in order.
//
// The running total is the property, not the per-glyph count. A glyph carries
// the rune count of its whole cluster and carries it on the last glyph of that
// cluster, so the counts are zero for most glyphs and larger than one for some;
// what has to hold is that they never go backwards and that they end on the
// number of runes that went in. A caller maps a screen position back to an
// offset in the string by walking that total, and a total that overshot or came
// up short is a caret that lands in the middle of a character.
func TestShapingAccountsForEveryRune(t *testing.T) {
	for _, input := range []string{
		"abc",
		"é",
		"بِّ",
		"one\ntwo\n",
		"mixed بِ scripts",
	} {
		t.Run(input, func(t *testing.T) {
			s := newTestShaper(t)
			s.LayoutString(testParams(), input)

			total := 0
			for i, g := range drain(s) {
				next := total + int(g.Runes)
				if next < total {
					t.Fatalf("glyph %d moved the rune total backwards, from %d to %d", i, total, next)
				}
				total = next
			}
			if want := len([]rune(input)); total != want {
				t.Errorf("glyphs account for %d runes, input has %d", total, want)
			}
		})
	}
}

// TestShapingClustersCombiningMarks checks where the correspondence stops being
// one glyph per rune.
//
// A base letter and the marks written on it are one cluster, and a face is free
// to draw that cluster with fewer glyphs than it has runes -- here with one
// glyph for a letter and the two marks over it. This is why a caller cannot
// count glyphs to count characters, and the previous property is the one it has
// to use instead.
//
// Arabic rather than Latin because the Latin faces in the collection draw a
// letter and its accent with one glyph each and prove nothing: the reduction
// has to be something a face actually does, not something a test asserts it
// ought to.
func TestShapingClustersCombiningMarks(t *testing.T) {
	// BEH, then SHADDA and KASRA written on it.
	const input = "بِّ"

	s := newTestShaper(t)
	s.LayoutString(testParams(), input)
	glyphs := drain(s)

	runes := len([]rune(input))
	if len(glyphs) >= runes {
		t.Fatalf("%d runes shaped to %d glyphs; the marks were not clustered", runes, len(glyphs))
	}

	total := 0
	clusters := 0
	for _, g := range glyphs {
		total += int(g.Runes)
		if g.Flags&FlagClusterBreak != 0 {
			clusters++
		}
	}
	if total != runes {
		t.Errorf("a cluster of %d runes accounted for %d", runes, total)
	}
	if clusters != 1 {
		t.Errorf("the base and its marks came out as %d clusters, want 1", clusters)
	}
}
