// Package text turns a string into glyphs that can be drawn.
//
// What comes back is a sequence of glyphs, each already positioned: a caller
// draws the text by drawing every glyph at the point the shaper gave it, and
// never has to know how the writing system it is drawing works. Between the
// string and that sequence sit the decisions a font makes -- which mark goes
// over which letter, which pair of letters is drawn as one shape, which
// direction a run of text travels in -- and this package is where they are
// made.
//
// The sequence corresponds to the runes of the input, in the order they were
// written, and that correspondence is what everything above this package is
// built on: a caret, a selection and a hit test all work by counting runes
// along the glyphs. It is a correspondence and not a pairing. A glyph is not a
// rune:
//
//   - A letter and the marks written on it are one cluster, and a face may
//     draw that cluster with fewer glyphs than it has runes.
//   - Two letters the face has one shape for are one cluster, and one glyph
//     stands for both.
//   - One rune may need several glyphs to be drawn at all.
//
// So the count is carried rather than assumed. Every glyph says how many runes
// its cluster covers, on the last glyph of that cluster, and walking those
// counts is the only way back to an offset in the string. Counting glyphs
// instead works for as long as the text is unaccented Latin, which is exactly
// long enough to ship.
package text

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	typeface "github.com/go-text/typesetting/font"
	"golang.org/x/image/math/fixed"
)

// WrapPolicy is how a line that is too long is broken.
//
// The three differ only in what they are willing to sacrifice when the text
// does not fit: the width it was given, or the reader's ability to recognise a
// word at a glance.
type WrapPolicy uint8

const (
	// WrapHeuristically breaks between words, and inside one only when that
	// word cannot fit on a line of its own. It is the default because it is
	// the only one of the three that never surprises: text stays inside the
	// width it was given, and a word is only cut when leaving it whole would
	// put it outside the screen entirely.
	//
	// When the last word of a line is being cut short, it keeps as much of
	// that word as will fit before the mark that stands for the rest.
	WrapHeuristically WrapPolicy = iota
	// WrapWords never breaks inside a word, and so lets a long one run past
	// the width it was wrapped with. It is for text whose words must stay
	// readable even when that costs the layout -- a code identifier, a URL, a
	// serial number.
	WrapWords
	// WrapGraphemes fills every line as far as it will go, breaking words
	// wherever the line runs out. It puts the most text on screen and is the
	// hardest to read, and it is here for callers measuring capacity rather
	// than presenting prose.
	WrapGraphemes
)

// Parameters is everything about a shaping run that does not vary within it.
//
// One value describes one block of text: the face, the size, the room, the
// language. What varies glyph by glyph is the shaper's answer, not its input.
type Parameters struct {
	// Font is the typeface asked for. What is actually used may differ: a face
	// that has no glyph for a rune cannot draw it, and the shaper falls back
	// to one that can rather than drawing nothing.
	Font font.Font
	// Alignment is where the text sits within the room it was given.
	//
	// It changes no glyph's shape and is not part of what gets cached. It is
	// here so that the offset every glyph needs can be computed once, while
	// the line's width is already known, instead of by measuring the text
	// again afterwards.
	Alignment Alignment
	// PxPerEm is the size to shape at, in pixels per em.
	PxPerEm fixed.Int26_6
	// MaxLines caps how many lines are produced. Zero means no cap.
	MaxLines int
	// Truncator is what is drawn in place of the text that a MaxLines cap cut
	// off. It appears only on the final line, and only when there was
	// something left to cut.
	Truncator string

	// WrapPolicy is how a line too long for MaxWidth is broken.
	WrapPolicy WrapPolicy

	// MinWidth and MaxWidth are the horizontal room the text is shaped into.
	MinWidth, MaxWidth int
	// Locale is the language and the primary direction of the text. Direction
	// is a property of the text rather than of the layout: a line of Arabic
	// inside an English paragraph still runs the other way.
	Locale system.Locale

	// LineHeightScale multiplies LineHeight. Zero means 1.2, which is the
	// spacing that keeps consecutive lines from touching at ordinary sizes.
	LineHeightScale float32

	// LineHeight is the distance from one baseline to the next. Zero takes it
	// from the paragraph's own PxPerEm, which is what makes a block of text at
	// one size spaced correctly without the caller computing anything.
	//
	// It is scaled by LineHeightScale, so a caller that wants exactly this
	// distance and no more sets that field to 1.
	LineHeight fixed.Int26_6

	// forceTruncate says the truncator must be inserted on the final line even
	// though this paragraph fits, because more paragraphs follow that will not
	// be shaped.
	//
	// It is unexported because only the shaper knows that: a caller hands over
	// one text, and it is the walk across paragraphs that discovers there is
	// more of it than MaxLines allows.
	forceTruncate bool

	// DisableSpaceTrim keeps the advance of a line's final whitespace instead
	// of zeroing it.
	//
	// Zeroed is right for text that is only read: a trailing space is invisible
	// and a line that is wide by it is a line that looks wrongly centred.
	// Kept is right for text that is edited, where that space is a character
	// the caret has to be able to stand after and the pointer has to be able
	// to select.
	DisableSpaceTrim bool
}

