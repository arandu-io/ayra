package widget

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
)

// field builds the context a control is drawn into, without a window and
// without a GPU. A frame is a list of operations, and a list can be built and
// measured on a machine with no display -- which is what a build server is.
func field(t *testing.T, scheme theme.Scheme, width int) (ayra.Context, *op.Ops) {
	t.Helper()

	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(width, 1000)},
	}
	return ayra.Context{
		Context: gtx,
		Theme:   theme.New(scheme),
		Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
	}, ops
}

// hintName is what the platform was told to raise, as a word, so a failure
// says which keyboard rather than which integer.
func hintName(h key.InputHint) string {
	switch h {
	case key.HintEmail:
		return "email"
	case key.HintPassword:
		return "password"
	case key.HintNumeric:
		return "numeric"
	case key.HintTelephone:
		return "telephone"
	case key.HintURL:
		return "url"
	case key.HintText:
		return "text"
	}
	return "any"
}

// TestAFieldFillsTheRoomItIsGiven fixes the one layout decision a form
// depends on: a row of fields that each ended where their content did would be
// a form with a ragged right edge, and no amount of styling fixes that after.
func TestAFieldFillsTheRoomItIsGiven(t *testing.T) {
	for _, width := range []int{200, 400, 800} {
		c, _ := field(t, theme.Light, width)
		var state Input

		dims := InputProps{Placeholder: "Seu e-mail"}.Layout(c, &state)

		if dims.Size.X != width {
			t.Errorf("in %dpt of room the field took %dpt, want the whole width", width, dims.Size.X)
		}
	}
}

// TestTheFieldHoldsWhatWasTyped is the contract everything else rests on.
func TestTheFieldHoldsWhatWasTyped(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	var state Input

	state.SetText("ada@example.org")
	InputProps{Kind: Email}.Layout(c, &state)

	if got := state.Text(); got != "ada@example.org" {
		t.Errorf("the field holds %q", got)
	}
}

// TestAPasswordFieldHidesWhatIsTyped keeps the one property that, missing,
// is a screenshot of somebody's password.
//
// The mask is checked on the state the control configures rather than on the
// pixels, because reading pixels needs a GPU and this has to fail on a build
// server too.
func TestAPasswordFieldHidesWhatIsTyped(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	var state Input

	state.SetText("hunter2")
	InputProps{Kind: Password}.Layout(c, &state)

	if state.editor.Mask == 0 {
		t.Error("a password field is drawing its characters")
	}

	// And the same field asked for prose shows them again, so the mask is a
	// property of the kind and not something that sticks once set.
	InputProps{Kind: Text}.Layout(c, &state)
	if state.editor.Mask != 0 {
		t.Error("a text field is still masking after a password field was drawn")
	}
}

// TestEveryKindRaisesItsOwnKeyboard fixes that Kind means something on a
// phone. A kind that raised the same keyboard as prose would be a word in the
// API that changes nothing, and the person typing a phone number would get
// letters.
func TestEveryKindRaisesItsOwnKeyboard(t *testing.T) {
	kinds := []Kind{Text, Email, Password, Number, Telephone, URL}

	seen := map[string]Kind{}
	for _, k := range kinds {
		c, _ := field(t, theme.Light, 400)
		var state Input
		InputProps{Kind: k}.Layout(c, &state)

		hint := hintName(state.editor.InputHint)
		if other, repeated := seen[hint]; repeated {
			t.Errorf("%s and %s raise the same keyboard (%s)", k, other, hint)
		}
		seen[hint] = k
	}
}

// TestASingleLineFieldSubmitsAndAMultilineOneDoesNot fixes what return means,
// which is different in the two and is the kind of thing that reads as a bug
// either way round: a login form where return does nothing, or a note field
// that submits instead of adding a line.
func TestASingleLineFieldSubmitsAndAMultilineOneDoesNot(t *testing.T) {
	c, _ := field(t, theme.Light, 400)

	var single Input
	InputProps{}.Layout(c, &single)
	if !single.editor.Submit {
		t.Error("return does nothing on a single-line field")
	}

	var many Input
	InputProps{Multiline: true}.Layout(c, &many)
	if many.editor.Submit {
		t.Error("return submits a multiline field instead of adding a line")
	}
}

// TestADisabledFieldRefusesInput keeps the drawing and the behaviour saying
// the same thing.
func TestADisabledFieldRefusesInput(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	var state Input

	InputProps{Disabled: true}.Layout(c, &state)

	if !state.editor.ReadOnly {
		t.Error("a field drawn as unavailable still accepts typing")
	}
}
