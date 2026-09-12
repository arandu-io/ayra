package widget

import (
	"image"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// What a text view promises is an agreement between two coordinate systems.
//
// One is the text: a count of runes, from zero to the length. The other is the
// screen: a pixel the person pointed at. Every editable field and every
// selectable label in the product moves between the two on every click, every
// arrow key and every drag, and a fault in the conversion is not a control that
// draws wrongly -- it is a caret that lands on the wrong character, which reads
// as the keyboard having typed something nobody typed.
//
// The tests below say the conversion closes, that a click cannot fall off the
// line it was aimed at, and that the lines a body of text is broken into add up
// to the body of text. They are written against shaped text rather than against
// a table of glyphs, because line breaking is where the two systems disagree
// and a table would be the disagreement written down twice.

// ayraEnglish is the locale every sample below is shaped in. Direction is what
// it is for: a right-to-left locale puts the end of a line at the low end of
// the x axis, and a test that asserted about a rune's x would then be asserting
// about the locale.
var ayraEnglish = system.Locale{Language: "EN", Direction: system.LTR}

// ayraSize is the type size of every sample. Ten is small enough that a line
// of a few words wraps inside a width a test can write, and large enough that
// two neighbouring runes are more than one pixel apart -- at which point a
// round trip through pixels would close by luck.
const ayraSize = unit.Sp(10)

// ayraWide is a width nothing in these tests reaches, used where a sample has
// to be measured unwrapped.
const ayraWide = 10000

// ayraSource is a text source over a plain string.
//
// The package's own gap buffer would serve, but it belongs to the editor and
// carries an editor's behaviour; what a text view needs from a source is four
// methods, and supplying exactly those is what keeps a failure here a failure
// of the view.
type ayraSource struct {
	data    []byte
	changed bool
}

var _ textSource = (*ayraSource)(nil)

func (s *ayraSource) Size() int64 { return int64(len(s.data)) }

func (s *ayraSource) Changed() bool {
	c := s.changed
	s.changed = false
	return c
}

func (s *ayraSource) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(s.data)) {
		return 0, io.EOF
	}
	n := copy(p, s.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (s *ayraSource) ReplaceRunes(byteOffset, runeCount int64, replacement string) {
	end := byteOffset
	for range runeCount {
		_, size := utf8.DecodeRune(s.data[end:])
		end += int64(size)
	}
	out := append([]byte{}, s.data[:byteOffset]...)
	out = append(out, replacement...)
	out = append(out, s.data[end:]...)
	s.data = out
	s.changed = true
}

// ayraShaper is a shaper over the bundled faces and nothing the machine
// happens to have installed. A system font would make the measurements below
// depend on which machine ran them.
func ayraShaper() *text.Shaper {
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
}

// ayraView shapes content into a view no wider than width.
func ayraView(content string, width int) *textView {
	v := new(textView)
	v.SetSource(&ayraSource{data: []byte(content)})
	ayraLayout(v, width)
	return v
}

// ayraLayout shapes whatever the view currently holds, at the given width.
// It is separate from ayraView so that a test can change a field and ask for
// the consequence.
func ayraLayout(v *textView, width int) {
	ayraLayoutIn(v, width, ayraWide)
}

// ayraLayoutIn shapes the view inside an explicit viewport.
func ayraLayoutIn(v *textView, width, height int) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Constraints{Max: image.Pt(width, height)},
		Locale:      ayraEnglish,
	}
	v.Layout(gtx, ayraShaper(), font.Font{}, ayraSize)
}

// ayraSamples are the shapes a body of text comes in: nothing at all, one
// word, a paragraph the width breaks for us, and text whose breaks the author
// wrote. The last two are distinct cases -- a soft break has no rune of its
// own and a hard one does -- and rune accounting has historically differed
// between them.
var ayraSamples = []struct {
	name    string
	content string
	width   int
}{
	{"nothing", "", 100},
	{"one word", "hello", 100},
	{"one line", "hello world", 200},
	{"wrapped", "hello world again and again", 40},
	{"hard breaks", "abc\ndefgh\nij", 200},
	{"blank line between", "abc\n\ndef", 200},
	{"trailing break", "abc\n", 200},
	{"multibyte", "안П你 hello 안П你", 200},
}

