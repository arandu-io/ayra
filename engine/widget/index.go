package widget

import (
	"bufio"
	"image"
	"io"
	"math"
	"sort"

	"github.com/arandu-io/ayra/engine/text"
	"github.com/go-text/typesetting/segmenter"
	"golang.org/x/image/math/fixed"
)

// lineInfo is the shape of one laid-out line: where it starts, how wide it is,
// and how far it reaches above and below its own baseline.
//
// It is kept beside the positions rather than derived from them, because the
// two answer different questions. A position knows where a caret goes; a line
// knows where the text stops, which is what a click past the last letter and a
// highlight that runs to the edge both need.
type lineInfo struct {
	// xOff is where the line begins. It is not zero for centred or
	// right-aligned text, and it is negative for a line that hangs left of
	// the origin.
	xOff fixed.Int26_6
	// yOff is the baseline of the line, in pixels down from the top.
	yOff int
	// width is how far the line extends from xOff.
	width fixed.Int26_6
	// ascent and descent are the room the line takes above and below its
	// baseline. They come from the glyphs actually on the line, so a line of
	// tall letters is taller than a line of short ones.
	ascent, descent fixed.Int26_6
	// glyphs is how many glyphs the line holds. Painting counts with it to
	// skip whole lines above the viewport without measuring them.
	glyphs int
}

// end is the far edge of the line.
func (l lineInfo) end() fixed.Int26_6 {
	return l.xOff + l.width
}

// pendingLine gathers geometry until the shaper marks the line complete.
// Keeping this transient state separate from lineInfo makes it impossible to
// expose a half-built line to lookups.
type pendingLine struct {
	left, right fixed.Int26_6
	glyphs      int
	started     bool
}

func (l *pendingLine) include(gl text.Glyph) {
	if !l.started {
		l.left = gl.X
		l.right = 0
		l.started = true
	}
	l.left = min(l.left, gl.X)
	l.right = max(l.right, gl.X+gl.Advance)
	l.glyphs++
}

func (l *pendingLine) finish(y int, ascent, descent fixed.Int26_6) lineInfo {
	line := lineInfo{
		xOff:    l.left,
		yOff:    y,
		width:   l.right - l.left,
		ascent:  ascent,
		descent: descent,
		glyphs:  l.glyphs,
	}
	*l = pendingLine{}
	return line
}

type pendingCluster struct {
	advance fixed.Int26_6
	open    bool
}

// glyphIndex is the map between the three ways a caret can be named: a rune
// offset, a line and column, and a point on the screen.
//
// It exists because a shaper answers in glyphs and everything above it counts
// in runes, and the two are not the same number. A ligature is one glyph for
// two runes; a letter and the accent over it are one glyph for two runes as
// well. Anything that walks glyphs and adds one per glyph is correct in English
// and loses a rune per accent everywhere else, so the index takes the rune count
// the shaper reports for each glyph and spreads the caret positions across the
// glyph's advance. That is why it is built glyph by glyph and read rune by rune.
//
// The positions are appended in order, which is the whole reason the lookups can
// be binary searches rather than scans of the text.
type glyphIndex struct {
	// glyphs holds every glyph that was indexed, in the order the shaper
	// produced them. Painting reads it back.
	glyphs []text.Glyph
	// positions holds every place a caret may sit, sorted by rune offset.
	// There is one more of them than there are runes, because a caret can
	// also sit after the last one.
	positions []combinedPos
	// lines describes each line of the laid-out text.
	lines []lineInfo

	// caret is the next caret position to be inserted. It is carried between
	// glyphs because a cluster's runes are numbered from where the previous
	// cluster left off.
	caret combinedPos
	line  pendingLine
	span  pendingCluster
	// truncated records that the shaper dropped text it could not fit.
	truncated bool
}

