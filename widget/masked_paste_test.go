package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/mask"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/widget"
)

// TestAPasteIsFormattedWhateverItCarries: what is pasted is usually the value
// formatted, or with a space or a line break the source had at its end.
func TestAPasteIsFormattedWhateverItCarries(t *testing.T) {
	cases := []struct {
		name  string
		mask  mask.Mask
		paste string
		want  string
	}{
		{"cpf formatted", brazil.CPF, "123.456.789-09", "123.456.789-09"},
		{"cpf raw with spaces", brazil.CPF, " 12345678909\n", "123.456.789-09"},
		{"cpf too long", brazil.CPF, "123456789012345", "123.456.789-01"},
		{"cep with accent noise", brazil.CEP, "CEP: 01310-100", "01310-100"},
		{"phone", brazil.Phone, "+(11) 98765-4321", "(11) 98765-4321"},
		{"plate old", brazil.Plate, "ABC1234", "ABC1234"},
		{"plate with hyphen", brazil.Plate, "ABC-1234", "ABC1234"},
		{"money", brazil.Real, "1234567", "R$ 12.345,67"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newPanel()
			var state widget.Masked
			screen := func(c ayra.Context) ayra.Dimensions {
				return widget.MaskedProps{Mask: tc.mask}.Layout(c, &state)
			}
			p.draw(screen)
			tap(p, screen, 100, 15)
			typeText(p, screen, tc.paste)
			p.draw(screen)
			if got := state.Text(); got != tc.want {
				t.Errorf("pasting %q left %q, want %q", tc.paste, got, tc.want)
			}
		})
	}
}
