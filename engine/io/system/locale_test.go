package system_test

import (
	"testing"

	"github.com/arandu-io/ayra/engine/io/system"
)

// TestTheZeroLocaleDrawsRatherThanRefusing is what keeps this a value and not a
// state a caller has to check.
//
// Everything that lays out text is handed a locale, and most of the things that
// lay out text never asked a platform anything: a test, a catalogue rendering
// one frame, a tool measuring a label. All of them pass the zero value, so the
// zero value has to be a direction a screen can be drawn with. Were it a
// sentinel meaning "nobody said", every caller would have to answer for it and
// one of them would forget -- and the one that forgot would draw nothing.
func TestTheZeroLocaleDrawsRatherThanRefusing(t *testing.T) {
	var zero system.Locale

	if zero.Direction != system.LTR {
		t.Errorf("the zero locale reads %v; it has to be a direction a screen can be drawn with", zero.Direction)
	}
	if got := zero.Direction.Axis(); got != system.Horizontal {
		t.Errorf("the zero locale runs along axis %d, want %d, the horizontal one", got, system.Horizontal)
	}
	if got := zero.Direction.Progression(); got != system.FromOrigin {
		t.Errorf("the zero locale runs %d, want %d, away from the origin", got, system.FromOrigin)
	}
	if zero.Language != "" {
		t.Errorf("the zero locale names the language %q, and it has nothing to name it from", zero.Language)
	}
}

// TestAKnownLanguageDecidesWhichWayItIsWritten is the bug this package exists
// to stop.
//
// The direction is not a detail of the text: it is the layout. A screen built
// without it puts the label to the left of the field, the back arrow on the
// left, the first column at the left edge -- and for a reader of Arabic, Hebrew
// or Persian every one of those is on the wrong side. Nothing crashes and no
// string is missing, so it reads as finished to everybody who does not read the
// language.
func TestAKnownLanguageDecidesWhichWayItIsWritten(t *testing.T) {
	for _, c := range []struct {
		tag  string
		want system.TextDirection
	}{
		{"en", system.LTR},
		{"pt", system.LTR},
		{"ja", system.LTR},
		{"ar", system.RTL},
		{"he", system.RTL},
		// The tag Hebrew carried before it was renamed. A platform older than
		// the rename still reports it, and a table that knew only the new
		// spelling would draw that platform the wrong way round.
		{"iw", system.RTL},
		{"fa", system.RTL},
		{"ur", system.RTL},
		// A region is not an answer: Arabic is Arabic in Egypt, and a tag
		// carrying one must not undo what the language already said.
		{"ar-EG", system.RTL},
		{"pt-BR", system.LTR},
		// Case is not part of the answer, and neither is which of the two
		// separators the platform happened to hand over.
		{"AR", system.RTL},
		{"ar_EG", system.RTL},
		// The script is the more specific answer. A language written in more
		// than one is written in whichever the tag names.
		{"az-Arab", system.RTL},
		{"az", system.LTR},
		{"ar-Latn", system.LTR},
		// Nothing recognised, and nothing to go on. Mirroring a screen on a
		// tag nobody can read would be a guess with a whole layout riding on
		// it.
		{"", system.LTR},
		{"zz", system.LTR},
	} {
		t.Run(c.tag, func(t *testing.T) {
			locale := system.LocaleFor(c.tag)

			if locale.Direction != c.want {
				t.Errorf("%q is written %v, want %v", c.tag, locale.Direction, c.want)
			}
			if locale.Language != c.tag {
				t.Errorf("the language came back as %q, and it was given as %q", locale.Language, c.tag)
			}
		})
	}
}

// TestADirectionCarriesItsAxisAndItsProgression checks the two questions a
// layout actually asks, on both of the directions there are names for.
//
// They are asked separately, so both have to be right separately: text that
// reported the correct axis and the wrong progression would run along the line
// it belongs on and start from the wrong end of it.
func TestADirectionCarriesItsAxisAndItsProgression(t *testing.T) {
	for _, c := range []struct {
		direction   system.TextDirection
		axis        system.TextAxis
		progression system.TextProgression
		name        string
	}{
		{system.LTR, system.Horizontal, system.FromOrigin, "LTR"},
		{system.RTL, system.Horizontal, system.TowardOrigin, "RTL"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.direction.Axis(); got != c.axis {
				t.Errorf("axis %d, want %d", got, c.axis)
			}
			if got := c.direction.Progression(); got != c.progression {
				t.Errorf("progression %d, want %d", got, c.progression)
			}
			if got := c.direction.String(); got != c.name {
				t.Errorf("named %q, want %q", got, c.name)
			}
		})
	}

	if system.LTR == system.RTL {
		t.Fatal("the two directions are one value, so nothing can tell them apart")
	}
}

// TestTwoLocalesThatSayTheSameThingAreEqual is relied on by the thing that
// decides when to shape text again.
//
// An editor keeps the locale it last shaped under and compares it with the one
// it is handed this frame, reshaping only when the two differ. That comparison
// is this struct against itself, so it has to hold in both directions: a locale
// that compared unequal to an identical one would reshape every paragraph on
// every frame, and one that compared equal to a different one would leave a
// person who changed their language reading the old direction until something
// else forced the work.
func TestTwoLocalesThatSayTheSameThingAreEqual(t *testing.T) {
	if a, b := system.LocaleFor("ar"), system.LocaleFor("ar"); a != b {
		t.Errorf("%v and %v were built from the same tag and did not compare equal", a, b)
	}
	if a, b := system.LocaleFor("ar"), system.LocaleFor("he"); a == b {
		t.Errorf("%v and %v are different languages and compared equal", a, b)
	}
	if a, b := system.LocaleFor("ar"), system.LocaleFor("en"); a == b {
		t.Errorf("%v and %v are written opposite ways and compared equal", a, b)
	}

	var zero system.Locale
	if zero != system.LocaleFor("") {
		t.Error("the zero locale and the one built from no tag are two different values")
	}

	// A locale is also used as a key, which is the same requirement asked
	// harder: a type that is not comparable does not compile here at all.
	seen := map[system.Locale]int{}
	seen[system.LocaleFor("ar")]++
	seen[system.LocaleFor("ar")]++
	if seen[system.LocaleFor("ar")] != 2 {
		t.Errorf("two equal locales landed on %d keys, want one", len(seen))
	}
}
