package text

import (
	"reflect"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"github.com/go-text/typesetting/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"

	"github.com/arandu-io/ayra/engine/font/opentype"
	"github.com/arandu-io/ayra/engine/io/system"
)

// The samples below are the ones every property here is measured against.
//
// They are long enough to be forced onto several lines at the widths used, and
// the bidirectional one changes script inside each direction, so a shaper that
// merged runs it should have split would be caught by the run counts rather
// than only by the pixels.
const (
	latinSample  = "The quick brown fox jumps over the lazy dog."
	arabicSample = "الحب سماء لا تمط غير الأحلام"
	bidiSample   = "The quick سماء שלום لا fox تمط שלום غير the lazy dog."
)

var (
	english = system.Locale{Language: "EN", Direction: system.LTR}
	arabic  = system.Locale{Language: "AR", Direction: system.RTL}
)

// ayraShaper builds a shaper over the faces named, in order of priority.
//
// It takes no system fonts: a test that asked the machine for a face would
// pass or fail by what happens to be installed, which is not a property of
// this package.
func ayraShaper(t *testing.T, faces ...[]byte) *shaperImpl {
	t.Helper()
	collection := make([]FontFace, 0, len(faces))
	for _, data := range faces {
		face, err := opentype.Parse(data)
		if err != nil {
			t.Fatalf("parsing test face: %v", err)
		}
		collection = append(collection, FontFace{Face: face})
	}
	return newShaperImpl(false, collection)
}

// glyphRecord is one glyph reduced to the numbers that decide where it lands.
type glyphRecord struct {
	id           GlyphID
	clusterIndex int
	glyphCount   int
	runeCount    int
	advance      fixed.Int26_6
	xOffset      fixed.Int26_6
	yOffset      fixed.Int26_6
	bounds       fixed.Rectangle26_6
}

// runRecord is one run reduced the same way.
type runRecord struct {
	visualPosition int
	x              fixed.Int26_6
	advance        fixed.Int26_6
	ppem           fixed.Int26_6
	direction      system.TextDirection
	runes          Range
	truncator      bool
	glyphs         []glyphRecord
}

// lineRecord is one line reduced the same way.
type lineRecord struct {
	width       fixed.Int26_6
	ascent      fixed.Int26_6
	descent     fixed.Int26_6
	lineHeight  fixed.Int26_6
	direction   system.TextDirection
	runeCount   int
	yOffset     int
	visualOrder []int
	runs        []runRecord
}

// record flattens a document into values that can be compared.
//
// The document itself cannot: a run points at the face it was shaped with, and
// two shapings of the same string share that pointer, so a deep comparison
// would walk into the font and compare a cache rather than a layout. What
// matters for sameness is where every glyph ends up, and that is all of this.
func record(doc document) []lineRecord {
	out := make([]lineRecord, 0, len(doc.lines))
	for _, l := range doc.lines {
		rec := lineRecord{
			width:       l.width,
			ascent:      l.ascent,
			descent:     l.descent,
			lineHeight:  l.lineHeight,
			direction:   l.direction,
			runeCount:   l.runeCount,
			yOffset:     l.yOffset,
			visualOrder: append([]int(nil), l.visualOrder...),
		}
		for _, r := range l.runs {
			run := runRecord{
				visualPosition: r.VisualPosition,
				x:              r.X,
				advance:        r.Advance,
				ppem:           r.PPEM,
				direction:      r.Direction,
				runes:          r.Runes,
				truncator:      r.truncator,
			}
			for _, g := range r.Glyphs {
				run.glyphs = append(run.glyphs, glyphRecord{
					id:           g.id,
					clusterIndex: g.clusterIndex,
					glyphCount:   g.glyphCount,
					runeCount:    g.runeCount,
					advance:      g.advance,
					xOffset:      g.xOffset,
					yOffset:      g.yOffset,
					bounds:       g.bounds,
				})
			}
			rec.runs = append(rec.runs, run)
		}
		out = append(out, rec)
	}
	return out
}

