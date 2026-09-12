package widget

import (
	"image"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"golang.org/x/image/math/fixed"
)

// Label lays out and paints non-interactive text.
type Label struct {
	// Alignment places each line within the width offered to the label.
	Alignment text.Alignment
	// MaxLines limits visible lines. Zero permits every line.
	MaxLines int
	// Truncator replaces omitted text on the final line. Empty uses "…".
	Truncator string
	// WrapPolicy chooses where soft line breaks may occur.
	WrapPolicy text.WrapPolicy
	// LineHeight is the distance between baselines. Zero uses the font's
	// natural line height.
	LineHeight unit.Sp
	// LineHeightScale multiplies LineHeight. Zero uses the default scale.
	LineHeightScale float32
}

// Layout shapes and paints content and returns the constrained dimensions.
func (l Label) Layout(gtx layout.Context, shaper *text.Shaper, face font.Font, size unit.Sp, content string, material op.CallOp) layout.Dimensions {
	dims, _ := l.LayoutDetailed(gtx, shaper, face, size, content, material)
	return dims
}

// TextInfo describes content omitted from a label.
type TextInfo struct {
	// Truncated is the number of runes represented by the truncator. Zero means
	// that all content is visible.
	Truncated int
}

// LayoutDetailed is Layout with truncation metadata.
func (l Label) LayoutDetailed(gtx layout.Context, shaper *text.Shaper, face font.Font, size unit.Sp, content string, material op.CallOp) (layout.Dimensions, TextInfo) {
	constraints := gtx.Constraints
	shaper.LayoutString(text.Parameters{
		Font:            face,
		PxPerEm:         fixed.I(gtx.Sp(size)),
		MaxLines:        l.MaxLines,
		Truncator:       l.Truncator,
		Alignment:       l.Alignment,
		WrapPolicy:      l.WrapPolicy,
		MaxWidth:        constraints.Max.X,
		MinWidth:        constraints.Min.X,
		Locale:          gtx.Locale,
		LineHeight:      fixed.I(gtx.Sp(l.LineHeight)),
		LineHeightScale: l.LineHeightScale,
	}, content)

	recording := op.Record(gtx.Ops)
	painter := textIterator{
		viewport: image.Rectangle{Max: constraints.Max},
		maxLines: l.MaxLines,
		material: material,
	}
	semantic.LabelOp(content).Add(gtx.Ops)

	var glyphStorage [32]text.Glyph
	line := glyphStorage[:0]
	for {
		glyph, ok := shaper.NextGlyph()
		if !ok {
			break
		}
		line, ok = painter.paintGlyph(gtx, shaper, glyph, line)
		if !ok {
			break
		}
	}
	drawing := recording.Stop()

	viewport := painter.viewport
	viewport.Min = viewport.Min.Add(painter.padding.Min)
	viewport.Max = viewport.Max.Add(painter.padding.Max)
	clipped := clip.Rect(viewport).Push(gtx.Ops)
	drawing.Add(gtx.Ops)
	clipped.Pop()

	sizePx := constraints.Constrain(painter.bounds.Size())
	dims := layout.Dimensions{
		Size:     sizePx,
		Baseline: sizePx.Y - painter.baseline,
	}
	return dims, TextInfo{Truncated: painter.truncated}
}

// textIterator turns the shaper's glyph stream into visible lines while
// accumulating the dimensions returned to layout.
type textIterator struct {
	// These fields remain concrete rather than hidden behind another iterator:
	// the glyph loop is hot and its fixed line buffer must stay on the stack.
	viewport image.Rectangle
	maxLines int
	material op.CallOp

	truncated int
	linesSeen int
	lineOff   f32.Point
	padding   image.Rectangle
	bounds    image.Rectangle
	visible   bool
	first     bool
	baseline  int
}

