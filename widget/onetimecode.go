package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// OneTimeCode is the state half of a code typed one character to a square: a
// six-digit code from an e-mail or a text message.
type OneTimeCode struct {
	input Input
	// completed is whether [OneTimeCode.Completed] has already answered for the
	// code as it stands, so that a code filled in answers once.
	completed bool
}

// Code is what has been typed so far.
func (o *OneTimeCode) Code() string { return o.input.Text() }

// Set replaces the code, for a code the platform filled in or a test.
func (o *OneTimeCode) Set(code string) {
	o.input.SetText(code)
	o.completed = false
}

// Clear empties the code, for a code the server refused.
func (o *OneTimeCode) Clear() {
	o.input.SetText("")
	o.completed = false
}

// Completed reports that the code was just filled in, once. It is the moment to
// send it: asking is the same as the field raising a change event that a
// button press is not needed to follow.
func (o *OneTimeCode) Completed(c ayra.Context, length int) bool {
	full := len([]rune(o.input.Text())) >= length
	if full && !o.completed {
		o.completed = true
		return true
	}
	if !full {
		o.completed = false
	}
	return false
}

// OneTimeCodeProps is a code typed into a row of squares -- the counterpart of
// Kyse's OneTimeCode. Typing fills the next square, deleting empties the last
// one, and a pasted code fills them all.
//
// It is one editor drawn as squares, and the editor is laid over the whole row
// with transparent ink: that is what gives the row the pointer, the keyboard and
// the platform's own autofill of a code, which a row of separate fields would
// have to rebuild between them.
//
// It holds characters and decides nothing about them: whether the code is right
// is the server's answer.
type OneTimeCodeProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes "root", the row; "square"; and "focus" and "invalid", which are
	// laid over the square that has the focus and over all of them when the code
	// was refused.
	ComponentProps
	// Length is how many squares. Zero takes six.
	Length int
	// Alphanumeric accepts letters as well as digits. Zero takes digits only,
	// which is what a code sent by message is.
	Alphanumeric bool
	// Disabled draws it as unavailable and refuses input.
	Disabled bool
	// Invalid draws the squares as rejected.
	Invalid bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p OneTimeCodeProps) PartNames() []string {
	return []string{"root", "square", "focus", "invalid"}
}

// Layout draws the row and returns the room it took.
func (p OneTimeCodeProps) Layout(c ayra.Context, state *OneTimeCode) ayra.Dimensions {
	n := p.Length
	if n <= 0 {
		n = 6
	}
	t := c.Theme

	editor := &state.input.editor
	editor.SingleLine = true
	editor.ReadOnly = p.Disabled
	editor.MaxLen = n
	editor.InputHint = Number.hint()
	if p.Alphanumeric {
		editor.Filter = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		editor.InputHint = Text.hint()
	} else {
		editor.Filter = "0123456789"
	}
	for {
		if _, ok := editor.Update(c.Context); !ok {
			break
		}
	}

	focused := c.Focused(editor)
	typed := []rune(editor.Text())
	current := min(len(typed), n-1)

	root := t.Resolve("onetimecode", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)

	gap := c.Dp(unit.Dp(8))
	side := min(c.Dp(unit.Dp(44)), max((c.Constraints.Max.X-gap*(n-1))/n, 1))
	height := side * 12 / 11
	row := image.Pt(side*n+gap*(n-1), height)

	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		for i := 0; i < n; i++ {
			style := p.square(c, i == current && focused)
			origin := op.Offset(image.Pt(i*(side+gap), 0)).Push(c.Ops)

			sc := c
			sc.Constraints = layout.Exact(image.Pt(side, height))
			digit := ""
			if i < len(typed) {
				digit = string(typed[i])
			}
			Surface(sc, style, func(c ayra.Context) ayra.Dimensions {
				c.Constraints.Min = image.Pt(side, height)
				return layout.Center.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					return drawPart(inner, digit, style, unit.Sp(t.Type.Heading), 1, text.Middle, font.Font{Weight: font.Medium})
				})
			})
			if i == current && focused && !p.Disabled {
				// The square the next character goes into holds the ring Kyse
				// draws around a focused field, in the destructive colour when the
				// code was refused.
				ring := t.Colours.Ring
				if p.Invalid {
					ring = t.Colours.Destructive
				}
				focusRing(sc, image.Pt(side, height), c.Dp(unit.Dp(style.Radius.Points())), ring)
			}
			origin.Pop()
		}

		// The editor over the row, in ink that is not there.
		hidden := op.Record(c.Ops)
		paint.ColorOp{}.Add(c.Ops)
		ink := hidden.Stop()

		ec := c
		ec.Constraints = layout.Exact(row)
		editor.Layout(ec.Context, c.Shaper, c.Theme.Type.Resolve(font.Font{}), unit.Sp(t.Type.Body), ink, ink)
		return ayra.Dimensions{Size: row}
	})
}

// square is how one square looks, with the state it is in laid over the root.
func (p OneTimeCodeProps) square(c ayra.Context, focus bool) theme.Style {
	t, variant := c.Theme, p.VariantName("default")

	style := t.Resolve("onetimecode", "square", variant, theme.Style{
		Background:  fieldFill(t),
		Foreground:  theme.Colour(t.Colours.Foreground),
		Border:      theme.Colour(t.Colours.Input),
		BorderWidth: theme.Pt(1),
		Radius:      theme.Pt(t.Metrics.Radius),
		Font:        theme.Font{Size: theme.Pt(t.Type.Heading)},
	}, p.ComponentProps)

	switch {
	case p.Invalid:
		style = t.Resolve("onetimecode", "invalid", variant, theme.Style{Border: theme.Colour(t.Colours.Destructive)}, theme.Component{Parts: p.Parts}).Or(style)
	case focus:
		style = t.Resolve("onetimecode", "focus", variant, theme.Style{Border: theme.Colour(t.Colours.Ring)}, theme.Component{Parts: p.Parts}).Or(style)
	}
	if p.Disabled {
		ground := t.Colours.Background
		style.Border = theme.Colour(fadeOver(style.Border.NRGBA(), ground))
		style.Foreground = theme.Colour(fadeOver(style.Foreground.NRGBA(), ground))
	}
	return style
}