// diffRecords reports the first place two layouts disagree, or the empty
// string when they do not.
func diffRecords(t *testing.T, want, got []lineRecord) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("line count changed between shapings: %d then %d", len(want), len(got))
	}
	for i := range want {
		w, g := want[i], got[i]
		if len(w.runs) != len(g.runs) {
			t.Fatalf("line %d: run count changed between shapings: %d then %d", i, len(w.runs), len(g.runs))
		}
		if w.width != g.width || w.ascent != g.ascent || w.descent != g.descent ||
			w.lineHeight != g.lineHeight || w.direction != g.direction ||
			w.runeCount != g.runeCount || w.yOffset != g.yOffset {
			t.Fatalf("line %d: metrics changed between shapings:\n\t%+v\n\t%+v", i, w, g)
		}
		for k := range w.visualOrder {
			if w.visualOrder[k] != g.visualOrder[k] {
				t.Fatalf("line %d: visual order changed between shapings: %v then %v", i, w.visualOrder, g.visualOrder)
			}
		}
		for k := range w.runs {
			wr, gr := w.runs[k], g.runs[k]
			if len(wr.glyphs) != len(gr.glyphs) {
				t.Fatalf("line %d run %d: glyph count changed between shapings: %d then %d", i, k, len(wr.glyphs), len(gr.glyphs))
			}
			for n := range wr.glyphs {
				if wr.glyphs[n] != gr.glyphs[n] {
					t.Fatalf("line %d run %d glyph %d changed between shapings:\n\t%+v\n\t%+v", i, k, n, wr.glyphs[n], gr.glyphs[n])
				}
			}
			wr.glyphs, gr.glyphs = nil, nil
			if !reflect.DeepEqual(wr, gr) {
				t.Fatalf("line %d run %d changed between shapings:\n\t%+v\n\t%+v", i, k, wr, gr)
			}
		}
	}
}

// TestShapingTwiceLandsInTheSamePlace fixes that shaping is a function of its
// input and nothing else.
//
// It is the property the whole drawing path is built on: a layout is cached by
// the parameters that produced it and reused across frames, so a shaper that
// answered differently the second time would put the text somewhere the cached
// measurements say it is not. The shaper carries scratch buffers reused between
// calls and a font map that learns faces as it resolves them, and either is a
// way for the second answer to drift from the first.
func TestShapingTwiceLandsInTheSamePlace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		locale system.Locale
		text   string
		width  int
	}{
		{"latin", system.Locale{Language: "EN", Direction: system.LTR}, latinSample, 120},
		{"arabic", system.Locale{Language: "AR", Direction: system.RTL}, arabicSample, 120},
		{"bidirectional", system.Locale{Language: "EN", Direction: system.LTR}, bidiSample, 120},
		{"unwrapped", system.Locale{Language: "EN", Direction: system.LTR}, latinSample, 10000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := ayraShaper(t, goregular.TTF, nsareg.TTF)
			params := Parameters{
				PxPerEm:  fixed.I(14),
				MaxWidth: tc.width,
				Locale:   tc.locale,
			}
			first := record(shaper.LayoutString(params, tc.text))
			if len(first) == 0 {
				t.Fatalf("sample produced no lines at all")
			}
			second := record(shaper.LayoutString(params, tc.text))
			diffRecords(t, first, second)

			// A third shaping through a shaper that never saw the text is the
			// same property without the scratch buffers: it separates "stable
			// because nothing was reused" from "stable because reuse is
			// correct".
			fresh := ayraShaper(t, goregular.TTF, nsareg.TTF)
			diffRecords(t, first, record(fresh.LayoutString(params, tc.text)))
		})
	}
}

// TestWrappedLinesFitTheWidthGiven fixes that wrapping is bounded by the width
// it was given rather than merely guided by it.
//
// A line wider than the space it was laid out for is drawn over whatever is
// beside it, and no later stage clips it back: the measurements this package
// returns are what the drawing believes. The runes are counted at the same time
// because the cheapest way to make every line narrow is to lose some of them.
func TestWrappedLinesFitTheWidthGiven(t *testing.T) {
	for _, tc := range []struct {
		name   string
		locale system.Locale
		text   string
	}{
		{"latin", system.Locale{Language: "EN", Direction: system.LTR}, latinSample},
		{"arabic", system.Locale{Language: "AR", Direction: system.RTL}, arabicSample},
		{"bidirectional", system.Locale{Language: "EN", Direction: system.LTR}, bidiSample},
	} {
		for _, width := range []int{40, 80, 120} {
			t.Run(tc.name, func(t *testing.T) {
				shaper := ayraShaper(t, goregular.TTF, nsareg.TTF)
				runes := []rune(tc.text)
				doc := shaper.LayoutRunes(Parameters{
					PxPerEm:  fixed.I(12),
					MaxWidth: width,
					Locale:   tc.locale,
				}, runes)

				if len(doc.lines) < 2 {
					t.Fatalf("width %d: sample fit on %d line(s); the property under test needs wrapping", width, len(doc.lines))
				}
				seen := 0
				for i, l := range doc.lines {
					if l.width.Ceil() > width {
						t.Errorf("width %d: line %d is %d wide", width, i, l.width.Ceil())
					}
					seen += l.runeCount
				}
				if seen != len(runes) {
					t.Errorf("width %d: input had %d runes, the lines carry %d", width, len(runes), seen)
				}

				// alignWidth is what alignment is measured against, so a line
				// wider than it would be aligned out of its own box.
				if doc.alignWidth > width {
					t.Errorf("width %d: alignWidth is %d", width, doc.alignWidth)
				}
			})
		}
	}
}

