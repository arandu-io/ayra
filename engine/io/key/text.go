package key

import (
	"image"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/internal/ops"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/op"
)

// Text does not arrive one key at a time, and this is the half of the package
// that admits it.
//
// Between the keyboard and a field there is an input method: the thing that
// turns a dead key and a vowel into an accented letter, several keystrokes into
// one character, and a word into the suggestion above it. It works by holding a
// piece of the field's text -- the composing text -- and rewriting it until the
// person is finished. For that it has to be able to read back what the field
// currently holds and where the caret is, so this is a conversation and not a
// stream of events: the field answers with [SnippetCmd] and [SelectionCmd], and
// is told what to become with [EditEvent].
//
// Getting this wrong is not a missing feature. A field that only listens to
// presses takes typing in English and refuses it in most of the world.

// Range is a stretch of text, such as a selection.
//
// Both ends are counted in runes rather than bytes, because they are exchanged
// with an input method that has no idea how this program stores a string, and
// because a caret between the two bytes of one character is not a position
// anybody can type at.
type Range struct {
	// Start is the first rune of the range.
	Start int
	// End is one past the last. It may be before Start, and that is meaningful:
	// which end moved last is which end a selection grows from.
	End int
}

// Snippet is a piece of a field's text, and where in the field it came from.
//
// It is a piece rather than the whole because an input method only needs what
// is around the caret, and a field can hold a book.
type Snippet struct {
	Range
	// Text is the content of Range.
	Text string
}

// Caret is where the insertion point is on the screen.
//
// It is reported in the field's own coordinates, and it is what an input method
// positions its candidate window against. A caret reported wrongly puts that
// window over the word being typed, which is the one thing the person needs to
// see.
type Caret struct {
	// Pos is where the caret meets its baseline.
	Pos f32.Point
	// Ascent is how far the caret reaches above that baseline.
	Ascent float32
	// Descent is how far it reaches below.
	Descent float32
}

// An EditEvent is an input method replacing part of a field's text.
//
// It is a replacement rather than an insertion because that is what composing
// is: every keystroke of a composed character rewrites the same stretch of
// text, and a field that appended instead would show every intermediate guess.
type EditEvent struct {
	// Range is what to replace, in runes.
	Range Range
	// Text is what to replace it with.
	Text string
}

func (EditEvent) ImplementsEvent() {}

// SelectionEvent is an input method moving the selection.
type SelectionEvent Range

func (SelectionEvent) ImplementsEvent() {}

// CompositionEvent is an input method changing which stretch of text it is
// still composing. An empty range means it has finished.
type CompositionEvent Range

func (CompositionEvent) ImplementsEvent() {}

// SnippetEvent is an input method asking for the text in a range, which the
// field answers with [SnippetCmd].
type SnippetEvent Range

func (SnippetEvent) ImplementsEvent() {}

// SelectionCmd tells the input method where the caret and the selection are.
//
// A field sends it whenever either moves, including when the person moved it
// with the mouse: an input method that is composing against a caret which has
// since been dragged elsewhere writes the finished character into the wrong
// place.
type SelectionCmd struct {
	// Tag is the field this is about.
	Tag event.Tag
	Range
	Caret
	// CompositionBounds is where the composing text is on the screen, in the
	// field's own coordinates. It is empty when nothing is being composed.
	CompositionBounds image.Rectangle
}

func (SelectionCmd) ImplementsCommand() {}

// SnippetCmd answers a [SnippetEvent] with the text that was asked for.
type SnippetCmd struct {
	// Tag is the field this is about.
	Tag event.Tag
	Snippet
}

func (SnippetCmd) ImplementsCommand() {}

// SoftKeyboardCmd shows or hides the on-screen keyboard.
//
// It does nothing where there is no such keyboard, which is most of the time on
// a desktop, so it is safe to send without asking what the device is.
type SoftKeyboardCmd struct {
	// Show is whether to bring the keyboard up.
	Show bool
}

func (SoftKeyboardCmd) ImplementsCommand() {}

// InputHint says what kind of text a field expects.
//
// It is a hint and not a rule: nothing here validates anything, and a field
// hinting at a number still has to refuse letters itself. What it buys is the
// on-screen keyboard the person is given -- digits for an amount, "@" and
// ".com" for an address -- and on a phone that is the difference between one
// tap and four.
type InputHint uint8

const (
	// HintAny expects anything, and asks for no particular keyboard.
	HintAny InputHint = iota
	// HintText expects prose. It may turn on autocorrection and suggestions.
	HintText
	// HintNumeric expects numbers. It may offer 0-9, "." and ",".
	HintNumeric
	// HintEmail expects an address. It may offer "@" and ".com".
	HintEmail
	// HintURL expects an address. It may offer "/" and ".com".
	HintURL
	// HintTelephone expects a telephone number. It may offer 0-9, "#" and "*".
	HintTelephone
	// HintPassword expects a secret. It may turn autocorrection off and
	// password autofill on.
	HintPassword
)

// InputHintOp attaches a hint to a field.
//
// It is an operation on the frame rather than a field of a command because the
// hint belongs to whatever is drawn, and what is drawn changes every frame. The
// hint that counts is the one on the focused tag, and it takes effect when that
// tag has the focus.
type InputHintOp struct {
	// Tag is the field the hint belongs to.
	Tag event.Tag
	// Hint is what that field expects.
	Hint InputHint
}

// Add records the hint on the frame.
//
// A nil Tag stops the program, because it cannot be recorded against anything:
// the hint would be written, cost a frame, and reach no field. That is a
// mistake in the calling code and not a state to be carried, so it is reported
// where it was made rather than as a keyboard that is subtly the wrong one.
func (h InputHintOp) Add(o *op.Ops) {
	if h.Tag == nil {
		panic("Tag must be non-nil")
	}
	data := ops.Write1(&o.Internal, ops.TypeKeyInputHintLen, h.Tag)
	data[0] = byte(ops.TypeKeyInputHint)
	data[1] = byte(h.Hint)
}
