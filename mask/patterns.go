package mask

// The patterns this package names.
//
// They are spellings and not rules: a pattern says how many characters of what
// kind, with what between them, and says nothing about whether the value means
// anything. A shape is what a mask can carry.
//
// # None of them belongs to a country
//
// A national document is a rule before it is a shape -- a CPF is eleven digits
// AND an arithmetic check, and a mask that named the first without the second
// would be a framework implying it validates something it cannot. The
// arithmetic belongs to whoever owns the document, which is the application,
// and so does the pattern that goes with it:
//
//	const CPF mask.Pattern = "000.000.000-00"
//
// A pattern is a string. Naming one costs an application one line and keeps the
// framework from having an opinion about which countries exist.
//
// What is named here is what has no nationality: a date is a date everywhere,
// and a card is sixteen digits in every country that issues one.
const (
	// Date is a day as most of the world writes it.
	Date Pattern = "00/00/0000"
	// DateISO is a day as a database writes it.
	DateISO Pattern = "0000-00-00"
	// Time is hours and minutes.
	Time Pattern = "00:00"
	// TimeSeconds is hours, minutes and seconds.
	TimeSeconds Pattern = "00:00:00"
	// CreditCard is the sixteen digits most cards carry.
	CreditCard Pattern = "0000 0000 0000 0000"
	// CardExpiry is the month and year on the front of a card.
	CardExpiry Pattern = "00/00"
	// CardSecurity is the code on the back, which is three digits or four.
	CardSecurity Pattern = "0009"
)

// Alternatives is a mask that picks by how much was typed.
//
// One field that takes either of two shapes -- a phone number of ten digits or
// eleven, a document that is one kind or another -- cannot be one pattern. The
// shortest pattern that still holds the value is the one used, so a field grows
// into the longer one as somebody types rather than jumping into it.
//
// This is the type an application reaches for when its country has two shapes
// for one field:
//
//	var Phone = mask.Alternatives{"(00) 0000-0000", "(00) 00000-0000"}
type Alternatives []Pattern

// For is the pattern that fits this value: the shortest whose capacity holds
// it, and the widest when none does.
//
// The widest when none does, so a value past every alternative is still
// formatted rather than dropped -- the extra characters fall off the end, which
// is what somebody typing one too many expects to see.
func (a Alternatives) For(value string) Pattern {
	if len(a) == 0 {
		return ""
	}
	widest := a[0]
	for _, pattern := range a {
		if pattern.Capacity() > widest.Capacity() {
			widest = pattern
		}
	}

	raw := len([]rune(widest.Unmask(value)))
	best, found := Pattern(""), false
	for _, pattern := range a {
		size := pattern.Capacity()
		if size < 0 || size < raw {
			continue
		}
		if !found || size < best.Capacity() {
			best, found = pattern, true
		}
	}
	if !found {
		return widest
	}
	return best
}

// Apply formats value with whichever alternative fits it.
func (a Alternatives) Apply(value string) string { return a.For(value).Apply(value) }

// Unmask gives back what a program stores.
func (a Alternatives) Unmask(value string) string { return a.For(value).Unmask(value) }

// Strings is the alternatives as the browser reads them.
func (a Alternatives) Strings() []string {
	out := make([]string, 0, len(a))
	for _, pattern := range a {
		out = append(out, string(pattern))
	}
	return out
}