// TestAGlyphTheFirstFaceLacksFallsToTheNext fixes that a rune absent from the
// preferred face is drawn by another face rather than dropped.
//
// Falling back is the difference between a word in an unexpected script and a
// row of empty boxes, and the failure is quiet: the layout is still valid, the
// line still has a width, and the only sign is that the text is not there. The
// test asserts the positive -- some glyph carries a face other than the first
// one -- because a shaper that dropped the runes would satisfy any count that
// only looked at what it did produce.
func TestAGlyphTheFirstFaceLacksFallsToTheNext(t *testing.T) {
	// Latin first, Arabic second: every rune of the sample is absent from the
	// face the shaper would otherwise reach for.
	shaper := ayraShaper(t, goregular.TTF, nsareg.TTF)
	runes := []rune(arabicSample)
	doc := shaper.LayoutRunes(Parameters{
		PxPerEm:  fixed.I(16),
		MaxWidth: 10000,
		Locale:   system.Locale{Language: "AR", Direction: system.RTL},
	}, runes)

	seenRunes := 0
	glyphs := 0
	fallback := 0
	for _, l := range doc.lines {
		seenRunes += l.runeCount
		for _, r := range l.runs {
			for _, g := range r.Glyphs {
				glyphs++
				if _, faceIdx, _ := splitGlyphID(g.id); faceIdx != 0 {
					fallback++
				}
			}
		}
	}
	if seenRunes != len(runes) {
		t.Errorf("input had %d runes, the lines carry %d", len(runes), seenRunes)
	}
	if glyphs == 0 {
		t.Fatalf("the sample produced no glyphs at all")
	}
	if fallback == 0 {
		t.Errorf("all %d glyphs came from the first face, which does not cover this script", glyphs)
	}

	// The same text with only the face that covers it is the control: the
	// fallback must reach the same number of glyphs the right face reaches on
	// its own, or it found the face and still lost characters.
	only := ayraShaper(t, nsareg.TTF)
	direct := only.LayoutRunes(Parameters{
		PxPerEm:  fixed.I(16),
		MaxWidth: 10000,
		Locale:   system.Locale{Language: "AR", Direction: system.RTL},
	}, runes)
	directGlyphs := 0
	for _, l := range direct.lines {
		for _, r := range l.runs {
			directGlyphs += len(r.Glyphs)
		}
	}
	if glyphs != directGlyphs {
		t.Errorf("fallback produced %d glyphs; the covering face alone produces %d", glyphs, directGlyphs)
	}
}