// reset empties the index for the next layout without giving back its memory.
//
// The text is re-indexed on every change, so this runs as often as a key is
// pressed. Allocating the slices again each time would make typing allocate in
// proportion to the length of the document.
func (g *glyphIndex) reset() {
	g.glyphs = g.glyphs[:0]
	g.positions = g.positions[:0]
	g.lines = g.lines[:0]
	g.caret = combinedPos{}
	g.line = pendingLine{}
	g.span = pendingCluster{}
	g.truncated = false
}

// screenPos is a caret named by line and column rather than by offset.
//
// The column is counted in runes and is only ever asked for at the start or the
// end of a line, so it is a coarse number on purpose: moving the caret down a
// line seeks by column and then corrects by pixel, because no column count
// survives proportional type.
type screenPos struct {
	col  int
	line int
}

// combinedPos is one place a caret may sit, named every way at once.
//
// The three namings are held together rather than converted on demand because
// each conversion is a search, and the caret is asked to name itself several
// times per frame -- to be drawn, to be scrolled to, to be compared with the
// other end of a selection.
type combinedPos struct {
	// runes is the offset into the text, counted in runes.
	runes int

	lineCol screenPos

	// x and y are where the caret is drawn. x carries the fractional part the
	// shaper works in, because rounding it here would drift a caret across a
	// long line.
	x fixed.Int26_6
	y int

	// ascent and descent are the height of the caret here, taken from the
	// glyphs around it rather than from the font, so a caret beside a tall
	// glyph is as tall as the glyph.
	ascent, descent fixed.Int26_6

	// runIndex is which run of the line this position belongs to. A line of
	// mixed direction is several runs, and a selection has to be drawn as one
	// rectangle per run.
	runIndex int
	// towardOrigin is whether this position's run reads right to left.
	towardOrigin bool
}

// incrementPosition returns the position after pos, and reports when there is
// none.
//
// pos has to be a position this index handed out, unmodified, because the walk
// starts by finding it again. A synthesised position is not equal to anything
// stored, and the search would then walk to the end looking for it.
func (g *glyphIndex) incrementPosition(pos combinedPos) (next combinedPos, eof bool) {
	_, index := g.closestToRune(pos.runes)
	for index < len(g.positions) && g.positions[index] != pos {
		index++
	}
	if index < len(g.positions)-1 {
		return g.positions[index+1], false
	}
	if index < len(g.positions) {
		return g.positions[index], true
	}
	return pos, true
}

// insertPosition appends pos, unless it replaces the one before it.
//
// A soft line break produces two positions for the same rune: one at the end of
// the line it broke and one at the start of the next. Both are wanted, and they
// differ in y. What is not wanted is the pair a line break also produces at the
// same point on the same line -- two carets the eye cannot tell apart, where
// pressing an arrow key appears to do nothing because the caret moved from one
// to the other.
func (g *glyphIndex) insertPosition(pos combinedPos) {
	if count := len(g.positions); count > 0 {
		last := &g.positions[count-1]
		sameLogicalOffset := last.runes == pos.runes
		samePoint := last.x == pos.x && last.y == pos.y
		lineChanged := last.y != pos.y
		if sameLogicalOffset && (samePoint || lineChanged) {
			*last = pos
			return
		}
	}
	g.positions = append(g.positions, pos)
}

func (g *glyphIndex) beginCluster(gl text.Glyph) {
	g.caret.x = gl.X
	g.caret.y = int(gl.Y)
	g.caret.ascent = gl.Ascent
	g.caret.descent = gl.Descent
	if g.caret.towardOrigin {
		g.caret.x += gl.Advance
	}
	g.insertPosition(g.caret)
}