// processGlyph includes a glyph in the measured bounds when it intersects the
// viewport. Its second argument is carried through for callers that already
// know iteration must stop.
func (it *textIterator) processGlyph(glyph text.Glyph, keepGoing bool) bool {
	if !it.withinLineLimit(glyph) {
		return false
	}
	it.includeOverhang(glyph)

	logical := image.Rectangle{
		Min: image.Pt(glyph.X.Floor(), int(glyph.Y)-glyph.Ascent.Ceil()),
		Max: image.Pt((glyph.X + glyph.Advance).Ceil(), int(glyph.Y)+glyph.Descent.Ceil()),
	}
	if !it.first {
		it.first = true
		it.baseline = int(glyph.Y)
		it.bounds = logical
	}

	above := logical.Max.Y < it.viewport.Min.Y
	below := logical.Min.Y > it.viewport.Max.Y
	left := logical.Max.X < it.viewport.Min.X
	right := logical.Min.X > it.viewport.Max.X
	it.visible = !above && !below && !left && !right
	if it.visible {
		it.bounds.Min.X = min(it.bounds.Min.X, logical.Min.X)
		it.bounds.Min.Y = min(it.bounds.Min.Y, logical.Min.Y)
		it.bounds.Max.X = max(it.bounds.Max.X, logical.Max.X)
		it.bounds.Max.Y = max(it.bounds.Max.Y, logical.Max.Y)
	}
	return keepGoing && !below
}

func (it *textIterator) withinLineLimit(glyph text.Glyph) bool {
	if it.maxLines <= 0 {
		return true
	}
	if glyph.Flags&text.FlagTruncator != 0 && glyph.Flags&text.FlagClusterBreak != 0 {
		it.truncated = int(glyph.Runes)
	}
	if glyph.Flags&text.FlagLineBreak != 0 {
		it.linesSeen++
	}
	return it.linesSeen != it.maxLines || glyph.Flags&text.FlagParagraphBreak == 0
}

func (it *textIterator) includeOverhang(glyph text.Glyph) {
	if left := glyph.Bounds.Min.X.Floor(); left < it.padding.Min.X {
		it.padding.Min.X = left
	}
	if right := (glyph.Bounds.Max.X - glyph.Advance).Ceil(); right > it.padding.Max.X {
		it.padding.Max.X = right
	}
	if top := (glyph.Bounds.Min.Y + glyph.Ascent).Floor(); top < it.padding.Min.Y {
		it.padding.Min.Y = top
	}
	if bottom := (glyph.Bounds.Max.Y - glyph.Descent).Ceil(); bottom > it.padding.Max.Y {
		it.padding.Max.Y = bottom
	}
}

func fixedPixels(value fixed.Int26_6) float32 {
	return float32(value) / 64
}

// paintGlyph buffers one visible glyph and flushes at a line boundary, a full
// stack buffer, or the end of the visible viewport.
func (it *textIterator) paintGlyph(gtx layout.Context, shaper *text.Shaper, glyph text.Glyph, line []text.Glyph) ([]text.Glyph, bool) {
	visibleOrBefore := it.processGlyph(glyph, true)
	if it.visible {
		if len(line) == 0 {
			it.lineOff = f32.Point{X: fixedPixels(glyph.X), Y: float32(glyph.Y)}.Sub(layout.FPt(it.viewport.Min))
		}
		line = append(line, glyph)
	}
	flush := glyph.Flags&text.FlagLineBreak != 0 || len(line) == cap(line) || !visibleOrBefore
	if flush {
		line = it.paintLine(gtx, shaper, line)
	}
	return line, visibleOrBefore
}

func (it *textIterator) paintLine(gtx layout.Context, shaper *text.Shaper, line []text.Glyph) []text.Glyph {
	positioned := op.Affine(f32.AffineId().Offset(it.lineOff)).Push(gtx.Ops)
	outline := clip.Outline{Path: shaper.Shape(line)}.Op().Push(gtx.Ops)
	it.material.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	outline.Pop()
	if bitmaps := shaper.Bitmaps(line); bitmaps != (op.CallOp{}) {
		bitmaps.Add(gtx.Ops)
	}
	positioned.Pop()
	return line[:0]
}
