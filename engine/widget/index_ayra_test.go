package widget

import (
	"image"
	"strings"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/font/opentype"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/text"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

var ayraEnglish = system.Locale{Language: "EN", Direction: system.LTR}

var ayraArabic = system.Locale{Language: "AR", Direction: system.RTL}

// ayraShape lays out source at the given size and width and returns the glyphs
// the shaper produced, which is the only input the index ever has.
func ayraShape(tb testing.TB, source string, fontSize, lineWidth int, locale system.Locale) []text.Glyph {
	tb.Helper()
	latin, err := opentype.Parse(goregular.TTF)
	if err != nil {
		tb.Fatalf("parsing the latin face: %v", err)
	}
	arabicFace, err := opentype.Parse(nsareg.TTF)
	if err != nil {
		tb.Fatalf("parsing the arabic face: %v", err)
	}
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{
		{Font: font.Font{Typeface: "LTR"}, Face: latin},
		{Font: font.Font{Typeface: "RTL"}, Face: arabicFace},
	}))
	shaper.LayoutString(text.Parameters{
		PxPerEm:    fixed.I(fontSize),
		MaxWidth:   lineWidth,
		Locale:     locale,
		WrapPolicy: text.WrapWords,
	}, source)
	var glyphs []text.Glyph
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		glyphs = append(glyphs, g)
	}
	return glyphs
}

// ayraIndex builds an index over shaped source, which is what every property
// below is asked about.
func ayraIndex(tb testing.TB, source string, fontSize, lineWidth int, locale system.Locale) *glyphIndex {
	tb.Helper()
	index := new(glyphIndex)
	for _, g := range ayraShape(tb, source, fontSize, lineWidth, locale) {
		index.Glyph(g)
	}
	return index
}

// ayraViewport is larger than any sample here, so that locate returns every
// region rather than the ones a scrolled window would show.
var ayraViewport = image.Rect(0, -1<<20, 1<<20, 1<<20)

// TestIndexSearchIsMonotonic fixes that a click further to the right never
// lands earlier in the text.
//
// It is the property the whole structure exists for: the positions are sorted,
// and a search over sorted data that can go backwards is a search that has
// stopped being one. Dragging a selection is a stream of these lookups, and one
// non-monotonic answer in the middle of a drag is a selection that shrinks while
// the pointer is still moving away.
func TestIndexSearchIsMonotonic(t *testing.T) {
	for _, source := range []string{
		"the quick brown fox",
		strings.Repeat("é", 12),
	} {
		index := ayraIndex(t, source, 16, 400, ayraEnglish)
		if len(index.lines) != 1 {
			t.Fatalf("%q was expected to fit on one line, got %d", source, len(index.lines))
		}
		y := index.positions[0].y
		previous := -1
		for px := -40; px < 460; px++ {
			pos, _ := index.closestToXY(fixed.I(px), y)
			if pos.runes < previous {
				t.Fatalf("%q: x=%d answered rune %d after rune %d", source, px, pos.runes, previous)
			}
			previous = pos.runes
		}
	}
}

// TestIndexPositionRoundTripsThroughRegion fixes that a rune drawn as a
// rectangle and then clicked in the middle of that rectangle is the same rune.
//
// The two directions are what the index is for -- a pixel becomes a caret, a
// caret becomes something to paint -- and they are computed by different code.
// Nothing forces them to agree, so this asks them to.
func TestIndexPositionRoundTripsThroughRegion(t *testing.T) {
	for _, source := range []string{
		"the quick brown fox",
		"the quick brown fox jumps over the lazy dog and keeps going",
		"one\ntwo\nthree",
	} {
		index := ayraIndex(t, source, 16, 200, ayraEnglish)
		runes := len([]rune(strings.ReplaceAll(source, "\n", "\n")))
		for r := 0; r <= runes; r++ {
			pos, _ := index.closestToRune(r)
			if pos.runes != r {
				// A rune past the end of the shaped text clamps, and the
				// clamped answer is tested elsewhere.
				continue
			}
			regions := index.locate(ayraViewport, r, r, nil)
			if len(regions) == 0 {
				t.Fatalf("%q: rune %d produced no region", source, r)
			}
			caret := regions[0].Bounds
			back, _ := index.closestToXY(fixed.I(caret.Min.X), pos.y)
			if back.runes != r {
				t.Errorf("%q: rune %d drawn at x=%d came back as rune %d", source, r, caret.Min.X, back.runes)
			}
		}
	}
}