// FontFace is a parsed face together with the description it answers to.
type FontFace = font.FontFace

// Glyph is one shaped symbol, positioned.
//
// Most of its distances are relative to the dot: the point on the baseline
// where this glyph sits. The baseline is the line the text rests on, not the
// top or the bottom of it, because that is the line a face's own measurements
// are given from and the only one that stays put when glyphs of different
// heights sit side by side.
//
// X and Y are in document coordinates -- pixels from the text's origin at the
// upper-left. Drawing each glyph at the document coordinates of its dot draws
// the text.
//
// Glyphs come in clusters, and a cluster is the unit that corresponds to runes:
// see [Runes]. Consecutive clusters drawn with the same face, size and
// direction form a run.
type Glyph struct {
	// ID identifies the shape, not the occurrence. Two glyphs from the same
	// shaper share an ID when they are the same symbol of the same face at the
	// same size, which is what lets the drawn outline be built once and reused
	// for every occurrence on screen.
	ID GlyphID

	// X is the dot's horizontal position in document coordinates.
	X fixed.Int26_6
	// Y is the dot's vertical position in document coordinates.
	Y int32

	// Advance is how far the dot moves after this glyph. It is what the text
	// is wide by, and not what the glyph covers: a mark advances by nothing
	// and is still drawn, and an italic tail may reach past it.
	Advance fixed.Int26_6
	// Ascent is the distance from the dot to the logical top of the face this
	// glyph came from. It belongs to the face, so every glyph on a line
	// reports the same one and a line of lowercase letters is as tall as a
	// line with capitals.
	Ascent fixed.Int26_6
	// Descent is the same downwards.
	Descent fixed.Int26_6
	// Offset is where the glyph's own drawing space begins, relative to the
	// dot. It is what turns a glyph into a path.
	Offset fixed.Point26_6
	// Bounds is what the glyph actually covers, relative to the dot.
	Bounds fixed.Rectangle26_6
	// Runes is how many runes this glyph's cluster stands for.
	//
	// It is carried by the last glyph of the cluster and is zero on every
	// other, so that summing it over a sequence of glyphs counts each cluster
	// once. A glyph without FlagClusterBreak always reports zero.
	Runes uint16
	// Flags says what this glyph ends, begins, or stands in for.
	Flags Flags
}

// Flags are the boundaries and the special cases a glyph carries.
//
// They are on the glyph rather than in a structure alongside it because the
// glyphs arrive one at a time: a caller that had to look ahead to learn that
// the glyph it is holding ended a line would have to buffer the whole text to
// draw any of it.
type Flags uint16

const (
	// FlagTowardOrigin is set on glyphs of a run that travels towards the
	// origin -- right to left.
	FlagTowardOrigin Flags = 1 << iota
	// FlagLineBreak is set on the last glyph of a line.
	FlagLineBreak
	// FlagRunBreak is set on the last glyph of a run: the last one before the
	// face, the size or the direction changes.
	FlagRunBreak
	// FlagClusterBreak is set on the last glyph of a cluster, and is what
	// makes the Runes count on that glyph meaningful.
	FlagClusterBreak
	// FlagParagraphBreak marks a cluster that stands for the whitespace that
	// ended a paragraph rather than for any symbol of a font. The glyph after
	// it always carries FlagParagraphStart, giving the position the next line
	// begins at even when that line is empty -- which is where a caret goes
	// when a person presses return.
	FlagParagraphBreak
	// FlagParagraphStart is set on the glyph that begins a paragraph.
	FlagParagraphStart
	// FlagTruncator marks the glyphs standing in for text that was cut off by
	// MaxLines. The one carrying both this and FlagClusterBreak counts every
	// rune that was cut, including the ones never shaped.
	FlagTruncator
)