func (g *glyphIndex) closeCluster(gl text.Glyph) {
	steps := int(gl.Runes)
	runesPerStep := 1
	if gl.Flags&text.FlagTruncator != 0 {
		steps = 1
		runesPerStep = int(gl.Runes)
		g.truncated = true
	}
	if steps == 0 {
		g.span.advance = 0
		return
	}

	delta := g.span.advance / fixed.Int26_6(steps)
	origin := fixed.Int26_6(0)
	if g.caret.towardOrigin {
		origin = g.span.advance
		delta = -delta
	}
	for step := 1; step <= steps; step++ {
		g.caret.x = gl.X + origin + fixed.Int26_6(step)*delta
		g.caret.runes += runesPerStep
		g.caret.lineCol.col += runesPerStep
		g.insertPosition(g.caret)
	}
	g.span.advance = 0
}

// Glyph indexes one glyph, generating the caret positions it carries.
//
// The glyphs must arrive in the order the shaper produced them, because
// everything here is accumulated: a cluster's width is the sum of the glyphs
// that make it up, and a line's extent is the sum of its lines' glyphs.
func (g *glyphIndex) Glyph(gl text.Glyph) {
	g.glyphs = append(g.glyphs, gl)
	g.line.include(gl)

	lineBreak := gl.Flags&text.FlagLineBreak != 0
	runBreak := gl.Flags&text.FlagRunBreak != 0
	paragraphBreak := gl.Flags&text.FlagParagraphBreak != 0
	clusterBreak := gl.Flags&text.FlagClusterBreak != 0
	// Positions are generated inside a glyph that closes a cluster, carries
	// runes of its own, and is not a hard newline -- a newline is zero width,
	// and dividing a zero width among its runes puts several carets on one
	// pixel.
	insertPositionsWithin := clusterBreak && !paragraphBreak && gl.Runes > 0

	g.caret.towardOrigin = gl.Flags&text.FlagTowardOrigin != 0
	if !g.span.open {
		// The position before the glyph. In a right-to-left run the leading
		// edge is on the far side of the advance, so the caret goes there.
		g.beginCluster(gl)
	}

	g.span.open = !clusterBreak

	if paragraphBreak {
		// A hard newline gets one caret, not one on each side of it: both
		// would be at the same place, and the second is a caret that cannot
		// be reached by clicking.
		g.span.advance = 0
		g.caret.runes += int(gl.Runes)
	}
	// The advance accumulates whether or not this glyph closes the cluster,
	// because the positions inside a cluster divide the whole of its width.
	g.span.advance += gl.Advance
	if insertPositionsWithin {
		g.caret.y = int(gl.Y)
		g.caret.ascent = gl.Ascent
		g.caret.descent = gl.Descent
		// This is where glyphs stop being runes. The cluster holds gl.Runes of
		// them and is drawn once, so the carets between them are spaced evenly
		// across the drawn width. It is a guess about where the runes are, and
		// it is the only one available: a ligature has no seam to measure, and
		// a combining mark has no width of its own.
		g.closeCluster(gl)
	}
	if runBreak {
		g.caret.runIndex++
	}
	if lineBreak {
		metrics := g.positions[len(g.positions)-1]
		g.lines = append(g.lines, g.line.finish(int(gl.Y), metrics.ascent, metrics.descent))
		g.caret.lineCol.line++
		g.caret.lineCol.col = 0
		g.caret.runIndex = 0
	}
}

// closestToRune returns the position at or after runeIdx, and where it sits in
// the index.
//
// It clamps to the last position rather than failing, because the callers are
// asking where to put a caret and there is no text to put one in past the end.
// An index that has been reset answers with the zero position, which is where a
// caret goes in an empty document.
func (g *glyphIndex) closestToRune(runeIdx int) (combinedPos, int) {
	n := len(g.positions)
	if n == 0 {
		return combinedPos{}, 0
	}
	i := sort.Search(n, func(i int) bool {
		return g.positions[i].runes >= runeIdx
	})
	if i == n {
		return g.positions[n-1], n - 1
	}
	return g.positions[i], i
}