// TestARuneSurvivesTheTripThroughPixels is the round trip the whole file
// exists for: ask where a rune is, then ask which rune is there.
//
// It runs over every rune of every sample rather than a chosen few, because
// the positions a conversion gets wrong are the ones at an edge -- the first,
// the last, and the one either side of a line break -- and a chosen few is a
// choice made by whoever already believed the code was right.
func TestARuneSurvivesTheTripThroughPixels(t *testing.T) {
	for _, sample := range ayraSamples {
		t.Run(sample.name, func(t *testing.T) {
			v := ayraView(sample.content, sample.width)
			for r := 0; r <= v.Len(); r++ {
				at := v.closestToRune(r)
				if at.runes != r {
					t.Fatalf("rune %d reports itself as %d", r, at.runes)
				}
				back, _ := v.closestToXY(at.x, at.y)
				if back.runes != r {
					t.Errorf("rune %d sits at (%v,%d), which reads back as rune %d", r, at.x, at.y, back.runes)
				}
			}
		})
	}
}

// TestAClickPastTheEndOfALineStaysOnThatLine fixes what a click to the right
// of the last word does.
//
// The answer has to be the end of the line that was clicked. A wrapped line
// ends at the same rune the next line begins at, so the two are one position
// in the text and two positions on the screen, and the arithmetic can pick
// either. Picking the next one puts the caret a line below the pointer: the
// person clicks to the right of a line and types at the start of the one
// underneath.
func TestAClickPastTheEndOfALineStaysOnThatLine(t *testing.T) {
	for _, sample := range ayraSamples {
		t.Run(sample.name, func(t *testing.T) {
			v := ayraView(sample.content, sample.width)
			for i, line := range v.index.lines {
				want := ayraLastRuneOnLine(v, i)

				v.MoveCoord(image.Pt(ayraWide, line.yOff))

				if got, _ := v.Selection(); got != want {
					t.Errorf("click past line %d put the caret at rune %d, want %d", i, got, want)
				}
				if got, _ := v.CaretPos(); got != i {
					t.Errorf("click past line %d put the caret on line %d", i, got)
				}
			}
		})
	}
}

// ayraLastRuneOnLine is the furthest rune index the shaper placed on a line,
// which is where a caret dropped past the end of that line belongs.
func ayraLastRuneOnLine(v *textView, line int) int {
	last := -1
	for _, pos := range v.index.positions {
		if pos.lineCol.line == line && pos.runes > last {
			last = pos.runes
		}
	}
	return last
}

// TestTheLineHeightsAddUpToTheHeightOfTheText says the height a view reports
// is the lines it drew and nothing else.
//
// A body of text is as tall as the ascent of its first line, plus the step
// from each baseline to the next, plus the descent of its last: that is the
// sum of the line heights, written in the quantities the index actually keeps.
// A height larger than that is space no line occupies, which a scrollbar
// offers to scroll to and a caret can never reach; a height smaller is a last
// line with its descenders cut off.
func TestTheLineHeightsAddUpToTheHeightOfTheText(t *testing.T) {
	for _, sample := range ayraSamples {
		t.Run(sample.name, func(t *testing.T) {
			v := ayraView(sample.content, sample.width)
			lines := v.index.lines
			if len(lines) == 0 {
				t.Fatal("no lines to measure")
			}

			height := lines[0].ascent.Ceil() + lines[len(lines)-1].descent.Ceil()
			for i := 1; i < len(lines); i++ {
				height += lines[i].yOff - lines[i-1].yOff
			}

			if got := v.FullDimensions().Size.Y; got != height {
				t.Errorf("the view is %d tall and its %d line(s) account for %d", got, len(lines), height)
			}
		})
	}
}

// TestTextWithNothingInItStillHasOneLine covers the empty field.
//
// An empty string has no glyphs, and a line count taken from the glyphs is
// zero -- at which point there is no line to put a caret on, no ascent to give
// the field a height, and an empty editor collapses to nothing on the screen
// with nowhere to click to start typing. The one line is the caret's line.
func TestTextWithNothingInItStillHasOneLine(t *testing.T) {
	v := ayraView("", 100)

	if got := len(v.index.lines); got != 1 {
		t.Fatalf("empty text has %d lines, want 1", got)
	}
	if got := v.Len(); got != 0 {
		t.Errorf("empty text is %d runes long", got)
	}
	if got := v.FullDimensions().Size.Y; got <= 0 {
		t.Errorf("empty text is %d tall, so there is nothing to click on", got)
	}

	line, col := v.CaretPos()
	if line != 0 || col != 0 {
		t.Errorf("the caret in empty text is at line %d column %d", line, col)
	}
}