// String renders the flags as seven fixed columns, one per flag, so that
// glyphs printed one per line can be read down the column.
//
// Fixed rather than a list of the ones that are set: a list is shorter and
// unreadable in bulk, because the reader has to compare names instead of
// looking at one position.
func (f Flags) String() string {
	var b strings.Builder
	for _, column := range []struct {
		flag Flags
		mark string
	}{
		{FlagParagraphStart, "S"},
		{FlagParagraphBreak, "P"},
		{FlagTowardOrigin, "T"},
		{FlagLineBreak, "L"},
		{FlagRunBreak, "R"},
		{FlagClusterBreak, "C"},
		{FlagTruncator, "…"},
	} {
		if f&column.flag != 0 {
			b.WriteString(column.mark)
		} else {
			b.WriteString("_")
		}
	}
	return b.String()
}

// GlyphID names a shape: one face, at one size, showing one of its symbols.
//
// It is a single number rather than those three fields because it is a cache
// key used once per glyph per frame, and a key that is one comparison is the
// difference between a cache and a cost.
type GlyphID uint64

// The three fields packed into a GlyphID, widest last: the symbol within a
// face needs the most room, and putting it in the low bits means the mask that
// recovers it is a constant.
const (
	facebits = 16
	sizebits = 16
	gidbits  = 64 - facebits - sizebits
)

// newGlyphID packs a face, a size and a symbol into one key.
//
// It panics on a value that does not fit, rather than truncating. A truncated
// key is a key that collides with another shape's, and the symptom of that is
// one letter drawn as a different letter somewhere else on the screen -- a
// fault that would be looked for anywhere but here.
func newGlyphID(ppem fixed.Int26_6, faceIdx int, gid typeface.GID) GlyphID {
	if gid&^((1<<gidbits)-1) != 0 {
		panic("glyph id out of bounds")
	}
	if faceIdx&^((1<<facebits)-1) != 0 {
		panic("face index out of bounds")
	}
	if ppem&^((1<<sizebits)-1) != 0 {
		panic("ppem out of bounds")
	}
	// Keep only the low bits of the size, which still covers every size a
	// screen can show.
	ppem &= ((1 << sizebits) - 1)
	return GlyphID(faceIdx)<<(gidbits+sizebits) | GlyphID(ppem)<<(gidbits) | GlyphID(gid)
}

// splitGlyphID is the opposite of newGlyphID.
func splitGlyphID(g GlyphID) (fixed.Int26_6, int, typeface.GID) {
	faceIdx := int(uint64(g) >> (gidbits + sizebits))
	ppem := fixed.Int26_6((g & ((1<<sizebits - 1) << gidbits)) >> gidbits)
	gid := typeface.GID(g) & (1<<gidbits - 1)
	return ppem, faceIdx, gid
}

// Shaper turns strings into glyphs, and holds the caches that make doing it
// every frame affordable.
//
// One shaper belongs to one goroutine. Its caches are plain maps, and two
// goroutines laying text out through the same shaper will write to one of them
// at the same time and bring the process down. In practice that means one
// shaper per top-level window, which is also the boundary the caches want:
// two windows rarely draw the same text at the same size.
//
// Laying text out and reading the result are two calls, in that order. Layout
// or [Shaper.LayoutString] does the work and keeps it; [Shaper.NextGlyph]
// walks it, one glyph at a time, until it says there are no more. Starting a
// new layout discards the walk that was in progress.
type Shaper struct {
	config struct {
		disableSystemFonts bool
		collection         []FontFace
	}
	initialized      bool
	shaper           shaperImpl
	pathCache        pathCache
	bitmapShapeCache bitmapShapeCache
	layoutCache      layoutCache

	reader    *bufio.Reader
	paragraph []byte

	// Where the walk over the last layout has got to.
	brokeParagraph bool
	paragraphStart Glyph
	txt            document
	line           int
	run            int
	glyph          int
	// advance is how far into the current run the dot has already moved.
	advance fixed.Int26_6
	// done says the walk is over.
	done bool
	err  error
}

// ShaperOption configures a shaper at construction.
type ShaperOption func(*Shaper)

// NoSystemFonts stops the shaper from looking at the fonts the machine has
// installed.
//
// It is what makes output reproducible: a text laid out against whatever a
// particular machine happens to have installed is a text that is a different
// width on the next machine. Anything measuring or comparing rendered text
// wants this, and so does anything shipping a fixed set of faces.
func NoSystemFonts() ShaperOption {
	return func(s *Shaper) {
		s.config.disableSystemFonts = true
	}
}