// TestEmptyTextIsLaidOutWithoutGlyphs fixes what an empty string answers.
//
// It is not the same question as "does it crash": the layout of nothing is
// asked for constantly -- an empty field, a label whose value has not arrived --
// and it must still carry the metrics that give the caret its height, which is
// why it is a line rather than no lines at all. What it must not carry is a
// glyph, and what it must not do is fault on a face list that cannot resolve
// anything for a text with no runes to resolve from.
func TestEmptyTextIsLaidOutWithoutGlyphs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		faces [][]byte
	}{
		{"with a face", [][]byte{goregular.TTF}},
		{"with no faces at all", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shaper := ayraShaper(t, tc.faces...)
			doc := shaper.LayoutRunes(Parameters{
				PxPerEm:  fixed.I(14),
				MaxWidth: 200,
				Locale:   system.Locale{Language: "EN", Direction: system.LTR},
			}, nil)
			for i, l := range doc.lines {
				if l.runeCount != 0 {
					t.Errorf("line %d claims %d runes for the empty string", i, l.runeCount)
				}
				for k, r := range l.runs {
					if n := len(r.Glyphs); n != 0 {
						t.Errorf("line %d run %d has %d glyphs for the empty string", i, k, n)
					}
					if r.Advance != 0 {
						t.Errorf("line %d run %d advances by %v for the empty string", i, k, r.Advance)
					}
				}
				if l.width != 0 {
					t.Errorf("line %d is %v wide for the empty string", i, l.width)
				}
			}
			if len(tc.faces) > 0 {
				if len(doc.lines) != 1 {
					t.Fatalf("expected one line carrying the metrics, got %d", len(doc.lines))
				}
				if doc.lines[0].ascent <= 0 {
					t.Errorf("the empty line has no ascent, so a caret on it has no height")
				}
			}
		})
	}
}

// TestMissingFaceOutputKeepsMissingTextVisible separates missing text from
// empty text. A rune no face can draw must occupy visible space and keep its
// source count, otherwise an unsupported script disappears without a trace.
func TestMissingFaceOutputKeepsMissingTextVisible(t *testing.T) {
	shaper := ayraShaper(t)
	runes := []rune("字")
	doc := shaper.LayoutRunes(Parameters{
		PxPerEm:  fixed.I(14),
		MaxWidth: 200,
		Locale:   system.Locale{Language: "ZH", Direction: system.LTR},
	}, runes)
	if len(doc.lines) != 1 || len(doc.lines[0].runs) != 1 {
		t.Fatalf("missing text produced %d lines", len(doc.lines))
	}
	run := doc.lines[0].runs[0]
	if len(run.Glyphs) != 1 || run.Advance == 0 {
		t.Errorf("missing text produced %d glyphs with advance %v", len(run.Glyphs), run.Advance)
	}
	if run.Runes.Count != len(runes) {
		t.Errorf("missing text accounts for %d runes, want %d", run.Runes.Count, len(runes))
	}
}

// TestBidirectionalRunsAreReorderedForDisplay fixes that a line mixing
// directions is handed back in two orders, and that they differ.
//
// Runs are stored in the order the runes were written and drawn in the order
// they are read, and for text that changes direction those are not the same
// sequence. A shaper that returned one order for both would place an Arabic
// phrase inside an English sentence at the wrong end of the line -- legible
// glyphs, wrong sentence.
func TestBidirectionalRunsAreReorderedForDisplay(t *testing.T) {
	shaper := ayraShaper(t, goregular.TTF, nsareg.TTF)
	doc := shaper.LayoutRunes(Parameters{
		PxPerEm:  fixed.I(14),
		MaxWidth: 10000,
		Locale:   system.Locale{Language: "EN", Direction: system.LTR},
	}, []rune(bidiSample))

	reordered := false
	for i, l := range doc.lines {
		if len(l.runs) != len(l.visualOrder) {
			t.Fatalf("line %d: %d runs but %d visual positions", i, len(l.runs), len(l.visualOrder))
		}
		// The two orders have to be a permutation of each other before their
		// difference means anything.
		for logical, run := range l.runs {
			if l.visualOrder[run.VisualPosition] != logical {
				t.Fatalf("line %d run %d: visual position %d points back at run %d",
					i, logical, run.VisualPosition, l.visualOrder[run.VisualPosition])
			}
			if run.VisualPosition != logical {
				reordered = true
			}
		}
		// Whatever the order, the runs are laid out left to right without
		// gaps: X follows the widths of everything visually before it.
		x := fixed.Int26_6(0)
		for _, runIdx := range l.visualOrder {
			if l.runs[runIdx].X != x {
				t.Errorf("line %d: run %d starts at %v, expected %v", i, runIdx, l.runs[runIdx].X, x)
			}
			x += l.runs[runIdx].Advance
		}
		if len(l.runs) > 1 && l.direction != system.LTR {
			t.Errorf("line %d: dominant direction is %v, expected the paragraph's", i, l.direction)
		}
	}
	if !reordered {
		t.Errorf("no run in the bidirectional sample was reordered for display")
	}
}