// TestWrappingAtAWordsExactWidthLeavesNoEmptyLine is the boundary case of line
// breaking: a width that a word fills exactly.
//
// It is the case an off-by-one lands on. If the break is taken when the word
// reaches the width rather than when it passes it, the word is moved to a line
// of its own and the line it came from is left with nothing on it -- a blank
// line in the middle of a paragraph, appearing only at one particular width,
// which is why it survives being looked at.
func TestWrappingAtAWordsExactWidthLeavesNoEmptyLine(t *testing.T) {
	const word = "hello"
	exact := ayraView(word, ayraWide).FullDimensions().Size.X
	if exact <= 0 {
		t.Fatalf("the sample word measured %d wide", exact)
	}

	v := ayraView(word+" "+word, exact)

	if got := len(v.index.lines); got != 2 {
		t.Fatalf("two words at the width of one broke into %d lines, want 2", got)
	}
	for i, line := range v.index.lines {
		if line.glyphs == 0 {
			t.Errorf("line %d carries no glyphs", i)
		}
		if line.width <= 0 {
			t.Errorf("line %d is %v wide", i, line.width)
		}
	}
	if got := string(v.Text(nil)); got != word+" "+word {
		t.Errorf("the text came back as %q", got)
	}
}

// TestByteOffsetsFollowRunesThroughMultibyteText walks the other conversion the
// view owns: rune index to byte offset in the source.
//
// The sample is long enough to pass the point where the view starts keeping
// index entries instead of counting from the beginning, and every rune in it
// is more than one byte. A rune count used where a byte offset belongs is
// invisible in text that is entirely ASCII, which is what most of a test suite
// is written in.
func TestByteOffsetsFollowRunesThroughMultibyteText(t *testing.T) {
	content := strings.Repeat("안П你", 60)
	v := ayraView(content, 200)

	if got, want := v.Len(), utf8.RuneCountInString(content); got != want {
		t.Fatalf("the view is %d runes long, want %d", got, want)
	}

	want := int64(0)
	for r, char := range []rune(content) {
		if got := v.ByteOffset(r); got != want {
			t.Fatalf("rune %d starts at byte %d, want %d", r, got, want)
		}
		want += int64(utf8.RuneLen(char))
	}
	if got := v.ByteOffset(v.Len()); got != want {
		t.Errorf("the end of the text is at byte %d, want %d", got, want)
	}
	if got := string(v.Text(nil)); got != content {
		t.Error("the text did not come back as it went in")
	}
}

// TestAMaskHidesTheTextWithoutReplacingIt is the password field.
//
// What the mask changes is what the shaper is given; what the view holds is
// still what the person typed. The width is the evidence that the substitution
// happened -- four narrow letters and four copies of a wide rune do not measure
// the same -- and the contents are the evidence that it did not reach the
// source, which is what would send the mask to the clipboard.
func TestAMaskHidesTheTextWithoutReplacingIt(t *testing.T) {
	const secret = "ii\nii"
	plain := ayraView(secret, ayraWide)
	bare := plain.FullDimensions().Size.X

	v := ayraView(secret, ayraWide)
	v.Mask = 'W'
	ayraLayout(v, ayraWide)

	if got := v.FullDimensions().Size.X; got == bare {
		t.Errorf("masked and unmasked text both measure %d wide, so the mask never reached the shaper", got)
	}
	if got := string(v.Text(nil)); got != secret {
		t.Errorf("the contents read back as %q, want %q", got, secret)
	}
	if got, want := v.Len(), len(secret); got != want {
		t.Errorf("the masked view is %d runes long, want %d", got, want)
	}
	if got := len(v.index.lines); got != 2 {
		t.Errorf("masking erased the newline: got %d lines, want 2", got)
	}
}