// WithCollection gives the shaper faces that are already parsed.
//
// They are preferred over anything the system offers, and they are the only
// faces available when NoSystemFonts is also given.
func WithCollection(collection []FontFace) ShaperOption {
	return func(s *Shaper) {
		s.config.collection = collection
	}
}

// NewShaper builds a shaper.
func NewShaper(options ...ShaperOption) *Shaper {
	s := &Shaper{}
	for _, opt := range options {
		opt(s)
	}
	s.init()
	return s
}

// init builds what the shaper needs on first use.
//
// Every entry point calls it, because a zero Shaper is a usable one: it is
// reached as a field of a larger value often enough that requiring the
// constructor would be a nil face and a blank screen rather than an error
// anybody could act on.
func (s *Shaper) init() {
	if s.initialized {
		return
	}
	s.initialized = true
	s.reader = bufio.NewReader(nil)
	s.shaper = *newShaperImpl(!s.config.disableSystemFonts, s.config.collection)
}

// Layout shapes the text from a reader. The result is read back with
// [Shaper.NextGlyph].
func (s *Shaper) Layout(params Parameters, txt io.Reader) {
	s.init()
	s.layoutText(params, txt, "")
}

// LayoutString is Layout for a string already in memory, and is the cheaper
// of the two: a string can be looked up in the layout cache as it stands,
// while bytes from a reader have to be assembled into one first.
func (s *Shaper) LayoutString(params Parameters, str string) {
	s.init()
	s.layoutText(params, nil, str)
}

// reset abandons the walk over the previous layout.
func (s *Shaper) reset(align Alignment) {
	s.line, s.run, s.glyph, s.advance = 0, 0, 0, 0
	s.done = false
	s.txt.reset()
	s.txt.alignment = align
}

// layoutText shapes a text one paragraph at a time.
//
// Paragraph by paragraph rather than all at once because that is the unit the
// cache can hold: editing one line of a long text leaves every other paragraph
// byte for byte what it was, and re-shaping only the one that changed is the
// difference between typing that keeps up and typing that does not.
//
// Only one of txt and str carries the text.
func (s *Shaper) layoutText(params Parameters, txt io.Reader, str string) {
	s.reset(params.Alignment)
	if txt == nil && len(str) == 0 {
		// An empty text still gets one shaped paragraph, so that the line it
		// produces can report how tall a line would have been. A caller handed
		// nothing at all would have to invent that height.
		s.txt.append(s.layoutParagraph(params, "", nil))
		return
	}
	s.reader.Reset(txt)
	truncating := params.MaxLines > 0
	var done bool
	var endByte int
	for !done {
		s.paragraph = s.paragraph[:0]
		if txt != nil {
			done = s.readParagraph()
		} else {
			idx := strings.IndexByte(str, '\n')
			if idx == -1 {
				done = true
				endByte = len(str)
			} else {
				endByte = idx + 1
				done = endByte == len(str)
			}
		}
		if len(str[:endByte]) > 0 || (len(s.paragraph) > 0 || len(s.txt.lines) == 0) {
			// Truncation has to be decided before this paragraph is shaped,
			// because the mark standing for the missing text belongs on its
			// final line and the shaper is what puts it there.
			params.forceTruncate = truncating && !done
			lines := s.layoutParagraph(params, str[:endByte], s.paragraph)
			if truncating {
				params.MaxLines -= len(lines.lines)
				if params.MaxLines == 0 {
					done = true
					s.txt.unreadRuneCount = s.countUnread(txt, str[endByte:])
				}
			}
			s.txt.append(lines)
		}
		if done {
			return
		}
		str = str[endByte:]
	}
}

// readParagraph fills the paragraph buffer with bytes up to and including the
// next newline, and reports whether the text ended.
//
// After a newline it looks one byte further ahead and puts it back. That
// lookahead is what tells a text ending in a newline from a text with another
// paragraph after it: without it the shaper would begin an empty paragraph it
// cannot know is empty, and a trailing newline would grow a blank line at the
// bottom of every document.
func (s *Shaper) readParagraph() bool {
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			// The end of the text, or a reader that failed; either way there
			// is no more of it to shape.
			return true
		}
		s.paragraph = append(s.paragraph, b)
		if b == '\n' {
			break
		}
	}
	if _, err := s.reader.ReadByte(); err != nil {
		return true
	}
	_ = s.reader.UnreadByte()
	return false
}