// TestDirectionMappingIsReversible fixes the pair of conversions between this
// project's direction and the shaper's.
//
// They are two small switches, and the failure mode of a small switch with a
// default arm is that a missing case is silently the default. A direction lost
// on the way out and defaulted on the way back reverses a paragraph.
func TestDirectionMappingIsReversible(t *testing.T) {
	for _, dir := range []system.TextDirection{system.LTR, system.RTL} {
		if got := unmapDirection(mapDirection(dir)); got != dir {
			t.Errorf("%v mapped out and back as %v", dir, got)
		}
	}
}

// TestControlCharactersBecomeSpaces fixes that the characters which would
// otherwise split a paragraph are replaced one for one.
//
// The count is the point. Every rune of the input is addressed by position --
// the caret, a selection, a hit test all index into it -- so a replacement that
// removed a character would shift every position after it, and a caret that
// lands one character off is the kind of fault that is blamed on the mouse.
func TestControlCharactersBecomeSpaces(t *testing.T) {
	in := []rune("abcd\re\nfg h")
	out := replaceControlCharacters(append([]rune(nil), in...))
	if len(out) != len(in) {
		t.Fatalf("replacement changed the rune count: %d became %d", len(in), len(out))
	}
	for i, r := range out {
		switch in[i] {
		case '', '', '', '\r', '\n', '', ' ':
			if r != ' ' {
				t.Errorf("rune %d: %q survived as %q", i, in[i], r)
			}
		default:
			if r != in[i] {
				t.Errorf("rune %d: %q was changed to %q", i, in[i], r)
			}
		}
	}
}

// TestLinesDescendDownThePage fixes that the vertical offsets a document hands
// back increase, and that appending one document to another keeps them doing
// so.
//
// Paragraphs are laid out separately and stitched together, and the offsets are
// recomputed for the whole run when they are. Without that, the second
// paragraph would start again from the top of the first.
func TestLinesDescendDownThePage(t *testing.T) {
	shaper := ayraShaper(t, goregular.TTF, nsareg.TTF)
	params := Parameters{
		PxPerEm:  fixed.I(14),
		MaxWidth: 120,
		Locale:   system.Locale{Language: "EN", Direction: system.LTR},
	}
	first := shaper.LayoutString(params, latinSample)
	second := shaper.LayoutString(params, latinSample)
	if len(first.lines) < 2 {
		t.Fatalf("sample did not wrap; the property under test needs several lines")
	}
	joined := len(first.lines) + len(second.lines)
	first.append(second)
	if len(first.lines) != joined {
		t.Fatalf("append produced %d lines from %d", len(first.lines), joined)
	}
	previous := first.lines[0].yOffset
	if previous <= 0 {
		t.Errorf("the first line sits at %d, which puts its ascent off the top", previous)
	}
	for i, l := range first.lines[1:] {
		if l.yOffset <= previous {
			t.Fatalf("line %d sits at %d, not below the previous %d", i+1, l.yOffset, previous)
		}
		previous = l.yOffset
	}
}

// TestAlignmentWidthUsesTheWidestRequirement fixes the box lines align in: it
// can be widened by the caller or by text, and neither is allowed to make the
// other smaller.
func TestAlignmentWidthUsesTheWidestRequirement(t *testing.T) {
	lines := []line{{width: fixed.I(50)}, {width: fixed.I(75)}, {width: fixed.I(25)}}
	for _, tc := range []struct {
		minimum int
		want    int
	}{{0, 75}, {50, 75}, {100, 100}} {
		if got := alignWidth(tc.minimum, lines); got != tc.want {
			t.Errorf("minimum %d produced alignment width %d, want %d", tc.minimum, got, tc.want)
		}
	}
}

// TestTrailingNewlineIsOneLogicalRune checks the glyph inserted for a newline
// that ends a paragraph. Its shape is empty, but it must occupy one logical
// position so a caret can stand after it.
func TestTrailingNewlineIsOneLogicalRune(t *testing.T) {
	shaper := ayraShaper(t, goregular.TTF, nsareg.TTF)
	input := []rune("left سماء right\n")
	doc := shaper.LayoutRunes(Parameters{
		PxPerEm:  fixed.I(14),
		MaxWidth: 10000,
		Locale:   system.Locale{Language: "EN", Direction: system.LTR},
	}, input)
	logical := 0
	synthetic := 0
	for _, line := range doc.lines {
		logical += line.runeCount
		for _, run := range line.runs {
			for _, glyph := range run.Glyphs {
				if glyph.glyphCount == 0 {
					synthetic++
					if glyph.runeCount != 1 || glyph.advance != 0 {
						t.Errorf("synthetic newline has %d runes and advance %v", glyph.runeCount, glyph.advance)
					}
				}
			}
		}
	}
	if logical != len(input) {
		t.Errorf("lines account for %d runes, input has %d", logical, len(input))
	}
	if synthetic != 1 {
		t.Errorf("got %d synthetic newline glyphs, want 1", synthetic)
	}
}

