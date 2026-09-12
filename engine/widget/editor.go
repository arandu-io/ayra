package widget

import (
	"bufio"
	"image"
	"io"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/io/transfer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Editor is the persistent state of an editable, scrollable text area.
type Editor struct {
	// text owns shaping, cursor placement and the view onto the buffer.
	text textView

	// Alignment places text within the available width.
	Alignment text.Alignment
	// LineHeight is the distance between baselines. Zero uses the shaper's
	// default.
	LineHeight unit.Sp
	// LineHeightScale multiplies LineHeight. Zero uses the default scale.
	LineHeightScale float32
	// SingleLine replaces incoming line breaks with spaces and scrolls
	// horizontally.
	SingleLine bool
	// ReadOnly permits navigation and copying but refuses user edits.
	ReadOnly bool
	// Submit turns an unmodified return key into a [SubmitEvent].
	Submit bool
	// Mask draws every non-newline rune as this rune while retaining the
	// original text.
	Mask rune
	// InputHint selects the on-screen keyboard requested while focused.
	InputHint key.InputHint
	// MaxLen limits content in runes. Zero means unlimited.
	MaxLen int
	// Filter contains every rune accepted as input. Empty accepts all runes.
	Filter string
	// WrapPolicy controls line breaking.
	WrapPolicy text.WrapPolicy

	buffer     *editBuffer
	scratch    []byte
	blinkStart time.Time

	ime struct {
		imeState
		scratch    []byte
		historyIdx int
	}

	dragging    bool
	dragger     gesture.Drag
	scroller    gesture.Scroll
	scrollCaret bool
	showCaret   bool

	clicker gesture.Click

	history []modification
	// nextHistoryIdx points after the last applied modification. Undo moves it
	// left; redo moves it right.
	nextHistoryIdx int

	pending []EditorEvent
}

type offEntry struct {
	runes int
	bytes int
}

type imeState struct {
	selection struct {
		rng               key.Range
		caret             key.Caret
		compositionBounds image.Rectangle
	}
	snippet     key.Snippet
	composition key.Range
	lastCompose key.Range
	textVersion uint64
	scrollOff   image.Point
	start, end  int
}

type maskReader struct {
	rr      io.RuneReader
	maskBuf [utf8.UTFMax]byte
	mask    []byte
	// overflow is the part of an encoded mask that did not fit in the previous
	// destination.
	overflow []byte
}

type selectionAction int

const (
	selectionExtend selectionAction = iota
	selectionClear
)

func (m *maskReader) Reset(source io.Reader, mask rune) {
	m.rr = bufio.NewReader(source)
	m.overflow = nil
	n := utf8.EncodeRune(m.maskBuf[:], mask)
	m.mask = m.maskBuf[:n]
}

// Read preserves newlines and substitutes the configured mask for other runes.
func (m *maskReader) Read(dst []byte) (written int, err error) {
	for len(dst) > 0 {
		replacement := m.overflow
		if len(m.overflow) > 0 {
			m.overflow = nil
		} else {
			var current rune
			current, _, err = m.rr.ReadRune()
			if err != nil {
				break
			}
			if current == '\n' {
				replacement = []byte{'\n'}
			} else {
				replacement = m.mask
			}
		}
		n := copy(dst, replacement)
		m.overflow = replacement[n:]
		dst = dst[n:]
		written += n
	}
	return written, err
}

// EditorEvent is an event emitted by an [Editor].
type EditorEvent interface {
	isEditorEvent()
}

// ChangeEvent reports a user-initiated text change.
type ChangeEvent struct{}

// SubmitEvent reports an unmodified return while [Editor.Submit] is enabled.
type SubmitEvent struct {
	Text string
}

// SelectEvent reports that either end of the selection moved.
type SelectEvent struct{}

const (
	blinksPerSecond  = 1
	maxBlinkDuration = 10 * time.Second
)

func (e *Editor) processEvents(gtx layout.Context) (EditorEvent, bool) {
	if pending, ok := e.popPending(); ok {
		return pending, true
	}

	beforeStart, beforeEnd := e.Selection()
	reported, ok := e.processPointer(gtx)
	if !ok {
		reported, ok = e.processKey(gtx)
	}

	afterStart, afterEnd := e.Selection()
	selectionMoved := beforeStart != afterStart || beforeEnd != afterEnd
	if !selectionMoved {
		return reported, ok
	}
	if ok {
		e.pending = append(e.pending, SelectEvent{})
		return reported, true
	}
	return SelectEvent{}, true
}

func (e *Editor) popPending() (EditorEvent, bool) {
	if len(e.pending) == 0 {
		return nil, false
	}
	first := e.pending[0]
	copy(e.pending, e.pending[1:])
	e.pending = e.pending[:len(e.pending)-1]
	return first, true
}

func (e *Editor) processPointer(gtx layout.Context) (EditorEvent, bool) {
	bounds := e.text.ScrollBounds()
	axis := gesture.Vertical
	minimum, maximum := bounds.Min.Y, bounds.Max.Y
	if e.SingleLine {
		axis = gesture.Horizontal
		minimum, maximum = bounds.Min.X, bounds.Max.X
	} else {
		axis = gesture.Vertical
	}

	var horizontal, vertical pointer.ScrollRange
	full := e.text.FullDimensions()
	visible := e.text.Dimensions()
	if e.SingleLine {
		offset := e.text.ScrollOff().X
		horizontal.Min = min(-offset, 0)
		horizontal.Max = max(0, full.Size.X-offset-visible.Size.X)
	} else {
		offset := e.text.ScrollOff().Y
		vertical.Min = -offset
		vertical.Max = max(0, full.Size.Y-offset-visible.Size.Y)
	}

	distance := e.scroller.Update(gtx.Metric, gtx.Source, gtx.Now, axis, horizontal, vertical)
	current := 0
	if e.SingleLine {
		e.text.ScrollRel(distance, 0)
		current = e.text.ScrollOff().X
	} else {
		e.text.ScrollRel(0, distance)
		current = e.text.ScrollOff().Y
	}

	for {
		clicked, ok := e.clicker.Update(gtx.Source)
		if !ok {
			break
		}
		ev, ok := e.processPointerEvent(gtx, clicked)
		if ok {
			return ev, ok
		}
	}
	for {
		dragged, ok := e.dragger.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		ev, ok := e.processPointerEvent(gtx, dragged)
		if ok {
			return ev, ok
		}
	}

	if distance > 0 && current >= maximum || distance < 0 && current <= minimum {
		e.scroller.Stop()
	}
	return nil, false
}

func (e *Editor) processPointerEvent(gtx layout.Context, incoming event.Event) (EditorEvent, bool) {
	switch incoming := incoming.(type) {
	case gesture.ClickEvent:
		switch {
		case incoming.Kind == gesture.KindPress && incoming.Source == pointer.Mouse,
			incoming.Kind == gesture.KindClick && incoming.Source != pointer.Mouse:
			prevCaretPos, _ := e.text.Selection()
			e.blinkStart = gtx.Now
			e.text.MoveCoord(image.Point{
				X: int(math.Round(float64(incoming.Position.X))),
				Y: int(math.Round(float64(incoming.Position.Y))),
			})
			gtx.Execute(key.FocusCmd{Tag: e})
			if !e.ReadOnly {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
			if e.scroller.State() != gesture.StateFlinging {
				e.scrollCaret = true
			}

			if incoming.Modifiers == key.ModShift {
				start, end := e.text.Selection()
				if abs(end-start) < abs(start-prevCaretPos) {
					e.text.SetCaret(start, prevCaretPos)
				}
			} else {
				e.text.ClearSelection()
			}
			e.dragging = true

			switch {
			case incoming.NumClicks == 2:
				e.text.MoveWord(-1, selectionClear)
				e.text.MoveWord(1, selectionExtend)
				e.dragging = false
			case incoming.NumClicks >= 3:
				e.text.MoveLineStart(selectionClear)
				e.text.MoveLineEnd(selectionExtend)
				e.dragging = false
			}
		}
	case pointer.Event:
		release := false
		switch {
		case incoming.Kind == pointer.Release && incoming.Source == pointer.Mouse:
			release = true
			fallthrough
		case incoming.Kind == pointer.Drag && incoming.Source == pointer.Mouse:
			if e.dragging {
				e.blinkStart = gtx.Now
				e.text.MoveCoord(image.Point{
					X: int(math.Round(float64(incoming.Position.X))),
					Y: int(math.Round(float64(incoming.Position.Y))),
				})
				e.scrollCaret = true

				if release {
					e.dragging = false
				}
			}
		}
	}
	return nil, false
}

func optionalKeyFilter(enabled bool, filter key.Filter) event.Filter {
	if !enabled {
		return nil
	}
	return filter
}

func (e *Editor) processKey(gtx layout.Context) (EditorEvent, bool) {
	if e.text.Changed() {
		return ChangeEvent{}, true
	}
	atBeginning, atEnd := e.keyFilterBounds(gtx)
	filters := [...]event.Filter{
		key.FocusFilter{Target: e},
		transfer.TargetFilter{Target: e, Type: "application/text"},
		key.Filter{Focus: e, Name: key.NameEnter, Optional: key.ModShift},
		key.Filter{Focus: e, Name: key.NameReturn, Optional: key.ModShift},

		key.Filter{Focus: e, Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
		key.Filter{Focus: e, Name: "C", Required: key.ModShortcut},
		key.Filter{Focus: e, Name: "V", Required: key.ModShortcut},
		key.Filter{Focus: e, Name: "X", Required: key.ModShortcut},
		key.Filter{Focus: e, Name: "A", Required: key.ModShortcut},

		key.Filter{Focus: e, Name: key.NameDeleteBackward, Optional: key.ModShortcutAlt | key.ModShift},
		key.Filter{Focus: e, Name: key.NameDeleteForward, Optional: key.ModShortcutAlt | key.ModShift},

		key.Filter{Focus: e, Name: key.NameHome, Optional: key.ModShortcut | key.ModShift},
		key.Filter{Focus: e, Name: key.NameEnd, Optional: key.ModShortcut | key.ModShift},
		key.Filter{Focus: e, Name: key.NamePageDown, Optional: key.ModShift},
		key.Filter{Focus: e, Name: key.NamePageUp, Optional: key.ModShift},
		optionalKeyFilter(!atBeginning, key.Filter{Focus: e, Name: key.NameLeftArrow, Optional: key.ModShortcutAlt | key.ModShift}),
		optionalKeyFilter(!atBeginning, key.Filter{Focus: e, Name: key.NameUpArrow, Optional: key.ModShortcutAlt | key.ModShift}),
		optionalKeyFilter(!atEnd, key.Filter{Focus: e, Name: key.NameRightArrow, Optional: key.ModShortcutAlt | key.ModShift}),
		optionalKeyFilter(!atEnd, key.Filter{Focus: e, Name: key.NameDownArrow, Optional: key.ModShortcutAlt | key.ModShift}),
	}
	// selectionAdjustment accounts for runes refused by MaxLen or Filter
	// before a following input-method selection is applied.
	selectionAdjustment := 0
	for {
		incoming, ok := gtx.Event(filters[:]...)
		if !ok {
			break
		}
		e.blinkStart = gtx.Now

		switch incoming := incoming.(type) {
		case key.FocusEvent:
			e.resetInputMethod()
			if incoming.Focus && !e.ReadOnly {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
		case key.CompositionEvent:
			e.ime.composition = key.Range(incoming)
		case key.Event:
			if !gtx.Focused(e) || incoming.State != key.Press {
				continue
			}
			if event, ok := e.submitFromKey(incoming); ok {
				return event, true
			}
			e.scrollCaret = true
			e.scroller.Stop()
			if event, ok := e.command(gtx, incoming); ok {
				return event, true
			}
		case key.SnippetEvent:
			e.updateSnippet(gtx, incoming.Start, incoming.End)
		case key.EditEvent:
			if e.ReadOnly {
				continue
			}
			moves, submitted := e.applyInputMethodEdit(incoming)
			selectionAdjustment += utf8.RuneCountInString(incoming.Text) - moves
			if submitted {
				event := e.submitEvent()
				if e.text.Changed() {
					e.pending = append(e.pending, event)
					return ChangeEvent{}, true
				}
				return event, true
			}
		case transfer.DataEvent:
			e.scrollCaret = true
			e.scroller.Stop()
			content, err := io.ReadAll(incoming.Open())
			if err == nil && e.Insert(string(content)) != 0 {
				return ChangeEvent{}, true
			}
		case key.SelectionEvent:
			e.scrollCaret = true
			e.scroller.Stop()
			incoming.Start -= selectionAdjustment
			incoming.End -= selectionAdjustment
			selectionAdjustment = 0
			e.text.SetCaret(incoming.Start, incoming.End)
			if e.ime.historyIdx == e.nextHistoryIdx {
				e.rememberLastSelection()
			}
			e.ime.historyIdx = 0
		}
	}
	if e.text.Changed() {
		return ChangeEvent{}, true
	}
	return nil, false
}

func (e *Editor) keyFilterBounds(gtx layout.Context) (atBeginning, atEnd bool) {
	caret, _ := e.text.Selection()
	atBeginning = caret == 0
	atEnd = caret == e.text.Len()
	if gtx.Locale.Direction.Progression() != system.FromOrigin {
		atEnd, atBeginning = atBeginning, atEnd
	}
	return atBeginning, atEnd
}

func (e *Editor) resetInputMethod() {
	e.ime.imeState = imeState{}
	e.ime.composition = key.Range{Start: -1, End: -1}
	e.ime.historyIdx = 0
}

func (e *Editor) submitFromKey(incoming key.Event) (EditorEvent, bool) {
	if e.ReadOnly || !e.Submit || incoming.Modifiers.Contain(key.ModShift) {
		return nil, false
	}
	if incoming.Name != key.NameReturn && incoming.Name != key.NameEnter {
		return nil, false
	}
	return e.submitEvent(), true
}

func (e *Editor) submitEvent() SubmitEvent {
	e.scratch = e.text.Text(e.scratch)
	return SubmitEvent{Text: string(e.scratch)}
}

func (e *Editor) applyInputMethodEdit(incoming key.EditEvent) (moves int, submit bool) {
	e.scrollCaret = true
	e.scroller.Stop()

	content := incoming.Text
	switch {
	case e.Submit:
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			submit = true
			moves += len(content) - newline
			content = content[:newline]
		}
	case e.SingleLine:
		content = strings.ReplaceAll(content, "\n", " ")
	}
	moves += e.replace(incoming.Range.Start, incoming.Range.End, content, true)
	e.ime.historyIdx = e.nextHistoryIdx
	e.text.MoveCaret(0, 0)
	return moves, submit
}

func (e *Editor) command(gtx layout.Context, pressed key.Event) (EditorEvent, bool) {
	progression := 1
	if gtx.Locale.Direction.Progression() == system.TowardOrigin {
		progression = -1
	}
	selection := selectionClear
	if pressed.Modifiers.Contain(key.ModShift) {
		selection = selectionExtend
	}
	if pressed.Modifiers.Contain(key.ModShortcut) {
		return e.shortcut(gtx, pressed, selection)
	}
	return e.directCommand(pressed, progression, selection)
}

func (e *Editor) shortcut(gtx layout.Context, pressed key.Event, selection selectionAction) (EditorEvent, bool) {
	switch pressed.Name {
	case "V":
		if !e.ReadOnly {
			gtx.Execute(clipboard.ReadCmd{Tag: e})
		}
	case "C", "X":
		e.scratch = e.text.SelectedText(e.scratch)
		selected := string(e.scratch)
		if selected == "" {
			return nil, false
		}
		gtx.Execute(clipboard.WriteCmd{
			Type: "application/text",
			Data: io.NopCloser(strings.NewReader(selected)),
		})
		if pressed.Name == "X" && !e.ReadOnly && e.Delete(1) != 0 {
			return ChangeEvent{}, true
		}
	case "A":
		e.text.SetCaret(0, e.text.Len())
	case "Z":
		if e.ReadOnly {
			return nil, false
		}
		if pressed.Modifiers.Contain(key.ModShift) {
			return e.redo()
		}
		return e.undo()
	case key.NameHome:
		e.text.MoveTextStart(selection)
	case key.NameEnd:
		e.text.MoveTextEnd(selection)
	}
	return nil, false
}

func (e *Editor) directCommand(pressed key.Event, progression int, selection selectionAction) (EditorEvent, bool) {
	moveByWord := pressed.Modifiers.Contain(key.ModShortcutAlt)
	changed := false
	switch pressed.Name {
	case key.NameReturn, key.NameEnter:
		if !e.ReadOnly && e.Insert("\n") != 0 {
			changed = true
		}
	case key.NameDeleteBackward:
		if !e.ReadOnly {
			if moveByWord {
				changed = e.deleteWord(-1) != 0
			} else {
				changed = e.Delete(-1) != 0
			}
		}
	case key.NameDeleteForward:
		if !e.ReadOnly {
			if moveByWord {
				changed = e.deleteWord(1) != 0
			} else {
				changed = e.Delete(1) != 0
			}
		}
	case key.NameUpArrow:
		e.text.MoveLines(-1, selection)
	case key.NameDownArrow:
		e.text.MoveLines(1, selection)
	case key.NameLeftArrow:
		if moveByWord {
			e.text.MoveWord(-progression, selection)
		} else {
			if selection == selectionClear {
				e.text.ClearSelection()
			}
			e.text.MoveCaret(-progression, -progression*int(selection))
		}
	case key.NameRightArrow:
		if moveByWord {
			e.text.MoveWord(progression, selection)
		} else {
			if selection == selectionClear {
				e.text.ClearSelection()
			}
			e.text.MoveCaret(progression, int(selection)*progression)
		}
	case key.NamePageUp:
		e.text.MovePages(-1, selection)
	case key.NamePageDown:
		e.text.MovePages(1, selection)
	case key.NameHome:
		e.text.MoveLineStart(selection)
	case key.NameEnd:
		e.text.MoveLineEnd(selection)
	}
	if changed {
		return ChangeEvent{}, true
	}
	return nil, false
}

// initBuffer makes the zero Editor usable and projects its public options onto
// the text view.
func (e *Editor) initBuffer() {
	if e.buffer == nil {
		e.buffer = new(editBuffer)
		e.text.SetSource(e.buffer)
	}
	e.text.Alignment = e.Alignment
	e.text.LineHeight = e.LineHeight
	e.text.LineHeightScale = e.LineHeightScale
	e.text.SingleLine = e.SingleLine
	e.text.Mask = e.Mask
	e.text.WrapPolicy = e.WrapPolicy
	e.text.DisableSpaceTrim = true
}

// Update consumes input until it emits one editor event or has nothing left.
// Callers drain it by calling Update until ok is false.
func (e *Editor) Update(gtx layout.Context) (EditorEvent, bool) {
	e.initBuffer()
	reported, ok := e.processEvents(gtx)
	e.updateIMEState(gtx)
	e.updateSnippet(gtx, e.ime.start, e.ime.end)
	return reported, ok
}

func (e *Editor) updateIMEState(gtx layout.Context) {
	start, end := e.text.Selection()
	selection := key.Range{Start: start, End: end}
	scrollOff := e.text.ScrollOff()
	if selection == e.ime.selection.rng &&
		e.ime.composition == e.ime.lastCompose &&
		e.text.version == e.ime.textVersion &&
		scrollOff == e.ime.scrollOff {
		return
	}
	e.ime.lastCompose = e.ime.composition
	e.ime.textVersion = e.text.version
	e.ime.scrollOff = scrollOff

	next := e.ime.selection
	next.rng = selection
	position, ascent, descent := e.text.CaretInfo()
	next.caret = key.Caret{
		Pos:     layout.FPt(position),
		Ascent:  float32(ascent),
		Descent: float32(descent),
	}
	next.compositionBounds = e.compositionBounds()
	if next != e.ime.selection {
		e.ime.selection = next
		gtx.Execute(key.SelectionCmd{
			Tag:               e,
			Range:             next.rng,
			Caret:             next.caret,
			CompositionBounds: next.compositionBounds,
		})
	}
}

// Layout shapes and paints the editor with separate text and selection
// materials.
func (e *Editor) Layout(gtx layout.Context, shaper *text.Shaper, face font.Font, size unit.Sp, textMaterial, selectMaterial op.CallOp) layout.Dimensions {
	for {
		_, ok := e.Update(gtx)
		if !ok {
			break
		}
	}

	e.text.Layout(gtx, shaper, face, size)
	return e.layout(gtx, textMaterial, selectMaterial)
}

// updateSnippet publishes the requested rune range when its range or contents
// changed.
func (e *Editor) updateSnippet(gtx layout.Context, start, end int) {
	start, end = boundedRuneRange(start, end, e.text.Len())
	e.ime.start = start
	e.ime.end = end
	startOff := e.text.ByteOffset(start)
	endOff := e.text.ByteOffset(end)
	byteCount := endOff - startOff
	if byteCount > int64(len(e.ime.scratch)) {
		e.ime.scratch = make([]byte, byteCount)
	}
	scratch := e.ime.scratch[:byteCount]
	read, _ := e.text.ReadAt(scratch, startOff)
	if read != len(scratch) {
		panic("editor snippet read was truncated")
	}
	next := key.Snippet{
		Range: key.Range{Start: start, End: end},
		Text:  e.ime.snippet.Text,
	}
	if content := string(scratch); content != next.Text {
		next.Text = content
	}
	if next == e.ime.snippet {
		return
	}
	e.ime.snippet = next
	gtx.Execute(key.SnippetCmd{Tag: e, Snippet: next})
}

func (e *Editor) layout(gtx layout.Context, textMaterial, selectMaterial op.CallOp) layout.Dimensions {
	// Adjust scrolling for new viewport and layout.
	e.text.ScrollRel(0, 0)

	if e.scrollCaret {
		e.scrollCaret = false
		e.text.ScrollToCaret()
	}
	e.updateIMEState(gtx)
	visibleDims := e.text.Dimensions()

	defer clip.Rect(image.Rectangle{Max: visibleDims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorText.Add(gtx.Ops)
	event.Op(gtx.Ops, e)
	key.InputHintOp{Tag: e, Hint: e.InputHint}.Add(gtx.Ops)

	e.scroller.Add(gtx.Ops)

	e.clicker.Add(gtx.Ops)
	e.dragger.Add(gtx.Ops)
	e.showCaret = false
	if gtx.Focused(e) {
		now := gtx.Now
		dt := now.Sub(e.blinkStart)
		blinking := dt < maxBlinkDuration
		const timePerBlink = time.Second / blinksPerSecond
		nextBlink := now.Add(timePerBlink/2 - dt%(timePerBlink/2))
		if blinking {
			gtx.Execute(op.InvalidateCmd{At: nextBlink})
		}
		e.showCaret = !blinking || dt%timePerBlink < timePerBlink/2
	}
	semantic.Editor.Add(gtx.Ops)
	if e.Len() > 0 {
		e.paintSelection(gtx, selectMaterial)
		e.paintText(gtx, textMaterial)
		e.paintComposition(gtx, textMaterial)
	}
	if gtx.Enabled() {
		e.paintCaret(gtx, textMaterial)
	}
	return visibleDims
}

// paintSelection paints the contrasting background for selected text using the provided
// material to set the painting material for the selection.
func (e *Editor) paintSelection(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	if !gtx.Focused(e) {
		return
	}
	e.text.PaintSelection(gtx, material)
}

// paintText paints the text glyphs using the provided material to set the fill of the
// glyphs.
func (e *Editor) paintText(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	e.text.PaintText(gtx, material)
}

func (e *Editor) paintComposition(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	r := e.ime.composition
	if r.Start == -1 || r.Start == r.End {
		return
	}
	e.text.regions = e.text.Regions(r.Start, r.End, e.text.regions)
	thickness := max(gtx.Dp(unit.Dp(1)), 1)
	for _, region := range e.text.regions {
		y := region.Bounds.Max.Y - max(region.Baseline/3, thickness)
		underline := image.Rect(region.Bounds.Min.X, y, region.Bounds.Max.X, y+thickness)
		underline = underline.Intersect(image.Rectangle{Max: e.text.viewSize})
		if underline.Empty() {
			continue
		}
		stack := clip.Rect(underline).Push(gtx.Ops)
		material.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		stack.Pop()
	}
}

// compositionBounds returns the part of the composing text visible in the editor.
func (e *Editor) compositionBounds() image.Rectangle {
	r := e.ime.composition
	if r.Start == -1 || r.Start == r.End {
		return image.Rectangle{}
	}
	e.text.regions = e.text.Regions(r.Start, r.End, e.text.regions)
	visible := image.Rectangle{Max: e.text.viewSize}
	var bounds image.Rectangle
	for _, region := range e.text.regions {
		r := region.Bounds.Intersect(visible)
		if r.Empty() {
			continue
		}
		if bounds.Empty() {
			bounds = r
		} else {
			bounds = bounds.Union(r)
		}
	}
	return bounds
}

// paintCaret paints the text glyphs using the provided material to set the fill material
// of the caret rectangle.
func (e *Editor) paintCaret(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	if !e.showCaret || e.ReadOnly {
		return
	}
	e.text.PaintCaret(gtx, material)
}

// Len is the length of the editor contents, in runes.
func (e *Editor) Len() int {
	e.initBuffer()
	return e.text.Len()
}

// Text returns the contents of the editor.
func (e *Editor) Text() string {
	e.initBuffer()
	e.scratch = e.text.Text(e.scratch)
	return string(e.scratch)
}

// SetText replaces the contents and moves both ends of the selection to zero.
func (e *Editor) SetText(s string) {
	e.initBuffer()
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	e.replace(0, e.text.Len(), s, true)
	// Reset xoff and move the caret to the beginning.
	e.SetCaret(0, 0)
	e.rememberLastSelection()
}

// CaretPos returns the zero-based line and column of the caret.
func (e *Editor) CaretPos() (line, col int) {
	e.initBuffer()
	return e.text.CaretPos()
}

// CaretCoords returns the caret position in editor-local coordinates.
func (e *Editor) CaretCoords() f32.Point {
	e.initBuffer()
	return e.text.CaretCoords()
}

// Delete removes grapheme clusters beside the caret. Positive values delete
// forward and negative values delete backward.
//
// A non-empty selection is removed first and consumes one requested cluster.
// The returned rune delta can be negative when the selection runs backwards.
func (e *Editor) Delete(graphemeClusters int) (deletedRunes int) {
	e.initBuffer()
	if graphemeClusters == 0 {
		return
	}

	start, end := e.text.Selection()
	if start != end {
		graphemeClusters -= sign(graphemeClusters)
	}
	e.text.MoveCaret(0, graphemeClusters)
	start, end = e.text.Selection()
	e.replace(start, end, "", true)
	e.text.MoveCaret(0, 0)
	e.ClearSelection()
	e.rememberLastSelection()
	return end - start
}

// Insert replaces the selection with s and returns the runes accepted.
func (e *Editor) Insert(s string) (insertedRunes int) {
	e.initBuffer()
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	start, end := e.text.Selection()
	insertedRunes = e.replace(start, end, s, true)
	if end < start {
		start = end
	}
	e.text.MoveCaret(0, 0)
	e.SetCaret(start+insertedRunes, start+insertedRunes)
	e.rememberLastSelection()
	e.scrollCaret = true
	return insertedRunes
}

// modification contains both directions of one replacement and the selection
// produced when it was first applied.
type modification struct {
	startRune int
	forward   string
	reverse   string
	selection key.Range
}

func (e *Editor) undo() (EditorEvent, bool) {
	e.initBuffer()
	if e.nextHistoryIdx == 0 {
		return nil, false
	}
	change := e.history[e.nextHistoryIdx-1]
	end := change.startRune + utf8.RuneCountInString(change.forward)
	e.replace(change.startRune, end, change.reverse, false)
	restoredEnd := change.startRune + utf8.RuneCountInString(change.reverse)
	e.SetCaret(restoredEnd, change.startRune)
	e.nextHistoryIdx--
	return ChangeEvent{}, true
}

func (e *Editor) redo() (EditorEvent, bool) {
	e.initBuffer()
	if e.nextHistoryIdx >= len(e.history) {
		return nil, false
	}
	change := e.history[e.nextHistoryIdx]
	end := change.startRune + utf8.RuneCountInString(change.reverse)
	e.replace(change.startRune, end, change.forward, false)
	e.SetCaret(change.selection.Start, change.selection.End)
	e.nextHistoryIdx++
	return ChangeEvent{}, true
}

// replace changes a rune range without assuming it is the current selection.
func (e *Editor) replace(start, end int, content string, addHistory bool) int {
	length := e.text.Len()
	start, end = boundedRuneRange(start, end, length)
	replaced := end - start
	content, accepted := e.acceptedText(content, length-replaced)

	if addHistory {
		removed := e.runesInRange(start, end)
		if e.nextHistoryIdx < len(e.history) {
			e.history = e.history[:e.nextHistoryIdx]
		}
		e.history = append(e.history, modification{
			startRune: start,
			forward:   content,
			reverse:   removed,
			selection: key.Range{Start: start + accepted, End: start + accepted},
		})
		e.nextHistoryIdx++
	}

	accepted = e.text.Replace(start, end, content)
	newEnd := start + accepted
	adjust := func(pos int) int {
		switch {
		case newEnd < pos && pos <= end:
			return newEnd
		case end < pos:
			return pos + newEnd - end
		}
		return pos
	}
	e.ime.start = adjust(e.ime.start)
	e.ime.end = adjust(e.ime.end)
	return accepted
}

func (e *Editor) acceptedText(content string, currentLength int) (string, int) {
	var accepted strings.Builder
	count := 0
	for _, r := range content {
		if e.MaxLen > 0 && currentLength+count >= e.MaxLen {
			break
		}
		if e.Filter != "" && !strings.ContainsRune(e.Filter, r) {
			continue
		}
		accepted.WriteRune(r)
		count++
	}
	return accepted.String(), count
}

func (e *Editor) runesInRange(start, end int) string {
	var removed strings.Builder
	byteOffset := e.text.ByteOffset(start)
	for runeIndex := start; runeIndex < end; runeIndex++ {
		r, size, _ := e.text.ReadRuneAt(byteOffset)
		removed.WriteRune(r)
		byteOffset += int64(size)
	}
	return removed.String()
}

func (e *Editor) rememberLastSelection() {
	if e.nextHistoryIdx == 0 || e.nextHistoryIdx > len(e.history) {
		return
	}
	start, end := e.Selection()
	e.history[e.nextHistoryIdx-1].selection = key.Range{Start: start, End: end}
}

func boundedRuneRange(start, end, length int) (int, int) {
	if start > end {
		start, end = end, start
	}
	start = min(max(start, 0), length)
	end = min(max(end, 0), length)
	return start, end
}

// MoveCaret moves the caret (aka selection start) and the selection end
// relative to their current positions. Positive distances moves forward,
// negative distances moves backward. Distances are in grapheme clusters,
// which closely match what users perceive as "characters" even when the
// characters are multiple code points long.
func (e *Editor) MoveCaret(startDelta, endDelta int) {
	e.initBuffer()
	e.text.MoveCaret(startDelta, endDelta)
}

// deleteWord deletes the next word(s) in the specified direction.
// Unlike moveWord, deleteWord treats whitespace as a word itself.
// Positive is forward, negative is backward.
// Absolute values greater than one will delete that many words.
// The selection counts as a single word.
func (e *Editor) deleteWord(distance int) (deletedRunes int) {
	if distance == 0 {
		return
	}

	start, end := e.text.Selection()
	if start != end {
		deletedRunes = e.Delete(1)
		distance -= sign(distance)
	}
	if distance == 0 {
		return deletedRunes
	}

	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	caret, _ := e.text.Selection()
	// atEnd if offset is at or beyond either side of the buffer.
	atEnd := func(runes int) bool {
		idx := caret + runes*direction
		return idx <= 0 || idx >= e.Len()
	}
	// next returns the appropriate rune given the direction and offset in runes).
	next := func(runes int) rune {
		idx := caret + runes*direction
		if idx < 0 {
			idx = 0
		} else if idx > e.Len() {
			idx = e.Len()
		}
		off := e.text.ByteOffset(idx)
		var r rune
		if direction < 0 {
			r, _, _ = e.text.ReadRuneBefore(int64(off))
		} else {
			r, _, _ = e.text.ReadRuneAt(int64(off))
		}
		return r
	}
	runes := 1
	for range words {
		r := next(runes)
		wantSpace := unicode.IsSpace(r)
		for r := next(runes); unicode.IsSpace(r) == wantSpace && !atEnd(runes); r = next(runes) {
			runes += 1
		}
	}
	deletedRunes += e.Delete(runes * direction)
	return deletedRunes
}

// SelectionLen returns the length of the selection, in runes; it is
// equivalent to utf8.RuneCountInString(e.SelectedText()).
func (e *Editor) SelectionLen() int {
	e.initBuffer()
	return e.text.SelectionLen()
}

// Selection returns the start and end of the selection, as rune offsets.
// start can be > end.
func (e *Editor) Selection() (start, end int) {
	e.initBuffer()
	return e.text.Selection()
}

// SetCaret moves the caret to start, and sets the selection end to end. start
// and end are in runes, and represent offsets into the editor text.
func (e *Editor) SetCaret(start, end int) {
	e.initBuffer()
	e.text.SetCaret(start, end)
	e.scrollCaret = true
	e.scroller.Stop()
}

// SelectedText returns the currently selected text (if any) from the editor.
func (e *Editor) SelectedText() string {
	e.initBuffer()
	e.scratch = e.text.SelectedText(e.scratch)
	return string(e.scratch)
}

// ClearSelection clears the selection, by setting the selection end equal to
// the selection start.
func (e *Editor) ClearSelection() {
	e.initBuffer()
	e.text.ClearSelection()
}

// WriteTo implements io.WriterTo.
func (e *Editor) WriteTo(w io.Writer) (int64, error) {
	e.initBuffer()
	return e.text.WriteTo(w)
}

// Seek implements io.Seeker.
func (e *Editor) Seek(offset int64, whence int) (int64, error) {
	e.initBuffer()
	return e.text.Seek(offset, whence)
}

// Read implements io.Reader.
func (e *Editor) Read(p []byte) (int, error) {
	e.initBuffer()
	return e.text.Read(p)
}

// Regions returns visible regions covering the rune range [start,end).
func (e *Editor) Regions(start, end int, regions []Region) []Region {
	e.initBuffer()
	return e.text.Regions(start, end, regions)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

func (s ChangeEvent) isEditorEvent() {}
func (s SubmitEvent) isEditorEvent() {}
func (s SelectEvent) isEditorEvent() {}
