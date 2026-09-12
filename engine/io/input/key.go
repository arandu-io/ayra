package input

import (
	"image"
	"slices"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
)

// EditorState is what an editor has to tell the platform about itself.
//
// It exists because the text a person types does not always arrive as
// keystrokes. A composed character, a suggestion accepted from a strip above
// the keyboard, a correction applied to a word already typed -- all of them are
// the platform editing the text, and to do that it has to know where the
// selection is, where the caret is drawn, and what is written around it.
//
// The transform is carried with the selection rather than applied to it,
// because the platform wants the caret in screen coordinates and the editor
// only knows where it drew it.
type EditorState struct {
	Selection struct {
		Transform f32.Affine2D
		key.Range
		key.Caret
		CompositionBounds image.Rectangle
	}
	Snippet key.Snippet
}

// TextInputState is what the on-screen keyboard should do next.
//
// It is an instruction rather than a condition, and it is read once: whoever
// reads it acts on it, and what follows is [TextInputKeep] until something asks
// for a change again. A condition polled every frame would reopen a keyboard a
// person had just dismissed, every frame, for as long as the field kept focus.
type TextInputState uint8

const (
	// TextInputKeep leaves the keyboard as it is.
	TextInputKeep TextInputState = iota
	// TextInputClose dismisses the keyboard.
	TextInputClose
	// TextInputOpen raises the keyboard.
	TextInputOpen
)

// keyQueue is the frame's account of who can take focus and in what order.
//
// It is rebuilt from scratch every frame, because the answer is a property of
// the frame: a handler that is not drawn this time cannot be reached by Tab,
// and one that moved has moved for the arrow keys too. What is not rebuilt is
// the focus itself, which lives in keyState and has to survive the frame that
// changes everything around it.
type keyQueue struct {
	// order is the tags that registered, in the order the frame declared them.
	// That is the order Tab walks, and it is the only ordering the author of a
	// screen writes down: the layout they see on screen is a consequence of
	// constraints, and following it would make the tab order change with the
	// window size.
	order []event.Tag
	// dirOrder is the same handlers arranged in rows for the arrow keys, where
	// what matters is where they ended up rather than when they were declared.
	dirOrder []dirFocusEntry
	// hint is the last input hint reported, kept so that a change can be
	// distinguished from a repetition.
	hint key.InputHint
}

// keyState is what the keyboard means right now, and it survives frames.
type keyState struct {
	focus   event.Tag
	state   TextInputState
	content EditorState
}

// keyHandler is one handler's share of the keyboard state.
type keyHandler struct {
	// visible reports whether the handler registered itself in the current
	// frame. A focus held by something no longer drawn is a focus a person
	// cannot see and cannot leave, so the frame takes it back.
	visible bool
	// reset records that the handler has been told, once, that it does not have
	// the focus. Without it a handler that starts unfocused is never told
	// anything at all, and has to assume rather than be informed.
	reset        bool
	hint         key.InputHint
	orderPlusOne int
	dirOrder     int
	trans        f32.Affine2D
}

// keyFilter is the union of what every handler in the frame listens for.
//
// Key events are not addressed to a tag the way pointer events are: a keystroke
// happens to the window, and who receives it depends on the focus and on what
// was asked for. So the filters are collected in one place and matched against
// the event, rather than looked up per handler.
type keyFilter []key.Filter

// dirFocusEntry is a handler's place on the screen, for the arrow keys.
type dirFocusEntry struct {
	tag    event.Tag
	row    int
	area   int
	bounds image.Rectangle
}

func (k *keyHandler) inputHint(hint key.InputHint) {
	k.hint = hint
}

// InputState answers what the keyboard should do and takes the instruction
// away, leaving the state at [TextInputKeep].
func (s keyState) InputState() (keyState, TextInputState) {
	state := s.state
	s.state = TextInputKeep
	return s, state
}