// TestGlyphIDsRoundTrip protects the packed identifier passed from layout to
// drawing. Size, face and glyph share one integer; losing bits from any field
// asks the wrong face for the wrong outline at the wrong scale.
func TestGlyphIDsRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		size  fixed.Int26_6
		face  int
		glyph font.GID
	}{
		{fixed.I(1), 0, 0},
		{fixed.I(16), 7, 42},
		{fixed.Int26_6((1 << sizebits) - 1), (1 << facebits) - 1, font.GID((1 << gidbits) - 1)},
	} {
		gotSize, gotFace, gotGlyph := splitGlyphID(newGlyphID(tc.size, tc.face, tc.glyph))
		if gotSize != tc.size || gotFace != tc.face || gotGlyph != tc.glyph {
			t.Errorf("(%v, %d, %d) round-tripped as (%v, %d, %d)", tc.size, tc.face, tc.glyph, gotSize, gotFace, gotGlyph)
		}
	}
}

// TestTruncationStandsInForWhatWasDropped fixes that a line limit leaves a mark
// and that the mark accounts for the text it replaced.
//
// The truncator is one glyph standing for an unknown number of runes, and the
// count it carries is what lets a caret move past it into text that is not
// drawn. A truncator that reported one rune would strand the rest.
func TestTruncationStandsInForWhatWasDropped(t *testing.T) {
	shaper := ayraShaper(t, goregular.TTF)
	runes := []rune(latinSample)
	doc := shaper.LayoutRunes(Parameters{
		PxPerEm:  fixed.I(14),
		MaxWidth: 80,
		MaxLines: 1,
		Locale:   system.Locale{Language: "EN", Direction: system.LTR},
	}, runes)
	if len(doc.lines) != 1 {
		t.Fatalf("expected the limit to hold the text to one line, got %d", len(doc.lines))
	}
	last := doc.lines[0].runs[len(doc.lines[0].runs)-1]
	if !last.truncator {
		t.Fatalf("the final run of a truncated line is not marked as the truncator")
	}
	if last.Runes.Count <= 1 {
		t.Errorf("the truncator stands for %d runes of the %d dropped", last.Runes.Count, len(runes))
	}
	if doc.lines[0].runeCount != len(runes) {
		t.Errorf("the visible text and its truncator account for %d runes of %d", doc.lines[0].runeCount, len(runes))
	}
}

// TestTruncationCountsDroppedRunesOnce uses a truncator with several glyphs.
// The run stands for all dropped runes, but the count belongs only to its final
// glyph; putting it on every glyph makes walking the line skip text that never
// existed.
func TestTruncationCountsDroppedRunesOnce(t *testing.T) {
	shaper := ayraShaper(t, goregular.TTF)
	doc := shaper.LayoutRunes(Parameters{
		PxPerEm:   fixed.I(14),
		MaxWidth:  80,
		MaxLines:  1,
		Truncator: "...",
		Locale:    system.Locale{Language: "EN", Direction: system.LTR},
	}, []rune(latinSample))
	last := doc.lines[0].runs[len(doc.lines[0].runs)-1]
	if len(last.Glyphs) < 2 {
		t.Fatalf("the multi-glyph truncator produced %d glyphs", len(last.Glyphs))
	}
	total := 0
	for i, glyph := range last.Glyphs {
		total += glyph.runeCount
		if i < len(last.Glyphs)-1 && glyph.runeCount != 0 {
			t.Errorf("truncator glyph %d accounts for %d runes; only the final glyph may carry the count", i, glyph.runeCount)
		}
	}
	if total != last.Runes.Count {
		t.Errorf("truncator glyphs account for %d runes, run accounts for %d", total, last.Runes.Count)
	}
}