// countUnread counts the runes that will never be shaped because the line cap
// was reached.
//
// They are counted rather than ignored because the truncator stands for them:
// a caller maps a position back to an offset in the string by summing rune
// counts, and a sum that stopped at the last drawn glyph would put the end of
// the text in the middle of it.
func (s *Shaper) countUnread(txt io.Reader, rest string) int {
	if txt == nil {
		return utf8.RuneCountInString(rest)
	}
	count := 0
	for {
		if _, _, err := s.reader.ReadRune(); err != nil {
			return count
		}
		count++
	}
}

// layoutParagraph shapes and wraps one paragraph, from the cache when it can.
//
// It takes the paragraph both ways and prefers the string, because the string
// is the cache key: reaching the cached answer without first turning bytes
// into one is the point of having it.
func (s *Shaper) layoutParagraph(params Parameters, asStr string, asBytes []byte) document {
	if s == nil {
		return document{}
	}
	if len(asStr) == 0 && len(asBytes) > 0 {
		asStr = string(asBytes)
	}
	// Alignment is deliberately absent from the key: it moves the finished
	// line and changes no glyph, so keying on it would shape the same text
	// again for every alignment it is ever drawn with.
	lk := layoutKey{
		ppem:            params.PxPerEm,
		maxWidth:        params.MaxWidth,
		minWidth:        params.MinWidth,
		maxLines:        params.MaxLines,
		truncator:       params.Truncator,
		locale:          params.Locale,
		font:            params.Font,
		forceTruncate:   params.forceTruncate,
		wrapPolicy:      params.WrapPolicy,
		str:             asStr,
		lineHeight:      params.LineHeight,
		lineHeightScale: params.LineHeightScale,
	}
	if cached, ok := s.layoutCache.Get(lk); ok {
		return cached
	}
	lines := s.shaper.LayoutRunes(params, []rune(asStr))
	s.layoutCache.Put(lk, lines)
	return lines
}

