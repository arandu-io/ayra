package mask_test

import (
	"testing"

	"github.com/arandu-io/ayra/mask"
	"github.com/arandu-io/ayra/mask/brazil"
)

func TestTheBrazilianShapes(t *testing.T) {
	for _, c := range []struct {
		name  string
		mask  mask.Mask
		typed string
		shown string
		kept  string
	}{
		{"cpf", brazil.CPF, "12345678909", "123.456.789-09", "12345678909"},
		{"cpf pasted formatted", brazil.CPF, "123.456.789-09", "123.456.789-09", "12345678909"},
		{"cnpj", brazil.CNPJ, "12345678000195", "12.345.678/0001-95", "12345678000195"},
		{"document as a cpf", brazil.Document, "12345678909", "123.456.789-09", "12345678909"},
		{"document as a cnpj", brazil.Document, "12345678000195", "12.345.678/0001-95", "12345678000195"},
		{"landline", brazil.Phone, "1133334444", "(11) 3333-4444", "1133334444"},
		{"mobile", brazil.Phone, "11987654321", "(11) 98765-4321", "11987654321"},
		{"cep", brazil.CEP, "01310100", "01310-100", "01310100"},
		{"old plate", brazil.Plate, "abc1234", "abc1234", "abc1234"},
		{"mercosul plate", brazil.Plate, "ABC1D23", "ABC1D23", "ABC1D23"},
		{"date", mask.Date, "10092026", "10/09/2026", "10092026"},
		{"time", mask.Time, "0930", "09:30", "0930"},
		{"real", brazil.Real, "123456", "R$ 1.234,56", "123456"},
		{"real, cents", brazil.Real, "5", "R$ 0,05", "5"},
		{"real, nothing", brazil.Real, "", "", ""},
		{"real, formatted", brazil.Real, "R$ 1.234,56", "R$ 1.234,56", "123456"},
	} {
		if got := c.mask.Apply(c.typed); got != c.shown {
			t.Errorf("%s: Apply(%q) = %q, want %q", c.name, c.typed, got, c.shown)
		}
		if got := c.mask.Unmask(c.typed); got != c.kept {
			t.Errorf("%s: Unmask(%q) = %q, want %q", c.name, c.typed, got, c.kept)
		}
	}
}

func TestAPlateGrowsIntoTheMercosulShape(t *testing.T) {
	if !brazil.Plate.Complete("ABC1D23") || !brazil.Plate.Complete("ABC1234") {
		t.Error("both plate shapes must fill the pattern")
	}
	if brazil.Plate.Complete("ABC1D2") {
		t.Error("six characters are not a plate")
	}
	if got := brazil.Plate.Apply("ab1"); got != "ab" {
		t.Errorf("a digit where a letter goes was kept: %q", got)
	}
}

func TestMoneyKeepsTheCentsAsDigits(t *testing.T) {
	// Typing 1, 2, 3, 4, 5 one key at a time, the way a field is fed.
	shown := ""
	for _, key := range "12345" {
		shown = brazil.Real.Apply(shown + string(key))
	}
	if shown != "R$ 123,45" {
		t.Errorf("five keys typed show %q, want R$ 123,45", shown)
	}
	if kept := brazil.Real.Unmask(shown); kept != "12345" {
		t.Errorf("what is stored is %q, want 12345: the amount in centavos", kept)
	}
}

func TestAlternativesAnswerCompleteness(t *testing.T) {
	if brazil.Phone.Complete("(11) 3333-444") {
		t.Error("nine digits are not a phone number")
	}
	if !brazil.Phone.Complete("1133334444") {
		t.Error("ten digits are a landline")
	}
	if !brazil.Phone.Accepts("11987654321") || brazil.Phone.Accepts("119876543210") {
		t.Error("Accepts must take either shape and nothing longer")
	}
}