// TestIndexClampsClicksOutsideTheText fixes that a click nowhere near the text
// still answers with a position inside it.
//
// Every caller treats the answer as an offset it may slice by, so an answer
// past the end is not a wrong caret, it is a panic in whatever reads the text
// next. Above and to the left is the first position, below and to the right is
// the last, and there is no third outcome.
func TestIndexClampsClicksOutsideTheText(t *testing.T) {
	source := "the quick brown fox jumps over the lazy dog"
	index := ayraIndex(t, source, 16, 200, ayraEnglish)
	last := index.positions[len(index.positions)-1]
	first := index.positions[0]

	cases := []struct {
		name string
		x    fixed.Int26_6
		y    int
		want int
	}{
		{"far left of the first line", fixed.I(-100000), first.y, first.runes},
		{"far above the text", fixed.I(100000), -100000, first.runes},
		{"far right of the last line", fixed.I(100000), last.y, last.runes},
		{"far below the text", fixed.I(-100000), 100000, last.runes},
	}
	for _, c := range cases {
		pos, _ := index.closestToXY(c.x, c.y)
		if pos.runes != c.want {
			t.Errorf("%s: got rune %d, want %d", c.name, pos.runes, c.want)
		}
		if pos.runes < first.runes || pos.runes > last.runes {
			t.Errorf("%s: rune %d is outside the indexed text", c.name, pos.runes)
		}
	}
	if pos, _ := index.closestToRune(-100000); pos != first {
		t.Errorf("a rune before the text answered %+v, want %+v", pos, first)
	}
	if pos, _ := index.closestToRune(100000); pos != last {
		t.Errorf("a rune after the text answered %+v, want %+v", pos, last)
	}
	if pos := index.closestToLineCol(screenPos{line: -100000, col: -100000}); pos != first {
		t.Errorf("a line before the text answered %+v, want %+v", pos, first)
	}
	if pos := index.closestToLineCol(screenPos{line: 100000, col: 100000}); pos != last {
		t.Errorf("a line after the text answered %+v, want %+v", pos, last)
	}
}

// TestIndexWithoutGlyphsAnswersZero fixes that an index nobody has fed answers
// every question with the zero position instead of reading past the end of an
// empty slice.
//
// An editor is in this state on its first frame and every time its content is
// cleared, which is exactly when a pointer event is most likely to arrive.
func TestIndexWithoutGlyphsAnswersZero(t *testing.T) {
	var index glyphIndex

	if pos, end := index.closestToXY(fixed.I(40), 12); pos != (combinedPos{}) || end {
		t.Errorf("closestToXY answered %+v, %v", pos, end)
	}
	if pos, i := index.closestToRune(7); pos != (combinedPos{}) || i != 0 {
		t.Errorf("closestToRune answered %+v at %d", pos, i)
	}
	if pos := index.closestToLineCol(screenPos{line: 3, col: 9}); pos != (combinedPos{}) {
		t.Errorf("closestToLineCol answered %+v", pos)
	}
	if regions := index.locate(ayraViewport, 0, 5, nil); len(regions) != 0 {
		t.Errorf("locate answered %d regions", len(regions))
	}
	if _, eof := index.incrementPosition(combinedPos{}); !eof {
		t.Error("incrementPosition reported a position after the end")
	}
	if !index.atEndOfLine(0) || !index.atEndOfLine(-1) {
		t.Error("atEndOfLine reported a continuation in an empty index")
	}

	index.reset()
	if pos, _ := index.closestToXY(fixed.I(-5), -5); pos != (combinedPos{}) {
		t.Errorf("after reset, closestToXY answered %+v", pos)
	}
}