// closestToLineCol returns the position at the given line and column.
//
// A column past the end of its line lands on the first position of the next
// one, and that answer is corrected here: the caret belongs at the end of the
// line that was asked for, not at the start of the one after it. Without the
// correction, clicking to the right of a wrapped line puts the caret on the
// line below, which is the line the eye is not looking at.
func (g *glyphIndex) closestToLineCol(lineCol screenPos) combinedPos {
	n := len(g.positions)
	if n == 0 {
		return combinedPos{}
	}
	i := sort.Search(n, func(i int) bool {
		pos := g.positions[i]
		return pos.lineCol.line > lineCol.line || (pos.lineCol.line == lineCol.line && pos.lineCol.col >= lineCol.col)
	})
	if i == n {
		return g.positions[n-1]
	}
	pos := g.positions[i]
	if foundInNextLine := pos.lineCol.line > lineCol.line; foundInNextLine && i > 0 {
		prior := g.positions[i-1]
		prior.x = g.lines[lineCol.line].end()
		return prior
	}
	return pos
}

// atEndOfLine reports whether the position stored at index is the last one on
// its line.
//
// It takes the index into the positions and not a position, and that is the
// whole point. There is one position per rune only while the text runs in one
// direction: a change of direction inside a line keeps two positions for the
// same rune, so by the end of a mixed line the position stored at index n
// belongs to a rune several places earlier. Looking up "the rune after this
// one" in a slice that is not numbered by rune reads an unrelated entry, and
// the click that should have landed at the end of the line lands inside it.
func (g *glyphIndex) atEndOfLine(index int) bool {
	if index < 0 || index >= len(g.positions) {
		return true
	}
	next := index + 1
	if next >= len(g.positions) {
		return true
	}
	return g.positions[next].lineCol.line > g.positions[index].lineCol.line
}

// dist is the distance between two horizontal coordinates.
func dist(a, b fixed.Int26_6) fixed.Int26_6 {
	if a > b {
		return a - b
	}
	return b - a
}

// closestToXY returns the caret position nearest the given point, and whether
// it is the end of a line.
//
// The line is found by binary search, because the positions are in reading
// order and reading order is top to bottom. The column is not: a line of mixed
// direction has positions whose x coordinates go up, then down, then up again,
// so within the line every position is measured. That is a scan of one line,
// not of the document, which is why it is affordable on every pointer move
// during a drag.
//
// A point past the last line answers with the last position, and a point left
// of the first answers with the first. Callers slice the text by the rune this
// returns, so an answer outside the text is not a misplaced caret, it is a
// panic somewhere else.
func (g *glyphIndex) closestToXY(x fixed.Int26_6, y int) (pos combinedPos, atEndOfLine bool) {
	if len(g.positions) == 0 {
		return combinedPos{}, false
	}
	first := g.positions[0]
	if y < first.y-first.ascent.Ceil() {
		return first, false
	}
	last := g.positions[len(g.positions)-1]
	if y > last.y+last.descent.Round() {
		return last, false
	}
	i := sort.Search(len(g.positions), func(i int) bool {
		pos := g.positions[i]
		return pos.y+pos.descent.Round() >= y
	})
	if i == len(g.positions) {
		// The point is below the text.
		return g.positions[i-1], false
	}
	first = g.positions[i]
	closest := i
	closestDist := dist(first.x, x)
	line := first.lineCol.line
	for i := i + 1; i < len(g.positions) && g.positions[i].lineCol.line == line; i++ {
		candidate := g.positions[i]
		distance := dist(candidate.x, x)
		// Landing on a position exactly ends the search: nothing further along
		// the line can be nearer than the one under the pointer.
		if distance.Round() == 0 {
			return g.positions[i], false
		}
		if distance < closestDist {
			closestDist = distance
			closest = i
		}
	}
	next := closest + 1
	if hasNext := next < len(g.positions); hasNext && g.atEndOfLine(closest) {
		// Past the end of a wrapped line the nearest position is the last one
		// on it, but the caret the reader means is the first one of the next
		// line -- the same place in the text, drawn where they clicked.
		if distance := dist(g.lines[line].end(), x); distance < closestDist {
			return g.positions[next], true
		}
	}
	return g.positions[closest], false
}