// TestScrollingIsClampedToTheTextThatExists holds the viewport over the text.
//
// Scrolling is relative and arrives from a wheel or a drag that does not know
// how much text there is, so the clamp is the only thing standing between a
// long gesture and a viewport parked on empty space with the text off-screen
// and no way back that does not overshoot the other way.
func TestScrollingIsClampedToTheTextThatExists(t *testing.T) {
	t.Run("wrapped text moves vertically", func(t *testing.T) {
		v := ayraView("hello world again and again and again", 40)
		fullHeight := v.FullDimensions().Size.Y
		if fullHeight < 2 {
			t.Fatalf("the wrapped sample is only %d pixel tall", fullHeight)
		}
		ayraLayoutIn(v, 40, fullHeight/2)
		bounds := v.ScrollBounds()
		if bounds.Max.Y <= 0 {
			t.Fatalf("the sample fits in the viewport, so there is nothing to scroll: %v", bounds)
		}

		v.ScrollRel(0, ayraWide)
		if got := v.ScrollOff().Y; got != bounds.Max.Y {
			t.Errorf("scrolling down a long way stopped at %d, want %d", got, bounds.Max.Y)
		}

		v.ScrollRel(0, -2*ayraWide)
		if got := v.ScrollOff().Y; got != bounds.Min.Y {
			t.Errorf("scrolling back up stopped at %d, want %d", got, bounds.Min.Y)
		}
	})

	t.Run("single line text moves horizontally", func(t *testing.T) {
		v := new(textView)
		v.SingleLine = true
		v.SetSource(&ayraSource{data: []byte("one line that is much wider than its view")})
		ayraLayoutIn(v, 30, 30)
		bounds := v.ScrollBounds()
		if bounds.Max.X <= bounds.Min.X {
			t.Fatalf("the sample has no horizontal scroll range: %v", bounds)
		}

		v.ScrollRel(ayraWide, ayraWide)
		if got := v.ScrollOff(); got != image.Pt(bounds.Max.X, 0) {
			t.Errorf("scrolling past the far edge stopped at %v, want (%d,0)", got, bounds.Max.X)
		}
		v.ScrollRel(-2*ayraWide, -ayraWide)
		if got := v.ScrollOff(); got != image.Pt(bounds.Min.X, 0) {
			t.Errorf("scrolling back past the origin stopped at %v, want (%d,0)", got, bounds.Min.X)
		}
	})
}

// TestReplaceCarriesTheCaretWithTheTextAroundIt says an edit somewhere else in
// the document does not move the caret through the text.
//
// A caret after the edit has to shift by the difference in length. A caret in
// the removed suffix has to land at the replacement's end; when a longer
// replacement still covers its old logical position, that position remains
// meaningful and is retained. This is what keeps an edit elsewhere from
// throwing the caret into another word or beyond the document.
func TestReplaceCarriesTheCaretWithTheTextAroundIt(t *testing.T) {
	v := ayraView("hello world", 200)
	v.SetCaret(11, 11)

	if got := v.Replace(0, 5, "bye"); got != 3 {
		t.Fatalf("Replace reported %d runes inserted, want 3", got)
	}
	ayraLayout(v, 200)

	if got := string(v.Text(nil)); got != "bye world" {
		t.Fatalf("the text is now %q", got)
	}
	if got, _ := v.Selection(); got != 9 {
		t.Errorf("the caret is at rune %d, want 9", got)
	}

	v.SetCaret(4, 4)
	v.Replace(2, 6, "")
	ayraLayout(v, 200)

	if got := string(v.Text(nil)); got != "byrld" {
		t.Fatalf("the text is now %q", got)
	}
	if got, _ := v.Selection(); got != 2 {
		t.Errorf("a caret inside the replaced range is at rune %d, want 2", got)
	}

	t.Run("a multibyte expansion retains a still-valid inner position", func(t *testing.T) {
		v := ayraView("abcdef", 200)
		v.SetCaret(3, 6)

		if got := v.Replace(5, 1, "你éXYZ"); got != 5 {
			t.Fatalf("Replace reported %d inserted runes, want 5", got)
		}
		ayraLayout(v, 200)

		if got := string(v.Text(nil)); got != "a你éXYZf" {
			t.Fatalf("the text is now %q", got)
		}
		start, end := v.Selection()
		if start != 3 || end != 7 {
			t.Errorf("the selection moved to (%d,%d), want (3,7)", start, end)
		}
		if got := v.ByteOffset(start); got != int64(len("a你é")) {
			t.Errorf("the caret is at byte %d, want %d", got, len("a你é"))
		}
	})
}
