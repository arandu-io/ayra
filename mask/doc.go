// Package mask formats a value as somebody reads it and gives back what a
// program stores.
//
// A mask is a pattern of tokens: 000.000.000-00 is eleven digits with
// punctuation between them. Applying it turns 12345678900 into
// 123.456.789-00; unmasking turns either of them back into 12345678900.
//
// # One pattern, both sides
//
// The pattern is the whole contract, and it is shared: the browser formats what
// somebody types with it, and the server unmasks and checks with the same
// value. A formatting rule written twice is a formatting rule that disagrees
// with itself the first time one copy is edited -- and what reaches the
// database when they disagree is punctuation.
//
// So the browser sends the raw value, not the formatted one, and this package
// is what the server uses to decide whether that value fits.
//
// # What a form can send
//
// The raw value is what arrives once the browser's script has run. Until it
// has -- scripting off, the script blocked or not loaded yet -- the form posts
// the visible box as it stands, and that is the value as it was typed, which
// may be the formatted one. A server cannot tell the two apart and should not
// have to, so the checks here take either: Accepts answers yes to 12345678900
// and to 123.456.789-00 alike, Complete counts the same characters in both,
// and Unmask turns both into 12345678900, which is what gets stored.
//
// # What it does not do
//
// It does not validate. 000.000.000-00 accepts eleven digits, and eleven digits
// are not a CPF -- the check digits are arithmetic, and arithmetic about a
// country's documents belongs to whoever owns that domain. Complete answers
// whether the pattern is filled, and that is the whole of what a shape can say.
//
// It has no opinion about locale beyond the named patterns below, which are
// spellings and not rules.
package mask