// makeRegion builds the rectangle covering a line between two horizontal
// coordinates.
//
// The height comes from the line rather than from the glyphs inside the span,
// so that a highlight over a word with no descenders is the same height as the
// rest of the line it is on. The two coordinates are sorted first, because a
// selection dragged backwards arrives with its ends the other way round and a
// rectangle whose minimum is past its maximum draws as nothing at all.
func makeRegion(line lineInfo, y int, start, end fixed.Int26_6) Region {
	if start > end {
		start, end = end, start
	}
	dotStart := image.Pt(start.Round(), y)
	dotEnd := image.Pt(end.Round(), y)
	return Region{
		Bounds: image.Rectangle{
			Min: dotStart.Sub(image.Point{Y: line.ascent.Ceil()}),
			Max: dotEnd.Add(image.Point{Y: line.descent.Floor()}),
		},
		Baseline: line.descent.Floor(),
	}
}

// Region describes an area of shaped text: where it is, and where its baseline
// sits inside it.
//
// It is what a selection highlight and a spelling underline are drawn from, and
// both need the baseline rather than only the box -- a line drawn at the bottom
// of the box sits under the descenders instead of under the letters.
type Region struct {
	// Bounds is the area, in the coordinates of the widget that holds the
	// text.
	Bounds image.Rectangle
	// Baseline is how many pixels the baseline is above the bottom of Bounds.
	Baseline int
}

// locate returns the regions covering the runes in [startRune,endRune).
//
// One region per line is not enough: a line whose direction changes partway is
// drawn as several rectangles, because the runes between the two ends of the
// selection are not contiguous on the screen even though they are contiguous in
// the text. The walk therefore advances run by run within each line and closes
// a rectangle wherever the direction turns.
//
// Lines outside the viewport are skipped before any of that, since a selection
// may run for thousands of lines and only the ones on screen are painted. The
// returned bounds are relative to the viewport, and rects is reused when it has
// the room, so that dragging a selection does not allocate per frame.
func (g *glyphIndex) locate(viewport image.Rectangle, startRune, endRune int, rects []Region) []Region {
	if startRune > endRune {
		startRune, endRune = endRune, startRune
	}
	rects = rects[:0]
	caretStart, _ := g.closestToRune(startRune)
	caretEnd, _ := g.closestToRune(endRune)

	for lineIdx := caretStart.lineCol.line; lineIdx < len(g.lines); lineIdx++ {
		if lineIdx > caretEnd.lineCol.line {
			break
		}
		pos := g.closestToLineCol(screenPos{line: lineIdx})
		if int(pos.y)+pos.descent.Ceil() < viewport.Min.Y {
			continue
		}
		if int(pos.y)-pos.ascent.Ceil() > viewport.Max.Y {
			break
		}
		line := g.lines[lineIdx]
		if lineIdx > caretStart.lineCol.line && lineIdx < caretEnd.lineCol.line {
			// A line in the middle of the selection is covered end to end, and
			// its direction cannot matter.
			startX := line.xOff
			endX := startX + line.width
			rects = append(rects, makeRegion(line, pos.y, startX, endX))
			continue
		}
		selectionStart := caretStart
		selectionEnd := caretEnd
		if lineIdx != caretStart.lineCol.line {
			selectionStart = g.closestToLineCol(screenPos{line: lineIdx})
		}
		if lineIdx != caretEnd.lineCol.line {
			selectionEnd = g.closestToLineCol(screenPos{line: lineIdx, col: math.MaxInt})
		}

		var (
			startX, endX fixed.Int26_6
			eof          bool
		)
	lineLoop:
		for !eof {
			startX = selectionStart.x
			if selectionStart.runIndex == selectionEnd.runIndex {
				// Both ends are in the same run: one rectangle closes the line.
				endX = selectionEnd.x
				rects = append(rects, makeRegion(line, pos.y, startX, endX))
				break
			}
			currentDirection := selectionStart.towardOrigin
			previous := selectionStart
		runLoop:
			for !eof {
				// Walk to the first position of the next run.
				for startRun := selectionStart.runIndex; selectionStart.runIndex == startRun; {
					previous = selectionStart
					selectionStart, eof = g.incrementPosition(selectionStart)
					if eof {
						endX = selectionStart.x
						rects = append(rects, makeRegion(line, pos.y, startX, endX))
						break runLoop
					}
				}
				if selectionStart.towardOrigin != currentDirection {
					// The text turned around. Close the rectangle at the last
					// position that read the old way, and start another.
					endX = previous.x
					rects = append(rects, makeRegion(line, pos.y, startX, endX))
					break
				}
				if selectionStart.runIndex == selectionEnd.runIndex {
					endX = selectionEnd.x
					rects = append(rects, makeRegion(line, pos.y, startX, endX))
					break lineLoop
				}
			}
		}
	}
	for i := range rects {
		rects[i].Bounds = rects[i].Bounds.Sub(viewport.Min)
	}
	return rects
}

