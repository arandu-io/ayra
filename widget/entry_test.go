package widget_test

import (
	"testing"

	"github.com/arandu-io/ayra/mask"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// TestAMaskedFieldIsDrawnFormatted: what was in the field when the frame began
// is what the frame shows, in the shape of the mask, with nothing for the caller
// to do.
func TestAMaskedFieldIsDrawnFormatted(t *testing.T) {
	for _, c := range []struct {
		name   string
		mask   mask.Mask
		typed  string
		shown  string
		stored string
	}{
		{"cpf", brazil.CPF, "12345678909", "123.456.789-09", "12345678909"},
		{"cnpj", brazil.CNPJ, "12345678000195", "12.345.678/0001-95", "12345678000195"},
		{"phone", brazil.Phone, "11987654321", "(11) 98765-4321", "11987654321"},
		{"cep", brazil.CEP, "01310100", "01310-100", "01310100"},
		{"plate", brazil.Plate, "ABC1D23", "ABC1D23", "ABC1D23"},
		{"money", brazil.Real, "123456", "R$ 1.234,56", "123456"},
		{"date", mask.Date, "10092026", "10/09/2026", "10092026"},
		{"time", mask.Time, "0930", "09:30", "0930"},
		{"letters refused", brazil.CEP, "01a310b100", "01310-100", "01310100"},
		{"the product's own", mask.Pattern("AA-9999"), "xy12", "xy-12", "xy12"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx, _ := frame(t, theme.Light, 400)
			var field widget.Masked
			field.SetValue(nil, c.typed)

			dims := widget.MaskedProps{Mask: c.mask}.Layout(ctx, &field)
			if dims.Size.X <= 0 {
				t.Fatalf("the field took %v", dims.Size)
			}
			if got := field.Text(); got != c.shown {
				t.Errorf("the field shows %q, want %q", got, c.shown)
			}
			if got := field.Value(c.mask); got != c.stored {
				t.Errorf("the field stores %q, want %q", got, c.stored)
			}
		})
	}
}

func TestAMaskedFieldKnowsWhenItIsComplete(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var field widget.Masked

	field.SetValue(nil, "1234567890")
	widget.MaskedProps{Mask: brazil.CPF}.Layout(ctx, &field)
	if field.Complete(brazil.CPF) {
		t.Error("ten digits completed a CPF")
	}

	field.SetValue(brazil.CPF, "12345678909")
	if !field.Complete(brazil.CPF) {
		t.Error("eleven digits did not complete a CPF")
	}
}

func TestAOneTimeCodeAnswersOnceWhenItIsFull(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var code widget.OneTimeCode

	dims := widget.OneTimeCodeProps{}.Layout(ctx, &code)
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		t.Fatalf("the row took %v", dims.Size)
	}

	code.Set("123")
	widget.OneTimeCodeProps{}.Layout(ctx, &code)
	if code.Completed(ctx, 6) {
		t.Error("three of six characters completed the code")
	}

	code.Set("123456")
	widget.OneTimeCodeProps{}.Layout(ctx, &code)
	if !code.Completed(ctx, 6) {
		t.Fatal("six of six characters did not complete the code")
	}
	if code.Completed(ctx, 6) {
		t.Error("a full code answered twice")
	}

	code.Clear()
	if code.Code() != "" {
		t.Error("Clear left characters in the code")
	}
}

func TestTheRowOfSquaresFitsTheRoomItIsGiven(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 200)
	var code widget.OneTimeCode

	if got := (widget.OneTimeCodeProps{Length: 8}).Layout(ctx, &code).Size.X; got > 200 {
		t.Errorf("eight squares took %d of 200", got)
	}
}
