package widget

import (
	"github.com/arandu-io/ayra"
	engine "github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/mask"
)

// Masked is the state half of a field that formats what is typed in it. It is an
// [Input] that keeps its text in the shape of a mask, and the caller holds one
// per field.
type Masked struct {
	input     Input
	submitted bool
}

// Text is what the field shows: the formatted value.
func (m *Masked) Text() string { return m.input.Text() }

// Value is what a program stores: the text with the mask taken off, which is the
// same whether the field holds the formatted value or the raw one. A nil mask
// answers the text as it is.
func (m *Masked) Value(by mask.Mask) string {
	if by == nil {
		return m.input.Text()
	}
	return by.Unmask(m.input.Text())
}

// Complete reports whether enough was typed to fill the mask. It answers about
// the shape and nothing else: eleven digits are not a CPF until the application
// says they are.
func (m *Masked) Complete(by mask.Mask) bool {
	if by == nil {
		return m.input.Text() != ""
	}
	return by.Complete(m.input.Text())
}

// SetValue puts a raw or formatted value in the field, shown formatted.
func (m *Masked) SetValue(by mask.Mask, value string) {
	if by != nil {
		value = by.Apply(value)
	}
	m.input.SetText(value)
}

// Submitted reports that return was pressed on the field, once, and consumes it.
func (m *Masked) Submitted() bool {
	submitted := m.submitted
	m.submitted = false
	return submitted
}

// MaskedProps is a text field that formats what somebody types and keeps what a
// program stores -- the counterpart of Kyse's Masked.
//
// The mask is a [mask.Mask]: a [mask.Pattern] for a shape such as 00000-000, a
// [mask.Alternatives] for a field that takes one of two (a phone number of ten
// digits or of eleven), [mask.Money] for an amount, or a type of the product's
// own. The shapes Brazilian forms ask for -- CPF, CNPJ, phone, CEP, licence
// plate and the real -- are in [github.com/arandu-io/ayra/mask/brazil], and the
// rule they share with the web half is the one in hesape/mask.
//
// It formats and does not decide. A mask says what may be typed; whether what
// was typed means anything is the server's answer, and [Masked.Value] is the
// raw value it should be asked about.
type MaskedProps struct {
	// ComponentProps is what the caller changes on this one instance. A masked
	// field publishes the parts of an [InputProps], restyled under the name
	// "masked".
	ComponentProps
	// Mask is the shape. Nil makes it an ordinary field.
	Mask mask.Mask
	// Placeholder is shown while the field is empty.
	Placeholder string
	// Kind is what the field expects, which decides the on-screen keyboard. A
	// digits-only mask wants [Number] or [Telephone].
	Kind Kind
	// Disabled draws the field as unavailable and refuses input.
	Disabled bool
	// Invalid draws the field as rejected.
	Invalid bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p MaskedProps) PartNames() []string {
	return []string{"root", "focus", "invalid", "placeholder"}
}

// Layout draws the field and returns the room it took.
func (p MaskedProps) Layout(c ayra.Context, state *Masked) ayra.Dimensions {
	editor := &state.input.editor
	editor.SingleLine, editor.Submit = true, true

	// What was typed since the last frame is read first, so that the field is
	// drawn already formatted. Left to the layout below, the keystroke would be
	// drawn as typed for a frame and formatted in the next.
	for {
		event, ok := editor.Update(c.Context)
		if !ok {
			break
		}
		if _, submitted := event.(engine.SubmitEvent); submitted {
			state.submitted = true
		}
	}
	if p.Mask != nil && !p.Disabled {
		format(editor, p.Mask)
	}

	return InputProps{
		ComponentProps: p.ComponentProps,
		widget:         "masked",
		Placeholder:    p.Placeholder,
		Kind:           p.Kind,
		Disabled:       p.Disabled,
		Invalid:        p.Invalid,
	}.Layout(c, &state.input)
}

// format puts the editor's text in the shape of the mask and keeps the caret
// beside the character it was beside.
//
// The caret is carried by what was before it: the characters up to the caret
// are unmasked and masked again, and the length of that is where the caret goes.
// An amount is the one exception, because its digits are placed from the right
// and a caret in the middle of it would mean a different digit after the next
// keystroke; it stays at the end.
func format(editor *engine.Editor, by mask.Mask) {
	text := editor.Text()
	shown := by.Apply(text)
	if shown == text {
		return
	}

	_, column := editor.CaretPos()
	runes := []rune(text)
	column = min(column, len(runes))

	caret := len([]rune(by.Apply(by.Unmask(string(runes[:column])))))
	if _, amount := by.(mask.Money); amount {
		caret = len([]rune(shown))
	}

	editor.SetText(shown)
	editor.SetCaret(caret, caret)
}