// graphemeReader cuts paragraphs of text into grapheme clusters.
//
// The clusters are what the caret moves by and what a double click selects,
// because a rune is not a character anybody recognises: an accented letter is
// two runes, a flag is two, and an emoji with a skin tone is several. Moving by
// rune through any of those leaves the caret inside something that is drawn as
// one mark.
//
// It reads a paragraph at a time rather than the whole document, so the memory
// it holds is the longest paragraph and not the text.
type graphemeReader struct {
	segmenter.Segmenter
	graphemes  []int
	paragraph  []rune
	source     io.ReaderAt
	cursor     int64
	reader     *bufio.Reader
	runeOffset int
}

// SetSource points the reader at source and starts again from its beginning.
func (p *graphemeReader) SetSource(source io.ReaderAt) {
	p.source = source
	p.cursor = 0
	p.reader = bufio.NewReader(p)
	p.runeOffset = 0
}

// Read exists to satisfy [io.Reader]. It should not be called directly.
func (p *graphemeReader) Read(b []byte) (int, error) {
	n, err := p.source.ReadAt(b, p.cursor)
	p.cursor += int64(n)
	return n, err
}

// next decodes one paragraph of runes, up to and including its newline.
func (p *graphemeReader) next() ([]rune, bool) {
	p.paragraph = p.paragraph[:0]
	var err error
	var r rune
	for err == nil {
		r, _, err = p.reader.ReadRune()
		if err != nil {
			break
		}
		p.paragraph = append(p.paragraph, r)
		if r == '\n' {
			break
		}
	}
	return p.paragraph, err == nil
}

// Graphemes returns the cluster boundaries of the next paragraph, as rune
// offsets into the whole text. An empty result means there are no more
// paragraphs.
//
// The boundaries are offsets rather than clusters, because the callers are
// deciding where a caret may stop, and a list of the places it may stop is
// exactly what they need. The first cluster contributes both of its edges and
// every later one contributes only its far edge, since one cluster's end is the
// next one's beginning.
func (p *graphemeReader) Graphemes() []int {
	var more bool
	p.graphemes = p.graphemes[:0]
	p.paragraph, more = p.next()
	if len(p.paragraph) == 0 && !more {
		return nil
	}
	p.Segmenter.Init(p.paragraph)
	iter := p.Segmenter.GraphemeIterator()
	if iter.Next() {
		graph := iter.Grapheme()
		p.graphemes = append(p.graphemes,
			p.runeOffset+graph.Offset,
			p.runeOffset+graph.Offset+len(graph.Text),
		)
	}
	for iter.Next() {
		graph := iter.Grapheme()
		p.graphemes = append(p.graphemes, p.runeOffset+graph.Offset+len(graph.Text))
	}
	p.runeOffset += len(p.paragraph)
	return p.graphemes
}
