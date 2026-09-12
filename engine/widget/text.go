package widget

import (
	"bufio"
	"image"
	"io"
	"math"
	"slices"
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"golang.org/x/image/math/fixed"
)

// textSource is the byte-oriented storage used by an interactive text view.
// Storage implementations surface their own persistent I/O failures; the view
// deliberately treats short reads as the available contents.
type textSource interface {
	io.ReaderAt

	// Size is measured in bytes, while editing operations are measured in runes.
	Size() int64
	Changed() bool
	ReplaceRunes(byteOffset int64, runeCount int64, replacement string)
}

type textCaret struct {
	// start is both the insertion point and one end of the selection. It may be
	// greater than end when a selection was extended backwards.
	start, end int
	// xoff preserves the intended horizontal position across vertical moves.
	xoff fixed.Int26_6
}

// textView owns the conversions between source bytes, logical rune positions,
// and shaped pixel positions. Shaping is lazy: any input change invalidates the
// derived indexes and the next geometry query rebuilds them.
type textView struct {
	Alignment text.Alignment
	// LineHeight is the requested distance between adjacent baselines. Zero asks
	// the shaper for its normal value.
	LineHeight unit.Sp
	// LineHeightScale multiplies the normal line height when it is non-zero.
	LineHeightScale float32
	// SingleLine disables wrapping and changes scrolling to the horizontal axis.
	SingleLine bool
	// MaxLines limits the shaped output; zero leaves it unrestricted.
	MaxLines int
	// Truncator marks omitted text when MaxLines is reached.
	Truncator  string
	WrapPolicy text.WrapPolicy
	// DisableSpaceTrim retains the advance of whitespace at a line boundary.
	DisableSpaceTrim bool
	// Mask substitutes each non-newline rune only while shaping. Source reads and
	// editing continue to use the original contents.
	Mask rune

	rr         textSource
	shaper     *text.Shaper
	params     text.Parameters
	lastMask   rune
	seekCursor int64

	maskReader      maskReader
	paragraphReader graphemeReader
	graphemes       []int
	offIndex        []offEntry
	index           glyphIndex

	dims      layout.Dimensions
	viewSize  image.Point
	scrollOff image.Point
	caret     textCaret
	regions   []Region

	valid   bool
	version uint64
}

func (e *textView) Changed() bool {
	return e.rr.Changed()
}

// Dimensions reports the constrained viewport while retaining the document's
// baseline distance from its bottom edge.
func (e *textView) Dimensions() layout.Dimensions {
	belowBaseline := e.dims.Size.Y - e.dims.Baseline
	return layout.Dimensions{
		Size:     e.viewSize,
		Baseline: e.viewSize.Y - belowBaseline,
	}
}

// FullDimensions reports the unconstrained shaped document.
func (e *textView) FullDimensions() layout.Dimensions {
	return e.dims
}

// SetSource replaces the backing store and drops every derived index.
func (e *textView) SetSource(source textSource) {
	e.rr = source
	e.seekCursor = 0
	e.invalidate()
}

// ReadRuneAt decodes the rune beginning at off.
func (e *textView) ReadRuneAt(off int64) (rune, int, error) {
	var buf [utf8.UTFMax]byte
	n, err := e.rr.ReadAt(buf[:], off)
	r, size := utf8.DecodeRune(buf[:n])
	return r, size, err
}

// ReadRuneBefore decodes the rune ending immediately before off.
func (e *textView) ReadRuneBefore(off int64) (rune, int, error) {
	var buf [utf8.UTFMax]byte
	start := off - utf8.UTFMax
	if start < 0 {
		start = 0
	}
	want := int(off - start)
	n, err := e.rr.ReadAt(buf[:want], start)
	r, size := utf8.DecodeLastRune(buf[:n])
	return r, size, err
}

func (e *textView) makeValid() {
	if e.valid {
		return
	}
	e.layoutText(e.shaper)
	e.valid = true
}

func (e *textView) closestToRune(runeIdx int) combinedPos {
	e.makeValid()
	pos, _ := e.index.closestToRune(runeIdx)
	return pos
}