// NextGlyph returns the next glyph of the last layout, in rune order, and
// reports false once there are none left.
//
// Rune order, not drawing order: a run that travels right to left is drawn
// from its far end, and is walked backwards here so that the glyphs arrive in
// the order the text was written. Everything that counts runes along a line
// depends on that, and a caller drawing the glyphs is unaffected because each
// one carries the position it goes at.
func (s *Shaper) NextGlyph() (_ Glyph, ok bool) {
	s.init()
	if s.done {
		return Glyph{}, false
	}
	for {
		if s.line == len(s.txt.lines) {
			if s.brokeParagraph {
				s.brokeParagraph = false
				return s.paragraphStart, true
			}
			if s.err == nil {
				s.err = io.EOF
			}
			return Glyph{}, false
		}
		line := s.txt.lines[s.line]
		if s.run == len(line.runs) {
			s.line++
			s.run = 0
			continue
		}
		run := line.runs[s.run]
		align := s.txt.alignment.Align(line.direction, line.width, s.txt.alignWidth)
		if s.line == 0 && s.run == 0 && len(run.Glyphs) == 0 {
			// The whole text was the empty string. One glyph standing for no
			// text at all goes back, carrying the line's height: a caller
			// drawing an empty field still has to give it a height, and this
			// is where that number comes from.
			s.done = true
			return Glyph{
				X:       align,
				Y:       int32(line.yOffset),
				Runes:   0,
				Flags:   FlagLineBreak | FlagClusterBreak | FlagRunBreak,
				Ascent:  line.ascent,
				Descent: line.descent,
			}, true
		}
		if s.glyph == len(run.Glyphs) {
			s.run++
			s.glyph = 0
			s.advance = 0
			continue
		}
		glyphIdx := s.glyph
		rtl := run.Direction.Progression() == system.TowardOrigin
		if rtl {
			glyphIdx = len(run.Glyphs) - 1 - glyphIdx
		}
		g := run.Glyphs[glyphIdx]
		if rtl {
			// Right to left the dot ends up on the far side of the glyph, so
			// the width is added before the offset is computed rather than
			// after. Added afterwards, every glyph of the run is drawn one
			// glyph's width away from where it belongs.
			s.advance += g.advance
		}
		runOffset := s.advance
		if rtl {
			runOffset = run.Advance - s.advance
		}
		glyph := Glyph{
			ID:      g.id,
			X:       align + run.X + runOffset,
			Y:       int32(line.yOffset),
			Ascent:  line.ascent,
			Descent: line.descent,
			Advance: g.advance,
			Runes:   uint16(g.runeCount),
			Offset: fixed.Point26_6{
				X: g.xOffset,
				Y: g.yOffset,
			},
			Bounds: g.bounds,
		}
		if run.truncator {
			glyph.Flags |= FlagTruncator
		}
		s.glyph++
		if !rtl {
			s.advance += g.advance
		}

		endOfRun := s.glyph == len(run.Glyphs)
		if endOfRun {
			glyph.Flags |= FlagRunBreak
		}
		endOfLine := endOfRun && s.run == len(line.runs)-1
		if endOfLine {
			glyph.Flags |= FlagLineBreak
		}
		endOfText := endOfLine && s.line == len(s.txt.lines)-1
		nextGlyph := s.glyph
		if rtl {
			nextGlyph = len(run.Glyphs) - 1 - nextGlyph
		}
		endOfCluster := endOfRun || run.Glyphs[nextGlyph].clusterIndex != g.clusterIndex
		if run.truncator {
			// The whole truncator is one cluster, however many glyphs it took
			// to draw. It stands for a stretch of text, not for any run of
			// runes inside it, and splitting it would offer positions within
			// text that is not on screen.
			endOfCluster = endOfRun
		}
		if endOfCluster {
			glyph.Flags |= FlagClusterBreak
			if run.truncator {
				glyph.Runes += uint16(s.txt.unreadRuneCount)
			}
		} else {
			// The count belongs to the cluster and is carried by its last
			// glyph, so that summing it over a sequence counts each cluster
			// once instead of once per glyph.
			glyph.Runes = 0
		}
		if run.Direction.Progression() == system.TowardOrigin {
			glyph.Flags |= FlagTowardOrigin
		}
		if s.brokeParagraph {
			glyph.Flags |= FlagParagraphStart
			s.brokeParagraph = false
		}
		if g.glyphCount == 0 {
			glyph.Flags |= FlagParagraphBreak
			s.brokeParagraph = true
			if endOfText {
				// A paragraph break that is also the end of the text is a
				// newline with nothing after it. The line it opens has no
				// glyphs to carry its position, so one is made: without it
				// there is nowhere for a caret to stand after the last
				// newline, and pressing return at the end of a text appears to
				// do nothing.
				s.paragraphStart = Glyph{
					Ascent:  glyph.Ascent,
					Descent: glyph.Descent,
					Flags:   FlagParagraphStart | FlagLineBreak | FlagRunBreak | FlagClusterBreak,
				}
				s.paragraphStart.X = s.txt.alignment.Align(line.direction, 0, s.txt.alignWidth)
				s.paragraphStart.Y = glyph.Y + int32(line.lineHeight.Round())
			}
		}
		return glyph, true
	}
}

// Shape turns glyphs into one path enclosing all their outlines.
//
// The glyphs are expected to come from a single line: their vertical positions
// are ignored, because a path built across lines would have to be rebuilt
// whenever any line of the text moved.
func (s *Shaper) Shape(gs []Glyph) clip.PathSpec {
	s.init()
	key := s.pathCache.hashGlyphs(gs)
	shape, ok := s.pathCache.Get(key, gs)
	if ok {
		return shape
	}
	pathOps := new(op.Ops)
	shape = s.shaper.Shape(pathOps, gs)
	s.pathCache.Put(key, gs, shape)
	return shape
}

// Bitmaps draws the glyphs of the sequence that are pictures rather than
// outlines, and returns the call that presents them.
//
// Two calls rather than one because the two are drawn by different means: an
// outline is filled and a picture is placed. A sequence mixing them -- a line
// of text with an emoji in it -- needs both, and what comes back here lines up
// with what [Shaper.Shape] returns for the same glyphs.
//
// The glyphs are expected to come from a single line, for the same reason.
func (s *Shaper) Bitmaps(gs []Glyph) op.CallOp {
	s.init()
	key := s.bitmapShapeCache.hashGlyphs(gs)
	call, ok := s.bitmapShapeCache.Get(key, gs)
	if ok {
		return call
	}
	callOps := new(op.Ops)
	call = s.shaper.Bitmaps(callOps, gs)
	s.bitmapShapeCache.Put(key, gs, call)
	return call
}
