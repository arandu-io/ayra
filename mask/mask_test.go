package mask_test

import (
	"testing"

	"github.com/arandu-io/ayra/mask"
)

const postcode mask.Pattern = "00000-000"

// TestAcceptsTheRawValueTheBehaviourPosts: once the browser's script has run,
// the field that submits carries the raw value. Accepts once compared a value
// with its own formatting, so 01310100 was refused by 00000-000 -- and a server
// validating the field the documented way refused every value the script sent.
func TestAcceptsTheRawValueTheBehaviourPosts(t *testing.T) {
	t.Parallel()

	if !postcode.Accepts("01310100") {
		t.Fatal(`Accepts("01310100") = false: the raw value is what the script posts, so every submitted postcode would be refused`)
	}
}

// TestAcceptsTheFormattedValueAFormPostsWithoutScript: with no script, the
// form posts the visible box as it was typed, and somebody types the
// separator.
func TestAcceptsTheFormattedValueAFormPostsWithoutScript(t *testing.T) {
	t.Parallel()

	if !postcode.Accepts("01310-100") {
		t.Fatal(`Accepts("01310-100") = false: the formatted value is what a form posts when no script ran`)
	}
}

// TestAcceptsRefusesWhatThePatternCannotProduce keeps taking both forms from
// becoming taking anything that unmasks to the right length.
func TestAcceptsRefusesWhatThePatternCannotProduce(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"0131-0100",  // a separator in a place the pattern never writes one
		"01310--100", // the separator twice
		"01310 100",  // a separator the pattern does not have
		"013101001",  // one digit more than the pattern takes
		"01310-1000", // the same, formatted
		"0131O100",   // a letter where a digit goes
		"01310-",     // a separator with nothing after it, which Apply never leaves
		"abc",
	} {
		if postcode.Accepts(value) {
			t.Errorf("Accepts(%q) = true, and %s never writes that value nor unmasks to it", value, postcode)
		}
	}
}

// TestAcceptsTheRawValueOfAPatternThatOpensWithALiteral: the raw value of
// (00) 0000-0000 has none of the punctuation, the leading parenthesis included.
func TestAcceptsTheRawValueOfAPatternThatOpensWithALiteral(t *testing.T) {
	t.Parallel()

	phone := mask.Pattern("(00) 0000-0000")
	for _, value := range []string{"1198765432", "(11) 9876-5432"} {
		if !phone.Accepts(value) {
			t.Errorf("Accepts(%q) = false for %s", value, phone)
		}
	}
	if phone.Accepts("(11)9876-5432") {
		t.Errorf("Accepts(%q) = true, and %s always writes the space", "(11)9876-5432", phone)
	}
}

// TestEveryValueAFormCanSendIsAccepted is the property the two tests above are
// examples of: whatever somebody types, the box shows Apply of it and the
// script posts Unmask of that, and both pass Accepts -- for every pattern this
// package names and for the shapes an application declares.
//
// A pattern that writes through a character its own tokens take -- the 1 in
// +1 000 -- is not in the list. There a raw value that begins with that
// character cannot be told from the formatted one, and Apply and Unmask both
// read it as the literal, so the raw value 123 unmasks to 23.
func TestEveryValueAFormCanSendIsAccepted(t *testing.T) {
	t.Parallel()

	patterns := []mask.Pattern{
		mask.Date, mask.DateISO, mask.Time, mask.TimeSeconds,
		mask.CreditCard, mask.CardExpiry, mask.CardSecurity,
		postcode, "000.000.000-00", "00.000.000/0000-00",
		"(00) 0000-0000", "(00) 00000-0000", "SSS-0A00", "0.#",
	}
	typed := []string{
		"", "1", "12", "1234", "12345678", "123456789012345678901234",
		"01310-100", "12/31/2026", "abc123XYZ", "a1b2c3d4", "+1 555",
		"155", "(11) 98765-4321", "1-2.3/4 5", "ABC1D23",
	}
	for _, pattern := range patterns {
		for _, input := range typed {
			for end := 0; end <= len(input); end++ {
				shown := pattern.Apply(input[:end])
				posted := pattern.Unmask(shown)
				if !pattern.Accepts(shown) {
					t.Errorf("%s: Accepts(%q) = false, and it is what the box shows for %q", pattern, shown, input[:end])
				}
				if !pattern.Accepts(posted) {
					t.Errorf("%s: Accepts(%q) = false, and it is what the script posts for %q", pattern, posted, input[:end])
				}
			}
		}
	}
}

// TestUnmaskAndCompleteAnswerTheSameForBothForms: what is stored and whether
// enough was typed cannot depend on whether the script ran.
func TestUnmaskAndCompleteAnswerTheSameForBothForms(t *testing.T) {
	t.Parallel()

	for _, pair := range [][2]string{
		{"01310100", "01310-100"},
		{"0131", "0131"},
		{"013101", "01310-1"},
	} {
		raw, formatted := pair[0], pair[1]
		if a, b := postcode.Unmask(raw), postcode.Unmask(formatted); a != b {
			t.Errorf("Unmask(%q) = %q, Unmask(%q) = %q: what is stored depends on whether the script ran", raw, a, formatted, b)
		}
		if a, b := postcode.Complete(raw), postcode.Complete(formatted); a != b {
			t.Errorf("Complete(%q) = %v, Complete(%q) = %v", raw, a, formatted, b)
		}
	}
}