func (e *textView) closestToLineCol(line, col int) combinedPos {
	e.makeValid()
	return e.index.closestToLineCol(screenPos{line: line, col: col})
}

func (e *textView) closestToXY(x fixed.Int26_6, y int) (combinedPos, bool) {
	e.makeValid()
	return e.index.closestToXY(x, y)
}

func (e *textView) closestToXYGraphemes(x fixed.Int26_6, y int) (combinedPos, bool) {
	pos, atEndOfLine := e.closestToXY(x, y)
	if atEndOfLine {
		return pos, true
	}

	firstRune := e.moveByGraphemes(pos.runes, 0)
	direction := 1
	if firstRune > pos.runes {
		direction = -1
	}
	secondRune := e.moveByGraphemes(firstRune, direction)
	first := e.closestToRune(firstRune)
	second := e.closestToRune(secondRune)
	if absFixed(first.x-x) > absFixed(second.x-x) {
		return second, false
	}
	return first, false
}

func absFixed(i fixed.Int26_6) fixed.Int26_6 {
	if i < 0 {
		return -i
	}
	return i
}

// MoveLines moves the caret vertically while retaining its preferred x.
func (e *textView) MoveLines(distance int, selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	x := caret.x + e.caret.xoff
	pos := e.closestToLineCol(caret.lineCol.line+distance, 0)
	pos, atEndOfLine := e.closestToXYGraphemes(x, pos.y)
	if atEndOfLine && pos.runes > 0 {
		pos.runes = e.moveByGraphemes(pos.runes, -1)
	}
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

// calculateViewSize reserves horizontal space for an empty field's caret, then
// applies the caller's constraints.
func (e *textView) calculateViewSize(gtx layout.Context) image.Point {
	size := e.dims.Size
	if width := e.caretWidth(gtx); size.X < width {
		size.X = width
	}
	return gtx.Constraints.Constrain(size)
}

func (e *textView) syncLayoutInputs(gtx layout.Context, shaper *text.Shaper, face font.Font, size unit.Sp) {
	maxWidth := gtx.Constraints.Max.X
	if e.SingleLine {
		maxWidth = math.MaxInt
	}

	next := e.params
	next.Locale = gtx.Locale
	next.Font = face
	next.PxPerEm = fixed.I(gtx.Sp(size))
	next.MinWidth = gtx.Constraints.Min.X
	next.MaxWidth = maxWidth
	next.Alignment = e.Alignment
	next.Truncator = e.Truncator
	next.MaxLines = e.MaxLines
	next.WrapPolicy = e.WrapPolicy
	next.LineHeight = fixed.I(gtx.Sp(e.LineHeight))
	next.LineHeightScale = e.LineHeightScale
	next.DisableSpaceTrim = e.DisableSpaceTrim

	if next != e.params || shaper != e.shaper || e.Mask != e.lastMask {
		e.params = next
		e.shaper = shaper
		e.lastMask = e.Mask
		e.invalidate()
	}
}

// Layout synchronizes shaping inputs and constrains the resulting document to
// the current viewport.
func (e *textView) Layout(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp) {
	e.syncLayoutInputs(gtx, lt, font, size)
	e.makeValid()
	nextView := e.calculateViewSize(gtx)
	if nextView != e.viewSize {
		e.viewSize = nextView
		e.invalidate()
		e.makeValid()
	}
}

func (e *textView) documentViewport() image.Rectangle {
	return image.Rectangle{Min: e.scrollOff, Max: e.viewSize.Add(e.scrollOff)}
}

// PaintSelection paints only the portion of the current selection inside the
// viewport.
func (e *textView) PaintSelection(gtx layout.Context, material op.CallOp) {
	defer clip.Rect(image.Rectangle{Max: e.viewSize}).Push(gtx.Ops).Pop()
	e.regions = e.index.locate(e.documentViewport(), e.caret.start, e.caret.end, e.regions)
	for _, region := range e.regions {
		selection := clip.Rect(region.Bounds).Push(gtx.Ops)
		material.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		selection.Pop()
	}
}

func (e *textView) firstVisibleGlyph(viewport image.Rectangle) int {
	first := 0
	for _, line := range e.index.lines {
		if line.yOff+line.descent.Ceil() >= viewport.Min.Y {
			break
		}
		first += line.glyphs
	}
	return first
}

// PaintText records the visible glyphs, then clips their padded outline to the
// viewport before replaying the recording.
func (e *textView) PaintText(gtx layout.Context, material op.CallOp) {
	recording := op.Record(gtx.Ops)
	viewport := e.documentViewport()
	it := textIterator{
		viewport: viewport,
		material: material,
	}

	var scratch [32]text.Glyph
	line := scratch[:0]
	for _, glyph := range e.index.glyphs[e.firstVisibleGlyph(viewport):] {
		var visible bool
		line, visible = it.paintGlyph(gtx, e.shaper, glyph, line)
		if !visible {
			break
		}
	}

	call := recording.Stop()
	viewport.Min = viewport.Min.Add(it.padding.Min)
	viewport.Max = viewport.Max.Add(it.padding.Max)
	defer clip.Rect(viewport.Sub(e.scrollOff)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

func (e *textView) caretWidth(gtx layout.Context) int {
	return max(gtx.Dp(1)/2, 1)
}

// PaintCaret draws the insertion caret if any part of it intersects the view.
func (e *textView) PaintCaret(gtx layout.Context, material op.CallOp) {
	halfWidth := e.caretWidth(gtx)
	position, ascent, descent := e.CaretInfo()

	caret := image.Rectangle{
		Min: position.Sub(image.Pt(halfWidth, ascent)),
		Max: position.Add(image.Pt(halfWidth, descent)),
	}
	caret = image.Rectangle{Max: e.viewSize}.Intersect(caret)
	if !caret.Empty() {
		defer clip.Rect(caret).Push(gtx.Ops).Pop()
		material.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
}

func (e *textView) CaretInfo() (pos image.Point, ascent, descent int) {
	caretStart := e.closestToRune(e.caret.start)

	ascent = caretStart.ascent.Ceil()
	descent = caretStart.descent.Ceil()

	pos = image.Pt(caretStart.x.Round(), caretStart.y).Sub(e.scrollOff)
	return
}

// ByteOffset converts a logical rune offset to a source byte offset, clamping
// through the shaped rune index first.
func (e *textView) ByteOffset(runeOffset int) int64 {
	return int64(e.runeOffset(e.closestToRune(runeOffset).runes))
}

// Len is the number of shaped source runes.
func (e *textView) Len() int {
	e.makeValid()
	return e.closestToRune(math.MaxInt).runes
}

// Text copies the source bytes into buf, reusing its allocation when possible.
func (e *textView) Text(buf []byte) []byte {
	size := int(e.rr.Size())
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	_, _ = e.Seek(0, io.SeekStart)
	n, _ := io.ReadFull(e, buf)
	return buf[:n]
}

// ScrollBounds returns the legal document offsets for the active scroll axis.
func (e *textView) ScrollBounds() image.Rectangle {
	var bounds image.Rectangle
	if e.SingleLine {
		if len(e.index.lines) > 0 {
			line := e.index.lines[0]
			bounds.Min.X = min(line.xOff.Floor(), 0)
		}
		bounds.Max.X = e.dims.Size.X + bounds.Min.X - e.viewSize.X
	} else {
		bounds.Max.Y = e.dims.Size.Y - e.viewSize.Y
	}
	return bounds
}

func (e *textView) ScrollRel(dx, dy int) {
	e.scrollAbs(e.scrollOff.X+dx, e.scrollOff.Y+dy)
}

// ScrollOff returns the viewport origin in document coordinates.
func (e *textView) ScrollOff() image.Point {
	return e.scrollOff
}

func (e *textView) scrollAbs(x, y int) {
	bounds := e.ScrollBounds()
	e.scrollOff = image.Pt(
		max(bounds.Min.X, min(x, bounds.Max.X)),
		max(bounds.Min.Y, min(y, bounds.Max.Y)),
	)
}

// MoveCoord maps a viewport pixel to the nearest grapheme boundary.
func (e *textView) MoveCoord(pos image.Point) {
	documentPoint := pos.Add(e.scrollOff)
	caret, _ := e.closestToXYGraphemes(fixed.I(documentPoint.X), documentPoint.Y)
	e.caret.start = caret.runes
	e.caret.xoff = 0
}

// Truncated reports whether line limiting caused the shaper to add a truncator.
func (e *textView) Truncated() bool {
	return e.index.truncated
}

func (e *textView) readerForShaping() io.Reader {
	_, _ = e.Seek(0, io.SeekStart)
	var r io.Reader = e
	if e.Mask != 0 {
		e.maskReader.Reset(e, e.Mask)
		r = &e.maskReader
	}
	return r
}

func (e *textView) indexShapedText(lt *text.Shaper, source io.Reader, it *textIterator) {
	if lt != nil {
		lt.Layout(e.params, source)
		for {
			glyph, ok := lt.NextGlyph()
			if !it.processGlyph(glyph, ok) {
				break
			}
			e.index.Glyph(glyph)
		}
		return
	}

	// A nil shaper is useful to callers that only need logical indexing.
	runes := bufio.NewReader(source)
	for {
		_, _, err := runes.ReadRune()
		if err == io.EOF {
			break
		}
		glyph := text.Glyph{Runes: 1, Flags: text.FlagClusterBreak}
		_ = it.processGlyph(glyph, true)
		e.index.Glyph(glyph)
	}
}

func (e *textView) indexGraphemes() {
	e.paragraphReader.SetSource(e.rr)
	e.graphemes = e.graphemes[:0]
	for boundaries := e.paragraphReader.Graphemes(); len(boundaries) > 0; boundaries = e.paragraphReader.Graphemes() {
		if len(e.graphemes) > 0 && boundaries[0] == e.graphemes[len(e.graphemes)-1] {
			boundaries = boundaries[1:]
		}
		e.graphemes = append(e.graphemes, boundaries...)
	}
}

func (e *textView) layoutText(lt *text.Shaper) {
	e.index.reset()
	it := textIterator{viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)}}
	e.indexShapedText(lt, e.readerForShaping(), &it)
	e.indexGraphemes()

	e.dims.Size = it.bounds.Size()
	e.dims.Baseline = e.dims.Size.Y - it.baseline
}

// CaretPos returns zero-based logical line and rune-column coordinates.
func (e *textView) CaretPos() (line, col int) {
	pos := e.closestToRune(e.caret.start)
	return pos.lineCol.line, pos.lineCol.col
}

// CaretCoords maps the logical caret to viewport pixels.
func (e *textView) CaretCoords() f32.Point {
	pos := e.closestToRune(e.caret.start)
	return f32.Pt(float32(pos.x)/64-float32(e.scrollOff.X), float32(pos.y-e.scrollOff.Y))
}

// indexRune finds the nearest cached byte conversion at or before r.
func (e *textView) indexRune(r int) offEntry {
	if len(e.offIndex) == 0 {
		e.offIndex = append(e.offIndex, offEntry{})
	}
	i := sort.Search(len(e.offIndex), func(i int) bool {
		return e.offIndex[i].runes >= r
	})
	if i > 0 {
		i--
	}
	return e.offIndex[i]
}

// runeOffset walks UTF-8 from a sparse checkpoint to the requested rune.
func (e *textView) runeOffset(r int) int {
	const runesPerIndexEntry = 50
	entry := e.indexRune(r)
	indexedThrough := e.offIndex[len(e.offIndex)-1].runes
	for entry.runes < r {
		if entry.runes > indexedThrough && entry.runes%runesPerIndexEntry == runesPerIndexEntry-1 {
			e.offIndex = append(e.offIndex, entry)
		}
		_, size, _ := e.ReadRuneAt(int64(entry.bytes))
		entry.bytes += size
		entry.runes++
	}
	return entry.bytes
}

func (e *textView) invalidate() {
	e.valid = false
	e.offIndex = e.offIndex[:0]
	e.version++
}

func positionAfterReplace(position, oldStart, oldEnd, newEnd int) int {
	switch {
	case newEnd < position && position <= oldEnd:
		return newEnd
	case oldEnd < position:
		return position + newEnd - oldEnd
	default:
		return position
	}
}

// Replace substitutes the rune range [start,end) and keeps both selection
// endpoints attached to the surrounding logical text. It returns the number of
// inserted runes.
func (e *textView) Replace(start, end int, s string) int {
	if start > end {
		start, end = end, start
	}
	oldStart := e.closestToRune(start).runes
	oldEnd := e.closestToRune(end).runes
	byteStart := e.runeOffset(oldStart)
	inserted := utf8.RuneCountInString(s)
	newEnd := oldStart + inserted

	e.rr.ReplaceRunes(int64(byteStart), int64(oldEnd-oldStart), s)
	e.caret.start = positionAfterReplace(e.caret.start, oldStart, oldEnd, newEnd)
	e.caret.end = positionAfterReplace(e.caret.end, oldStart, oldEnd, newEnd)
	e.invalidate()
	return inserted
}

// MovePages moves vertically by viewport heights and preserves the desired x.
func (e *textView) MovePages(pages int, selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	x := caret.x + e.caret.xoff
	y := caret.y + pages*e.viewSize.Y
	pos, _ := e.closestToXYGraphemes(x, y)
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

// moveByGraphemes advances from a rune position through user-perceived
// characters rather than through individual code points.
func (e *textView) moveByGraphemes(startRune, distance int) int {
	if len(e.graphemes) == 0 {
		return startRune
	}
	boundary, _ := slices.BinarySearch(e.graphemes, startRune)
	boundary = max(0, min(boundary+distance, len(e.graphemes)-1))
	return e.closestToRune(e.graphemes[boundary]).runes
}

func (e *textView) clampCursorToGraphemes() {
	e.caret.start = e.moveByGraphemes(e.caret.start, 0)
	e.caret.end = e.moveByGraphemes(e.caret.end, 0)
}

// MoveCaret moves both selection endpoints by grapheme cluster counts.
func (e *textView) MoveCaret(startDelta, endDelta int) {
	e.caret.xoff = 0
	e.caret.start = e.moveByGraphemes(e.caret.start, startDelta)
	e.caret.end = e.moveByGraphemes(e.caret.end, endDelta)
}

func (e *textView) MoveTextStart(selAct selectionAction) {
	selectionEnd := e.closestToRune(e.caret.end)
	e.caret.start = 0
	e.caret.end = selectionEnd.runes
	e.caret.xoff = -selectionEnd.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

func (e *textView) MoveTextEnd(selAct selectionAction) {
	end := e.closestToRune(math.MaxInt)
	e.caret.start = end.runes
	e.caret.xoff = fixed.I(e.params.MaxWidth) - end.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

func (e *textView) MoveLineStart(selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	start := e.closestToLineCol(caret.lineCol.line, 0)
	e.caret.start = start.runes
	e.caret.xoff = -start.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

func (e *textView) MoveLineEnd(selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	end := e.closestToLineCol(caret.lineCol.line, math.MaxInt)
	e.caret.start = end.runes
	e.caret.xoff = fixed.I(e.params.MaxWidth) - end.x
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

func wordMotion(distance int) (count, direction int) {
	if distance < 0 {
		return -distance, -1
	}
	return distance, 1
}

func (e *textView) runeBeside(caret combinedPos, direction int) rune {
	byteOffset := e.runeOffset(caret.runes)
	if direction < 0 {
		r, _, _ := e.ReadRuneBefore(int64(byteOffset))
		return r
	}
	r, _, _ := e.ReadRuneAt(int64(byteOffset))
	return r
}

// MoveWord crosses whitespace-delimited words in either direction. This is a
// deliberately simple boundary rule; scripts without separating whitespace
// need a richer word segmenter in a future change.
func (e *textView) MoveWord(distance int, selAct selectionAction) {
	words, direction := wordMotion(distance)
	caret := e.closestToRune(e.caret.start)
	atBoundary := func() bool { return caret.runes == 0 || caret.runes == e.Len() }
	advance := func() { e.MoveCaret(direction, 0); caret = e.closestToRune(e.caret.start) }

	for range words {
		for unicode.IsSpace(e.runeBeside(caret, direction)) && !atBoundary() {
			advance()
		}
		advance()
		for !unicode.IsSpace(e.runeBeside(caret, direction)) && !atBoundary() {
			advance()
		}
	}
	e.updateSelection(selAct)
	e.clampCursorToGraphemes()
}

func (e *textView) ScrollToCaret() {
	caret := e.closestToRune(e.caret.start)
	if e.SingleLine {
		delta := 0
		if before := caret.x.Floor() - e.scrollOff.X; before < 0 {
			delta = before
		} else if after := caret.x.Ceil() - e.scrollOff.X - e.viewSize.X; after > 0 {
			delta = after
		}
		e.ScrollRel(delta, 0)
		return
	}

	top := caret.y - caret.ascent.Ceil()
	bottom := caret.y + caret.descent.Ceil()
	delta := 0
	if before := top - e.scrollOff.Y; before < 0 {
		delta = before
	} else if after := bottom - e.scrollOff.Y - e.viewSize.Y; after > 0 {
		delta = after
	}
	e.ScrollRel(0, delta)
}

// SelectionLen returns the absolute selection length in runes.
func (e *textView) SelectionLen() int {
	return abs(e.caret.start - e.caret.end)
}

// Selection returns the directed selection endpoints as rune offsets.
func (e *textView) Selection() (start, end int) {
	return e.caret.start, e.caret.end
}

// SetCaret sets rune offsets for both ends, clamped to shaped grapheme bounds.
func (e *textView) SetCaret(start, end int) {
	e.caret.start = e.closestToRune(start).runes
	e.caret.end = e.closestToRune(end).runes
	e.clampCursorToGraphemes()
}

// SelectedText copies the source bytes covered by the rune selection.
func (e *textView) SelectedText(buf []byte) []byte {
	a := e.runeOffset(e.caret.start)
	b := e.runeOffset(e.caret.end)
	start, end := min(a, b), max(a, b)
	size := end - start
	if cap(buf) < size {
		buf = make([]byte, size)
	}
	buf = buf[:size]
	n, _ := e.rr.ReadAt(buf, int64(start))
	return buf[:n]
}

func (e *textView) updateSelection(selAct selectionAction) {
	if selAct == selectionClear {
		e.ClearSelection()
	}
}

// ClearSelection collapses the selection onto the insertion point.
func (e *textView) ClearSelection() {
	e.caret.end = e.caret.start
}

// WriteTo implements io.WriterTo.
func (e *textView) WriteTo(w io.Writer) (int64, error) {
	_, _ = e.Seek(0, io.SeekStart)
	return io.Copy(w, struct{ io.Reader }{e})
}

// Seek implements io.Seeker.
func (e *textView) Seek(offset int64, whence int) (int64, error) {
	next := e.seekCursor
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next += offset
	case io.SeekEnd:
		next = e.rr.Size() + offset
	}
	e.seekCursor = next
	return e.seekCursor, nil
}

// Read implements io.Reader.
func (e *textView) Read(p []byte) (int, error) {
	n, err := e.rr.ReadAt(p, e.seekCursor)
	e.seekCursor += int64(n)
	return n, err
}

// ReadAt implements io.ReaderAt.
func (e *textView) ReadAt(p []byte, offset int64) (int, error) {
	return e.rr.ReadAt(p, offset)
}

// Regions returns visible regions covering the rune range [start,end).
func (e *textView) Regions(start, end int, regions []Region) []Region {
	return e.index.locate(e.documentViewport(), start, end, regions)
}