// InputHint answers the focused handler's hint and whether it differs from the
// one answered last.
//
// The second return is what a caller acts on. Telling the platform which
// keyboard to show is not free -- on some of them it closes the keyboard and
// opens another -- so it is done when the answer changes and not every frame.
func (q *keyQueue) InputHint(handlers map[event.Tag]*handler, state keyState) (key.InputHint, bool) {
	focused, ok := handlers[state.focus]
	if !ok {
		return q.hint, false
	}
	old := q.hint
	q.hint = focused.key.hint
	return q.hint, old != q.hint
}

// Reset clears what belongs to one frame, before the next is collected.
func (k *keyHandler) Reset() {
	k.visible = false
	k.orderPlusOne = 0
	k.hint = key.HintAny
}

// Reset empties the frame's focus order, keeping the allocations.
func (q *keyQueue) Reset() {
	q.order = q.order[:0]
	q.dirOrder = q.dirOrder[:0]
}

// ResetEvent answers the one event a handler gets for having asked about focus
// before anything happened: the news that it does not have it.
//
// It is delivered once per handler and never again, which is why the flag is
// not cleared between frames. Delivered every frame it would be an event that
// means nothing arriving forever, and a handler cannot tell that apart from
// having just lost the focus.
func (k *keyHandler) ResetEvent() (event.Event, bool) {
	if k.reset {
		return nil, false
	}
	k.reset = true
	return key.FocusEvent{Focus: false}, true
}

// Frame closes the focus over the frame that was just collected.
//
// A focus is a promise that a particular thing on the screen is taking the
// typing, and this is where the promise is checked against what the frame
// actually drew. A handler that stopped asking for focus events, or that was
// not drawn at all, cannot keep it -- and the keyboard raised for it is
// dismissed in the same breath, because a keyboard open for a control that is
// gone covers whatever replaced it.
func (q *keyQueue) Frame(handlers map[event.Tag]*handler, state keyState) keyState {
	if state.focus != nil {
		if h, ok := handlers[state.focus]; !ok || !h.filter.focusable || !h.key.visible {
			state.focus = nil
			state.state = TextInputClose
		}
	}
	q.updateFocusLayout(handlers)
	return state
}

// updateFocusLayout sorts the frame's handlers into rows, which is what the
// arrow keys move along and between.
//
// Rows are found greedily rather than declared, because nothing in a frame says
// "these four are a row": a row is what a person sees, and what they see is
// things that line up. So the topmost handler starts a row, its bounds are
// extended into a horizontal beam, and every handler whose centre falls in the
// beam joins it. The centre rather than the edge, so that a tall control next
// to a short one does not swallow the row beneath it.
func (q *keyQueue) updateFocusLayout(handlers map[event.Tag]*handler) {
	order := q.dirOrder
	slices.SortStableFunc(order, func(a, b dirFocusEntry) int {
		return a.bounds.Min.Y - b.bounds.Min.Y
	})
	row := 0
	for len(order) > 0 {
		first := &order[0]
		first.row = row
		bottom := first.bounds.Max.Y
		end := 1
		for ; end < len(order); end++ {
			h := &order[end]
			centre := (h.bounds.Min.Y + h.bounds.Max.Y) / 2
			if centre > bottom {
				break
			}
			h.row = row
		}
		slices.SortStableFunc(order[:end], func(a, b dirFocusEntry) int {
			return a.bounds.Min.X - b.bounds.Min.X
		})
		order = order[end:]
		row++
	}
	// Each handler is told where it landed, so that a move can start from the
	// focused one without searching for it.
	for i, o := range q.dirOrder {
		handlers[o.tag].key.dirOrder = i
	}
}

