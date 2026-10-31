package mask

import "strings"

// Pattern is a mask written in tokens.
//
// The tokens are the ones the ecosystem this borrows its vocabulary from uses,
// so somebody who has written a mask before writes the same string here:
//
//	0  a digit, required
//	9  a digit, optional
//	#  a digit, repeated to the end of the value
//	A  a letter or a digit
//	S  a letter
//
// Every other character is written through as it stands. A pattern that needs a
// literal token character is not expressible, and that is the price of the
// vocabulary being the familiar one; a mask of literal hashes is not a thing
// anybody has asked for.
type Pattern string

// token reports what a pattern character accepts, and whether it is a token at
// all.
type token struct {
	accepts   func(rune) bool
	optional  bool
	repeating bool
}

// tokens is the dictionary. It is closed: a mask that needed a sixth token
// would be a mask nobody could read without this table beside them.
var tokens = map[rune]token{
	'0': {accepts: isDigit},
	'9': {accepts: isDigit, optional: true},
	'#': {accepts: isDigit, repeating: true},
	'A': {accepts: isAlphanumeric},
	'S': {accepts: isLetter},
}

func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isAlphanumeric(r rune) bool {
	return isDigit(r) || isLetter(r)
}

// Apply formats value with this pattern, keeping only what the pattern accepts.
//
// It stops at the end of either one: a value longer than the pattern loses its
// tail, and a pattern longer than the value is filled as far as the value goes.
// A trailing separator is never left dangling -- "123." is answered as "123" --
// because a separator with nothing after it is punctuation somebody is about to
// type past, and showing it moves the caret for no reason.
//
// Characters the pattern cannot accept are dropped rather than refused. What
// somebody pastes is usually the formatted value, and refusing the punctuation
// in it would mean refusing a paste of exactly what this would have produced.
func (p Pattern) Apply(value string) string {
	var out []rune
	pattern := []rune(string(p))
	input := []rune(value)

	var at int
	for cursor := 0; cursor < len(pattern) && at < len(input); {
		symbol := pattern[cursor]
		tk, isToken := tokens[symbol]
		if !isToken {
			out = append(out, symbol)
			// A literal in the value that matches the literal in the pattern is
			// the person retyping the separator, or a paste of the formatted
			// value. It is consumed rather than counted twice.
			if input[at] == symbol {
				at++
			}
			cursor++
			continue
		}

		if !tk.accepts(input[at]) {
			at++
			continue
		}
		out = append(out, input[at])
		at++
		if !tk.repeating {
			cursor++
		}
	}

	// A separator with nothing behind it is punctuation somebody is about to
	// type past, and showing it moves the caret for no reason.
	for len(out) > 0 && !isAlphanumeric(out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return string(out)
}

// Unmask gives back what a program stores: the characters the tokens accept,
// with everything the pattern writes through removed.
//
// It is the inverse of Apply for any value Apply produced, and it is safe on a
// value that was never masked: unmasking twice answers the same thing as
// unmasking once.
func (p Pattern) Unmask(value string) string {
	var out strings.Builder
	pattern := []rune(string(p))
	input := []rune(value)

	var at int
	for cursor := 0; cursor < len(pattern) && at < len(input); {
		symbol := pattern[cursor]
		tk, isToken := tokens[symbol]
		if !isToken {
			if input[at] == symbol {
				at++
			}
			cursor++
			continue
		}
		if !tk.accepts(input[at]) {
			at++
			continue
		}
		out.WriteRune(input[at])
		at++
		if !tk.repeating {
			cursor++
		}
	}
	return out.String()
}

// Complete reports whether value fills every required token.
//
// Required, so a pattern whose tail is optional is complete without it:
// 00000-999 is complete at five characters. A repeating token needs one.
//
// It answers about the shape and nothing else. Eleven digits are not a CPF --
// the check digits are arithmetic, and this package does no arithmetic.
func (p Pattern) Complete(value string) bool {
	raw := []rune(p.Unmask(value))

	var required int
	for _, symbol := range string(p) {
		tk, isToken := tokens[symbol]
		if !isToken || tk.optional {
			continue
		}
		required++
		if tk.repeating {
			break
		}
	}
	return len(raw) >= required
}

// Accepts reports whether value is one this pattern can produce: the formatted
// value, which Apply leaves as it is, or the raw value, which Unmask leaves as
// it is. Every character it holds is one the pattern would have kept, in a
// place the pattern would have put it, and there are no more than the pattern
// takes.
//
// Both, because a form sends either. The browser's script posts the raw value,
// and the same form with no script posts what was typed, punctuation included
// when somebody typed it -- so 01310100 and 01310-100 are both accepted by
// 00000-000, while 0131-0100, with its separator in a place the pattern never
// writes one, is not. Unmask turns whichever arrived into what is stored.
//
// Complete asks whether enough was typed; this asks whether what was typed
// belongs. A caller validating a submitted field wants both.
func (p Pattern) Accepts(value string) bool {
	return value == p.Apply(value) || value == p.Unmask(value)
}

// Capacity is how many characters the tokens accept, and -1 for a pattern that
// repeats and therefore has no end.
func (p Pattern) Capacity() int {
	var n int
	for _, symbol := range string(p) {
		tk, isToken := tokens[symbol]
		if !isToken {
			continue
		}
		if tk.repeating {
			return -1
		}
		n++
	}
	return n
}
