package widget

import (
	"image"
	"io"
	"math"
	"strings"
	"unicode"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// stringSource adapts an immutable string to textView's random-access source.
type stringSource struct {
	reader *strings.Reader
}

var _ textSource = stringSource{}

func newStringSource(value string) stringSource {
	return stringSource{reader: strings.NewReader(value)}
}

func (s stringSource) Changed() bool {
	return false
}

func (s stringSource) Size() int64 {
	return s.reader.Size()
}

func (s stringSource) ReadAt(b []byte, offset int64) (int, error) {
	return s.reader.ReadAt(b, offset)
}

// ReplaceRunes deliberately does nothing because selectable text is immutable.
func (stringSource) ReplaceRunes(int64, int64, string) {
}

// Selectable lays out read-only text that can be focused, selected, and copied.
type Selectable struct {
	// Alignment controls the alignment of the text.
	Alignment text.Alignment
	// MaxLines is the maximum number of lines of text to be displayed.
	MaxLines int
	// Truncator is the symbol to use at the end of the final line of text
	// if text was cut off. Defaults to "…" if left empty.
	Truncator string
	// WrapPolicy configures how displayed text will be broken into lines.
	WrapPolicy text.WrapPolicy
	// LineHeight controls the distance between the baselines of lines of text.
	// If zero, a sensible default will be used.
	LineHeight unit.Sp
	// LineHeightScale applies a scaling factor to the LineHeight. If zero, a
	// sensible default will be used.
	LineHeightScale float32
	initialized     bool
	source          stringSource
	text            textView
	value           string
	scratch         []byte

	focused  bool
	dragging bool
	click    gesture.Click
	drag     gesture.Drag
}

// initialize gives the zero value an empty source on first use.
func (s *Selectable) initialize() {
	if s.initialized {
		return
	}
	s.source = newStringSource("")
	s.text.SetSource(s.source)
	s.initialized = true
}

// Focused reports whether the selectable owns keyboard focus.
func (s *Selectable) Focused() bool {
	return s.focused
}

func (s *Selectable) paintSelection(gtx layout.Context, material op.CallOp) {
	if !s.focused {
		return
	}
	s.text.PaintSelection(gtx, material)
}

func (s *Selectable) paintText(gtx layout.Context, material op.CallOp) {
	s.text.PaintText(gtx, material)
}

// SelectionLen returns the length of the selection in runes.
func (s *Selectable) SelectionLen() int {
	s.initialize()
	return s.text.SelectionLen()
}

// Selection returns the start and end of the selection, as rune offsets.
// start can be > end.
func (s *Selectable) Selection() (start, end int) {
	s.initialize()
	return s.text.Selection()
}

// SetCaret sets both ends of the selection from rune offsets. Offsets outside
// the text are clamped to the nearest valid caret position.
func (s *Selectable) SetCaret(start, end int) {
	s.initialize()
	s.text.SetCaret(start, end)
}

// SelectedText returns the text inside the current selection.
func (s *Selectable) SelectedText() string {
	s.initialize()
	s.scratch = s.text.SelectedText(s.scratch)
	return string(s.scratch)
}

// ClearSelection collapses the selection at its active end.
func (s *Selectable) ClearSelection() {
	s.initialize()
	s.text.ClearSelection()
}

// Text returns the selectable's complete contents.
func (s *Selectable) Text() string {
	s.initialize()
	s.scratch = s.text.Text(s.scratch)
	return string(s.scratch)
}

// SetText replaces the contents. A different value clears the selection;
// setting the current value preserves it.
func (s *Selectable) SetText(value string) {
	s.initialize()
	if s.value == value {
		return
	}
	s.source = newStringSource(value)
	s.value = value
	s.text.SetSource(s.source)
	// The old offsets name a different document. Reset them directly instead
	// of asking textView to clamp: a Selectable may receive text before it has
	// a shaper, so forcing layout here would make SetText depend on Layout.
	s.text.caret.start = 0
	s.text.caret.end = 0
	s.text.caret.xoff = 0
}

// Truncated returns whether the text has been truncated by the text shaper to
// fit within available constraints.
func (s *Selectable) Truncated() bool {
	s.initialize()
	return s.text.Truncated()
}

// Update processes pending input and reports whether the visible selection
// range changed.
func (s *Selectable) Update(gtx layout.Context) bool {
	s.initialize()
	return s.handleEvents(gtx)
}

// Layout shapes the text, registers input handlers, and paints the contents and
// selection within the resulting dimensions.
func (s *Selectable) Layout(gtx layout.Context, shaper *text.Shaper, font font.Font, size unit.Sp, textMaterial, selectionMaterial op.CallOp) layout.Dimensions {
	s.Update(gtx)
	s.configureText()
	s.text.Layout(gtx, shaper, font, size)
	dims := s.text.Dimensions()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorText.Add(gtx.Ops)
	event.Op(gtx.Ops, s)
	s.click.Add(gtx.Ops)
	s.drag.Add(gtx.Ops)
	s.paintSelection(gtx, selectionMaterial)
	s.paintText(gtx, textMaterial)
	return dims
}

func (s *Selectable) configureText() {
	s.text.Alignment = s.Alignment
	s.text.MaxLines = s.MaxLines
	s.text.Truncator = s.Truncator
	s.text.WrapPolicy = s.WrapPolicy
	s.text.LineHeight = s.LineHeight
	s.text.LineHeightScale = s.LineHeightScale
}

type selectionState struct {
	start  int
	length int
}

func (s *Selectable) selectionState() selectionState {
	start, end := s.text.Selection()
	return selectionState{start: min(start, end), length: abs(start - end)}
}

func (s *Selectable) handleEvents(gtx layout.Context) bool {
	before := s.selectionState()
	s.processPointer(gtx)
	s.processKey(gtx)
	return before != s.selectionState()
}

func pointerPoint(position f32.Point) image.Point {
	return image.Pt(
		int(math.Round(float64(position.X))),
		int(math.Round(float64(position.Y))),
	)
}

func (s *Selectable) processPointer(gtx layout.Context) {
	for _, evt := range s.pointerEvents(gtx) {
		switch evt := evt.(type) {
		case gesture.ClickEvent:
			if evt.Kind == gesture.KindPress && evt.Source == pointer.Mouse ||
				evt.Kind == gesture.KindClick && evt.Source != pointer.Mouse {
				s.handleClick(gtx, evt)
			}
		case pointer.Event:
			s.handleDrag(evt)
		}
	}
}

func (s *Selectable) handleClick(gtx layout.Context, evt gesture.ClickEvent) {
	previousCaret, _ := s.text.Selection()
	s.text.MoveCoord(evt.Position)
	gtx.Execute(key.FocusCmd{Tag: s})

	if evt.Modifiers == key.ModShift {
		start, end := s.text.Selection()
		if abs(end-start) < abs(start-previousCaret) {
			s.text.SetCaret(start, previousCaret)
		}
	} else {
		s.text.ClearSelection()
	}

	s.dragging = evt.NumClicks < 2
	switch {
	case evt.NumClicks == 2:
		s.selectToken()
	case evt.NumClicks >= 3:
		s.text.MoveLineStart(selectionClear)
		s.text.MoveLineEnd(selectionExtend)
	}
}

func (s *Selectable) handleDrag(evt pointer.Event) {
	if evt.Source != pointer.Mouse || !s.dragging {
		return
	}
	if evt.Kind != pointer.Drag && evt.Kind != pointer.Release {
		return
	}
	s.text.MoveCoord(pointerPoint(evt.Position))
	if evt.Kind == pointer.Release {
		s.dragging = false
	}
}

type tokenKind uint8

const (
	spaceToken tokenKind = iota
	wordToken
	punctuationToken
)

func classifyTokenRune(r rune) tokenKind {
	switch {
	case unicode.IsSpace(r):
		return spaceToken
	case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsMark(r), r == '_':
		return wordToken
	default:
		return punctuationToken
	}
}

// selectToken selects the lexical unit under the caret. textView's word
// movement is intentionally whitespace based for keyboard navigation; pointer
// selection has a narrower contract and must leave adjacent punctuation out.
func (s *Selectable) selectToken() {
	runes := []rune(s.value)
	if len(runes) == 0 {
		s.text.SetCaret(0, 0)
		return
	}
	start, _ := s.text.Selection()
	start = min(max(start, 0), len(runes)-1)
	kind := classifyTokenRune(runes[start])
	end := start + 1
	for start > 0 && classifyTokenRune(runes[start-1]) == kind {
		start--
	}
	for end < len(runes) && classifyTokenRune(runes[end]) == kind {
		end++
	}
	s.text.SetCaret(start, end)
}

func (s *Selectable) pointerEvents(gtx layout.Context) []event.Event {
	var events []event.Event
	for {
		evt, ok := s.click.Update(gtx.Source)
		if !ok {
			break
		}
		events = append(events, evt)
	}
	for {
		evt, ok := s.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		events = append(events, evt)
	}
	return events
}

func (s *Selectable) processKey(gtx layout.Context) {
	for {
		ke, ok := gtx.Event(
			key.FocusFilter{Target: s},
			key.Filter{Focus: s, Name: key.NameLeftArrow, Optional: key.ModShortcutAlt | key.ModShift},
			key.Filter{Focus: s, Name: key.NameRightArrow, Optional: key.ModShortcutAlt | key.ModShift},
			key.Filter{Focus: s, Name: key.NameUpArrow, Optional: key.ModShortcutAlt | key.ModShift},
			key.Filter{Focus: s, Name: key.NameDownArrow, Optional: key.ModShortcutAlt | key.ModShift},

			key.Filter{Focus: s, Name: key.NamePageUp, Optional: key.ModShift},
			key.Filter{Focus: s, Name: key.NamePageDown, Optional: key.ModShift},
			key.Filter{Focus: s, Name: key.NameEnd, Optional: key.ModShift},
			key.Filter{Focus: s, Name: key.NameHome, Optional: key.ModShift},

			key.Filter{Focus: s, Name: "C", Required: key.ModShortcut},
			key.Filter{Focus: s, Name: "X", Required: key.ModShortcut},
			key.Filter{Focus: s, Name: "A", Required: key.ModShortcut},
		)
		if !ok {
			break
		}
		switch ke := ke.(type) {
		case key.FocusEvent:
			s.focused = ke.Focus
		case key.Event:
			if s.focused && ke.State == key.Press {
				s.command(gtx, ke)
			}
		}
	}
}

func (s *Selectable) command(gtx layout.Context, k key.Event) {
	direction := 1
	if gtx.Locale.Direction.Progression() == system.TowardOrigin {
		direction = -1
	}
	moveByWord := k.Modifiers.Contain(key.ModShortcutAlt)
	selAct := selectionClear
	if k.Modifiers.Contain(key.ModShift) {
		selAct = selectionExtend
	}
	if k.Modifiers == key.ModShortcut {
		switch k.Name {
		case "C", "X":
			s.copySelection(gtx)
		case "A":
			s.text.SetCaret(0, s.text.Len())
		}
		return
	}
	switch k.Name {
	case key.NameUpArrow:
		s.text.MoveLines(-1, selAct)
	case key.NameDownArrow:
		s.text.MoveLines(+1, selAct)
	case key.NameLeftArrow:
		if moveByWord {
			s.text.MoveWord(-direction, selAct)
		} else {
			s.moveCaret(-direction, selAct)
		}
	case key.NameRightArrow:
		if moveByWord {
			s.text.MoveWord(direction, selAct)
		} else {
			s.moveCaret(direction, selAct)
		}
	case key.NamePageUp:
		s.text.MovePages(-1, selAct)
	case key.NamePageDown:
		s.text.MovePages(+1, selAct)
	case key.NameHome:
		s.text.MoveLineStart(selAct)
	case key.NameEnd:
		s.text.MoveLineEnd(selAct)
	}
}

func (s *Selectable) copySelection(gtx layout.Context) {
	if s.text.SelectionLen() == 0 {
		return
	}
	s.scratch = s.text.SelectedText(s.scratch)
	selected := string(s.scratch)
	gtx.Execute(clipboard.WriteCmd{
		Type: "application/text",
		Data: io.NopCloser(strings.NewReader(selected)),
	})
}

func (s *Selectable) moveCaret(distance int, action selectionAction) {
	if action == selectionClear {
		s.text.ClearSelection()
	}
	s.text.MoveCaret(distance, distance*int(action))
}

// Regions returns visible regions covering the rune range [start,end).
func (s *Selectable) Regions(start, end int, regions []Region) []Region {
	s.initialize()
	if start == end {
		return regions[:0]
	}
	return s.text.Regions(start, end, regions)
}
