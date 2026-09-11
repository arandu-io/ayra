package widget

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/unit"
)

// TextareaProps is a field somebody writes more than a line into.
//
// It is [InputProps] with the multiline decision already made, and a height, so
// a caller cannot produce the thing that looks like a text area and submits on
// return. That mistake is invisible until somebody presses return expecting a
// new line and their half-written message is sent.
type TextareaProps struct {
	// Placeholder is shown while it is empty.
	Placeholder string
	// Rows is how many lines tall it opens. Zero takes a paragraph.
	Rows int
	// Disabled draws it as unavailable and refuses input.
	Disabled bool
	// Invalid draws it as rejected. What was wrong is said beside it.
	Invalid bool
}

// Layout draws the field and returns the room it took.
func (p TextareaProps) Layout(c ayra.Context, state *Input) ayra.Dimensions {
	rows := p.Rows
	if rows == 0 {
		rows = 4
	}

	// The height is a floor rather than a fixed size: a field that stopped
	// growing would scroll its own text out of sight while somebody is still
	// writing it.
	line := c.Sp(unit.Sp(c.Theme.Type.Body))
	c.Constraints.Min.Y = max(c.Constraints.Min.Y, line*rows+c.Dp(unit.Dp(16)))

	return InputProps{
		Placeholder: p.Placeholder,
		Multiline:   true,
		Disabled:    p.Disabled,
		Invalid:     p.Invalid,
	}.Layout(c, state)
}