// TestIndexCountsRunesNotGlyphs fixes the accounting that combining marks and
// ligatures break.
//
// A shaper answers in glyphs, and the caret is counted in runes. The two are
// not the same number: a base letter and the mark over it are one glyph and two
// runes, so anything that walks the glyphs and adds one per glyph loses a rune
// per accent. It is invisible in English, which is why the test writes a line
// that is nothing but accented letters and asserts the mismatch is real before
// asserting the index survived it.
func TestIndexCountsRunesNotGlyphs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		source    string
		locale    system.Locale
		monotonic bool
	}{
		{name: "combining marks", source: strings.Repeat("é", 10), locale: ayraEnglish, monotonic: true},
		{name: "arabic ligatures", source: strings.Repeat("لا", 8), locale: ayraArabic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runes := len([]rune(tc.source))
			glyphs := ayraShape(t, tc.source, 16, 400, tc.locale)
			if len(glyphs) >= runes {
				t.Fatalf("the sample no longer shapes multiple runes together: %d glyphs for %d runes", len(glyphs), runes)
			}
			var carriesSeveralRunes bool
			for _, glyph := range glyphs {
				carriesSeveralRunes = carriesSeveralRunes || glyph.Runes > 1
			}
			if !carriesSeveralRunes {
				t.Fatal("the sample produced no glyph that accounts for multiple runes")
			}

			index := new(glyphIndex)
			for _, glyph := range glyphs {
				index.Glyph(glyph)
			}

			last := index.positions[len(index.positions)-1]
			if last.runes != runes {
				t.Errorf("the index ends at rune %d, the text has %d", last.runes, runes)
			}
			for r := 0; r <= runes; r++ {
				pos, _ := index.closestToRune(r)
				if pos.runes != r {
					t.Fatalf("rune %d was answered with rune %d: the index is short by %d", r, pos.runes, r-pos.runes)
				}
			}

			if tc.monotonic {
				previous := fixed.Int26_6(-1 << 30)
				for i, pos := range index.positions {
					if pos.x < previous {
						t.Fatalf("position %d moved backwards to x=%v", i, pos.x)
					}
					previous = pos.x
				}
			}
		})
	}
}

// TestIndexEndOfLineIsFoundByPositionNotByRune fixes that the end of a line is
// decided by looking at the next position, not at the next rune.
//
// The two are the same count only while every rune has exactly one position,
// and bidirectional text breaks that: a run boundary keeps two positions for
// one rune, so by the end of a mixed line the position at index n belongs to a
// rune several places earlier. Asking "is the rune after this one on a later
// line?" then reads an unrelated entry, and the click that should have landed
// at the end of a line lands inside it.
func TestIndexEndOfLineIsFoundByPositionNotByRune(t *testing.T) {
	source := "The quick سماء שלום لا fox تمط שלום غير the lazy dog."
	index := ayraIndex(t, source, 16, 400, ayraEnglish)

	if len(index.positions) <= index.positions[len(index.positions)-1].runes+1 {
		t.Fatalf("the sample no longer holds more positions than runes: %d positions, %d runes",
			len(index.positions), index.positions[len(index.positions)-1].runes)
	}

	for i, pos := range index.positions {
		want := i == len(index.positions)-1 || index.positions[i+1].lineCol.line > pos.lineCol.line
		if got := index.atEndOfLine(i); got != want {
			t.Errorf("position %d (rune %d, line %d): atEndOfLine reported %v, want %v",
				i, pos.runes, pos.lineCol.line, got, want)
		}
	}
}

// TestIndexLocatesRegionsInsideTheViewport fixes that a selection is drawn on
// the lines it covers and on no others.
//
// The regions are what paints a highlight, and a highlight that reaches a line
// the selection does not cover is the kind of defect that only shows up on text
// long enough to wrap.
func TestIndexLocatesRegionsInsideTheViewport(t *testing.T) {
	source := "the quick brown fox jumps over the lazy dog and keeps going for a while"
	index := ayraIndex(t, source, 16, 200, ayraEnglish)
	if len(index.lines) < 3 {
		t.Fatalf("the sample was expected to wrap, got %d lines", len(index.lines))
	}

	start, _ := index.closestToRune(0)
	end, _ := index.closestToRune(len([]rune(source)))
	regions := index.locate(ayraViewport, start.runes, end.runes, nil)
	if len(regions) < len(index.lines) {
		t.Fatalf("a selection of everything produced %d regions for %d lines", len(regions), len(index.lines))
	}
	for i, r := range regions {
		if r.Bounds.Min.X > r.Bounds.Max.X {
			t.Errorf("region %d is inside out: %v", i, r.Bounds)
		}
	}

	// The reversed range is the same selection, because a selection dragged
	// backwards is still a selection.
	reversed := index.locate(ayraViewport, end.runes, start.runes, nil)
	if len(reversed) != len(regions) {
		t.Errorf("the reversed range produced %d regions, the forward one %d", len(reversed), len(regions))
	}
}
