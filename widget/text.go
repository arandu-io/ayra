package widget

import (
	"image/color"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	giowidget "github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/theme"
)

// Role is what a piece of text is for, which is what decides its size.
//
// It names the job and not the number, so a screen says what a line is rather
// than how big it is -- and the scale can be retuned in one place without
// visiting every screen that used it.
type Role uint8

const (
	// Body is running text and the ordinary line. Zero, because it is what
	// most text is.
	Body Role = iota
	// Display is the one line a screen is about.
	Display
	// Heading is a section.
	Heading
	// Caption is a description under a field, and a note.
	Caption
	// Mono is code, an identifier, and a column of figures that has to line
	// up.
	Mono
)

// String names the role, for a diagnostic screen and for a test failure.
func (r Role) String() string {
	switch r {
	case Display:
		return "display"
	case Heading:
		return "heading"
	case Caption:
		return "caption"
	case Mono:
		return "mono"
	}
	return "body"
}

// size answers the type size this role is set at.
func (r Role) size(t theme.Theme) unit.Sp {
	switch r {
	case Display:
		return unit.Sp(t.Type.Display)
	case Heading:
		return unit.Sp(t.Type.Heading)
	case Caption:
		return unit.Sp(t.Type.Small)
	case Mono:
		return unit.Sp(t.Type.Mono)
	}
	return unit.Sp(t.Type.Body)
}

// Tone is how much attention a line is asking for.
//
// It is separate from Role because the two vary independently: a heading can
// be quiet and a caption can be a warning, and folding them into one set would
// need an entry for every pair.
type Tone uint8

const (
	// Normal reads against the surface it is on.
	Normal Tone = iota
	// Muted is present and secondary: a label, a hint, a timestamp.
	Muted
	// Danger is what went wrong.
	Danger
	// Accent is the one line on a screen that is meant to be found first.
	Accent
)

// String names the tone.
func (t Tone) String() string {
	switch t {
	case Muted:
		return "muted"
	case Danger:
		return "danger"
	case Accent:
		return "accent"
	}
	return "normal"
}

// ink answers the colour this tone reads in.
func (t Tone) ink(th theme.Theme) color.NRGBA {
	switch t {
	case Muted:
		return th.Colours.MutedForeground
	case Danger:
		return th.Colours.Destructive
	case Accent:
		return th.Colours.Primary
	}
	return th.Colours.Foreground
}

// TextProps is what a piece of text is drawn from.
//
// It carries no state, so there is no second half to hold across frames: text
// is not pressed, focused or typed into. A field that can be typed into is an
// [Input].
type TextProps struct {
	// Content is the text.
	Content string
	// Role is what it is for. Zero is running text.
	Role Role
	// Tone is how much attention it asks for. Zero reads against the surface.
	Tone Tone
	// MaxLines caps the height. Zero is as many as the text needs, which is
	// what running text wants; one is what a label in a row wants, so that a
	// long value pushes nothing out of place.
	MaxLines int
	// Align is where the text sits in the room it was given.
	Align text.Alignment
	// Bold sets it in the heavier weight. It is a flag and not a number
	// because two weights is what this scale has, and a third would be a
	// weight only some faces carry.
	Bold bool
}

// Layout draws the text and returns the room it took.
func (p TextProps) Layout(c ayra.Context) ayra.Dimensions {
	return drawText(c, p.Content, p.Role.size(c.Theme), p.Tone.ink(c.Theme), p.MaxLines, p.Align, p.face())
}

// face answers the font this is set in.
func (p TextProps) face() font.Font {
	f := font.Font{}
	if p.Role == Mono {
		f.Typeface = "Go Mono"
	}
	if p.Bold {
		f.Weight = font.Bold
	}
	return f
}

// drawText is the one place text is put on screen.
//
// The colour arrives as a recorded operation because that is what the shaper
// takes: it draws the glyphs and then replays what it was handed to fill them,
// so a caller cannot set a colour that the shaping forgets.
func drawText(c ayra.Context, content string, size unit.Sp, ink color.NRGBA, maxLines int, align text.Alignment, f font.Font) ayra.Dimensions {
	recording := op.Record(c.Ops)
	paint.ColorOp{Color: ink}.Add(c.Ops)
	stop := recording.Stop()

	return giowidget.Label{MaxLines: maxLines, Alignment: align}.Layout(c.Context, c.Shaper, f, size, content, stop)
}