// MoveFocus hands the focus to whatever lies in the direction of dir.
//
// Forward and backward walk the declaration order and wrap around; the arrow
// keys walk the rows and stop at the edges. The difference is deliberate: Tab
// is a promise to reach everything eventually, and an arrow is a promise about
// direction, which a wrap would break by sending the focus across the screen.
func (q *keyQueue) MoveFocus(handlers map[event.Tag]*handler, state keyState, dir key.FocusDirection) (keyState, []taggedEvent) {
	if len(q.dirOrder) == 0 {
		return state, nil
	}
	order := 0
	if state.focus != nil {
		order = handlers[state.focus].key.dirOrder
	}
	focus := q.dirOrder[order]
	switch dir {
	case key.FocusForward, key.FocusBackward:
		if len(q.order) == 0 {
			break
		}
		order := 0
		if dir == key.FocusBackward {
			order = -1
		}
		if state.focus != nil {
			order = handlers[state.focus].key.orderPlusOne - 1
			if dir == key.FocusForward {
				order++
			} else {
				order--
			}
		}
		order = (order + len(q.order)) % len(q.order)
		return q.Focus(handlers, state, q.order[order])
	case key.FocusRight, key.FocusLeft:
		next := order
		if state.focus != nil {
			next = order + 1
			if dir == key.FocusLeft {
				next = order - 1
			}
		}
		if 0 <= next && next < len(q.dirOrder) {
			newFocus := q.dirOrder[next]
			// Only within the row: the neighbour of the last control in a row
			// is the first of the next one, and moving right onto it would send
			// the focus leftwards across the whole screen.
			if newFocus.row == focus.row {
				return q.Focus(handlers, state, newFocus.tag)
			}
		}
	case key.FocusUp, key.FocusDown:
		delta := +1
		if dir == key.FocusUp {
			delta = -1
		}
		nextRow := 0
		if state.focus != nil {
			nextRow = focus.row + delta
		}
		var closest event.Tag
		// A distance no real layout reaches, standing for "nothing found yet".
		dist := int(1e6)
		centre := (focus.bounds.Min.X + focus.bounds.Max.X) / 2
		// Walk outwards from the current position, keeping the candidate whose
		// centre is nearest horizontally. The row is sorted by x, so distances
		// fall and then rise: the first rise is the answer, and stopping there
		// is what keeps a move down from scanning the whole screen.
	loop:
		for 0 <= order && order < len(q.dirOrder) {
			next := q.dirOrder[order]
			switch next.row {
			case nextRow:
				nextCentre := (next.bounds.Min.X + next.bounds.Max.X) / 2
				d := centre - nextCentre
				if d < 0 {
					d = -d
				}
				if d > dist {
					break loop
				}
				dist = d
				closest = next.tag
			case nextRow + delta:
				// Past the row being searched, with nothing found in it.
				break loop
			}
			order += delta
		}
		if closest != nil {
			return q.Focus(handlers, state, closest)
		}
	}
	return state, nil
}

// BoundsFor answers where the handler was drawn this frame.
func (q *keyQueue) BoundsFor(k *keyHandler) image.Rectangle {
	return q.dirOrder[k.dirOrder].bounds
}

// AreaFor answers the clip area the handler was registered under, which is what
// a scroll or a synthetic press has to be delivered through.
func (q *keyQueue) AreaFor(k *keyHandler) int {
	return q.dirOrder[k.dirOrder].area
}

// Matches reports whether any filter in the set wants this key event.
func (k *keyFilter) Matches(focus event.Tag, e key.Event, system bool) bool {
	for _, f := range *k {
		if keyFilterMatch(focus, f, e, system) {
			return true
		}
	}
	return false
}

// keyFilterMatch is one filter against one event.
//
// The modifiers are matched in both directions, and that is the part worth
// reading twice: every required modifier has to be held, and every modifier
// held has to be required or named optional. Without the second half a filter
// for a plain letter would also catch the shortcut built on it, and the
// shortcut would happen while the letter was being typed.
func keyFilterMatch(focus event.Tag, f key.Filter, e key.Event, system bool) bool {
	if f.Focus != nil && f.Focus != focus {
		return false
	}
	// A filter that names no key is a catch-all, and a catch-all is not offered
	// events the platform reserves for itself: the window moves the focus with
	// them unless something asked for that key by name.
	if (f.Name != "" || system) && f.Name != e.Name {
		return false
	}
	if e.Modifiers&f.Required != f.Required {
		return false
	}
	if e.Modifiers&^(f.Required|f.Optional) != 0 {
		return false
	}
	return true
}

