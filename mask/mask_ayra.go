package mask

import (
	"strings"
)

// What is in this file is Ayra's and not hesape's. The rest of the package is a
// copy of hesape/mask, kept identical by a test, so that the rule that formats
// what somebody types on the server side and the rule that formats it in a
// window are one rule and cannot disagree.
//
// It is a copy and not an import because this module may not require hesape: the
// drawing half of Arandu carries no server. The right home for a rule both halves
// share is a leaf module neither of them owns; until it exists, the copy and the
// test are how they stay one.

// Mask is anything that can format what somebody types and give back what a
// program stores. A [Pattern] is one, so is [Alternatives], and so is [Money];
// a product's own mask -- a licence plate that grows from the old shape into
// the Mercosul one, a code that has check digits -- is a type with these three
// methods.
type Mask interface {
	// Apply formats value as somebody reads it.
	Apply(value string) string
	// Unmask gives back what a program stores, from the formatted value or from
	// the raw one.
	Unmask(value string) string
	// Complete reports whether enough was typed. It answers about the shape and
	// nothing else.
	Complete(value string) bool
}

var (
	_ Mask = Pattern("")
	_ Mask = Alternatives(nil)
	_ Mask = Money{}
)

// Complete reports whether value fills the alternative that fits it.
func (a Alternatives) Complete(value string) bool { return a.For(value).Complete(value) }

// Accepts reports whether value is one an alternative can produce.
func (a Alternatives) Accepts(value string) bool {
	for _, pattern := range a {
		if pattern.Accepts(value) {
			return true
		}
	}
	return false
}

// Money is a mask for an amount of money, typed the way a cash register takes
// it: the digits are the cents, and each one pushes the earlier ones to the left.
// Typing 1, 2, 3, 4, 5 shows R$ 0,01, R$ 0,12, R$ 1,23, R$ 12,34 and R$ 123,45.
//
// It is the one mask a [Pattern] cannot be, because the position of a digit in a
// money field depends on how many follow it, and a pattern is read from the
// left. What a program stores is the digits, which are the amount in the
// smallest unit: 12345 for R$ 123,45, and no floating point anywhere.
type Money struct {
	// Symbol is written before the amount, with a space after it: "R$".
	// Empty writes none.
	Symbol string
	// Decimals is how many digits are after the separator. Zero is whole
	// units, and the default of the zero Money, which is why [Real] names two.
	Decimals int
	// Thousands separates groups of three digits in the integer part. Empty
	// writes none.
	Thousands string
	// Decimal separates the integer part from the decimals. Empty is ",".
	Decimal string
}

// Apply formats the digits of value as an amount. A value with no digit is
// answered empty, so that a field left alone shows its placeholder and not
// R$ 0,00.
func (m Money) Apply(value string) string {
	digits := m.Unmask(value)
	if digits == "" {
		return ""
	}

	decimal := m.Decimal
	if decimal == "" {
		decimal = ","
	}

	for len(digits) <= m.Decimals {
		digits = "0" + digits
	}
	whole, cents := digits[:len(digits)-m.Decimals], digits[len(digits)-m.Decimals:]

	if m.Thousands != "" {
		var groups []string
		for len(whole) > 3 {
			groups = append([]string{whole[len(whole)-3:]}, groups...)
			whole = whole[:len(whole)-3]
		}
		groups = append([]string{whole}, groups...)
		whole = strings.Join(groups, m.Thousands)
	}

	out := whole
	if m.Decimals > 0 {
		out += decimal + cents
	}
	if m.Symbol != "" {
		out = m.Symbol + " " + out
	}
	return out
}

// Unmask gives back the amount in the smallest unit, as digits, without leading
// zeros: "R$ 1.234,56" and "123456" are both 123456.
//
// A formatted value and a raw one are told apart by the symbol: what comes after
// the symbol is the formatted amount, whose separators are dropped, and a value
// with no symbol is taken as the digits it is.
func (m Money) Unmask(value string) string {
	if m.Symbol != "" {
		value = strings.TrimPrefix(strings.TrimSpace(value), m.Symbol)
	}
	var out strings.Builder
	for _, r := range value {
		if isDigit(r) {
			out.WriteRune(r)
		}
	}
	return strings.TrimLeft(out.String(), "0")
}

// Complete reports whether an amount was typed at all. Whether it is an amount
// worth charging is the application's question.
func (m Money) Complete(value string) bool { return m.Unmask(value) != "" }