// Focus moves the focus to focus, and says who has to be told.
//
// Both sides are told, and the old holder first. A handler that is never told
// it lost the focus goes on drawing a caret, and two carets on one screen is a
// screen where nobody knows where their typing is going.
//
// The editor state is dropped on the way, because it described the selection in
// the handler that just lost the focus. Carried over, it would describe a
// selection in text the new handler does not have.
func (q *keyQueue) Focus(handlers map[event.Tag]*handler, state keyState, focus event.Tag) (keyState, []taggedEvent) {
	if focus == state.focus {
		return state, nil
	}
	state.content = EditorState{}
	state.content.Selection.Transform = f32.AffineId()
	var evts []taggedEvent
	if state.focus != nil {
		evts = append(evts, taggedEvent{tag: state.focus, event: key.FocusEvent{Focus: false}})
	}
	state.focus = focus
	if state.focus != nil {
		evts = append(evts, taggedEvent{tag: state.focus, event: key.FocusEvent{Focus: true}})
	}
	// A handler that wants the keyboard asks for it, and it asks while taking
	// the focus. Anything that did not ask gets it closed: moving to a button
	// with the keyboard still up leaves half the screen covered by it.
	if state.focus == nil || state.state == TextInputKeep {
		state.state = TextInputClose
	}
	return state, evts
}

func (s keyState) softKeyboard(show bool) keyState {
	if show {
		s.state = TextInputOpen
	} else {
		s.state = TextInputClose
	}
	return s
}

// Add records a filter, ignoring one already recorded.
//
// The same filter arrives many times per frame -- a control asks on every call
// that might have an event for it -- and matching is a walk over the set, so
// letting duplicates in makes every keystroke cost more for nothing.
func (k *keyFilter) Add(f key.Filter) {
	if slices.Contains(*k, f) {
		return
	}
	*k = append(*k, f)
}

func (k *keyFilter) Merge(k2 keyFilter) {
	*k = append(*k, k2...)
}

// inputOp records that tag is on the screen and can be reached.
//
// Only the first registration in a frame counts for the order. A control drawn
// once may register more than once, and taking the later one would move it in
// the tab order for reasons its author cannot see in the frame they wrote.
func (q *keyQueue) inputOp(tag event.Tag, state *keyHandler, t f32.Affine2D, area int, bounds image.Rectangle) {
	state.visible = true
	if state.orderPlusOne == 0 {
		state.orderPlusOne = len(q.order) + 1
		q.order = append(q.order, tag)
		q.dirOrder = append(q.dirOrder, dirFocusEntry{tag: tag, area: area, bounds: bounds})
	}
	state.trans = t
}

// setSelection takes the focused editor's selection, and ignores anyone else's.
//
// An unfocused editor reporting a selection is not wrong to report it -- it
// still has one -- but the platform is told about the focused one, and letting
// the last caller win would describe the wrong text to whatever is composing.
func (q *keyQueue) setSelection(state keyState, req key.SelectionCmd) keyState {
	if req.Tag != state.focus {
		return state
	}
	state.content.Selection.Range = req.Range
	state.content.Selection.Caret = req.Caret
	state.content.Selection.CompositionBounds = req.CompositionBounds
	return state
}

// editorState answers what the platform needs to know about the focused editor.
//
// The transform is taken from the handler at the moment of asking rather than
// from what was stored with the selection, because the editor may have been
// moved or scrolled by the frame that has just been drawn: the selection is
// still the same characters, and it is no longer in the same place on screen.
func (q *keyQueue) editorState(handlers map[event.Tag]*handler, state keyState) EditorState {
	s := state.content
	if f := state.focus; f != nil {
		s.Selection.Transform = handlers[f].key.trans
	}
	return s
}

func (q *keyQueue) setSnippet(state keyState, req key.SnippetCmd) keyState {
	if req.Tag == state.focus {
		state.content.Snippet = req.Snippet
	}
	return state
}

// String names the state, for a person reading a log or a failed test.
func (t TextInputState) String() string {
	switch t {
	case TextInputKeep:
		return "Keep"
	case TextInputClose:
		return "Close"
	case TextInputOpen:
		return "Open"
	default:
		panic("unexpected value")
	}
}
