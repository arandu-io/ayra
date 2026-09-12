// Package system is what the platform and the application tell each other
// about the surface between them.
//
// Two things cross it, and neither can be worked out from this side alone.
// What comes in is the language the person is reading in and which way that
// language is written. What goes back out is which parts of the drawn surface
// stand for a window action -- the strip that drags the window, the mark that
// closes it -- because an application that paints its own titlebar has painted
// a picture until it says so.
//
// The direction is the half that is easy to mistake for a detail of the text.
// It is not: it is the layout. A screen laid out without asking puts the label
// left of its field, the back arrow at the left edge and the first column where
// the eye of its reader arrives last, and it does all of that without failing
// anything -- which is why it ships.
package system

import "strings"

// Locale is what the platform says about the language the application is being
// read in.
//
// The zero value is usable and means left to right with nothing named, which is
// the right answer for a context built outside a window: a test, a tool that
// renders one frame, a catalogue drawing a control on its own. None of them
// asked a platform anything, and all of them still have to lay out. It is not a
// sentinel for "nobody said" -- there is no state here a caller must check
// before drawing, because a check is a thing one caller forgets.
//
// It is comparable, and work depends on that. Shaped text is kept until the
// locale it was shaped under stops matching the one being drawn with, and that
// comparison is this struct against itself: a field that did not compare by
// value would either reshape every paragraph on every frame or leave a person
// who changed their language reading the old direction.
type Locale struct {
	// Language is the BCP-47 tag, as the platform reported it. It travels
	// unchanged to the shaper, which is what picks the letterforms a script
	// uses in one language and not in another.
	Language string
	// Direction is which way the text runs, and with it the layout around it.
	Direction TextDirection
}

// LocaleFor answers the locale for a language tag, with the direction read out
// of the tag itself.
//
// It exists so that the direction cannot be left behind. A Locale is two fields
// and one of them has a usable zero, so a caller that filled in only the
// language would compile, run, and draw Arabic left to right: no crash, no
// missing string, and a screen that reads as finished to everybody who does not
// read the language.
//
// A script subtag decides over the language whenever the tag carries one,
// because it is the more specific answer -- a language written in more than one
// script is written in whichever the tag names. A region decides nothing:
// Arabic is Arabic in Egypt and in Morocco.
//
// A tag nothing here recognises is left to right, which is what the zero value
// says too. Guessing the other way would mirror a whole layout on a typo.
func LocaleFor(language string) Locale {
	return Locale{Language: language, Direction: directionFor(language)}
}

// directionFor reads the direction out of a language tag.
func directionFor(tag string) TextDirection {
	language, script := subtags(tag)
	if script != "" {
		if rightToLeftScripts[script] {
			return RTL
		}
		return LTR
	}
	if rightToLeftLanguages[language] {
		return RTL
	}
	return LTR
}

// subtags takes the language and the script out of a tag, and ignores the rest.
//
// Both separators are accepted because a platform handing over an underscore is
// handing over the same tag, and the comparison is made in lower case because
// the case of a tag carries no meaning and platforms disagree about it anyway.
//
// A subtag of four letters is the script; two letters or three digits is the
// region, which has no bearing here and is passed over rather than mistaken for
// one.
func subtags(tag string) (language, script string) {
	separator := func(r rune) bool { return r == '-' || r == '_' }

	for i, part := range strings.FieldsFunc(strings.ToLower(tag), separator) {
		switch {
		case i == 0:
			language = part
		case len(part) == 4 && script == "":
			script = part
		}
	}
	return language, script
}

// rightToLeftLanguages are the languages written right to left when the tag
// says nothing about the script.
//
// The set is closed. Hebrew and Yiddish appear under both of their tags,
// because a platform older than the rename still reports the old one and a
// table that knew only the new spelling would draw that platform backwards.
var rightToLeftLanguages = map[string]bool{
	"ar":  true, // Arabic
	"arc": true, // Aramaic
	"ckb": true, // Central Kurdish
	"dv":  true, // Dhivehi
	"fa":  true, // Persian
	"he":  true, // Hebrew
	"iw":  true, // Hebrew, under the tag it carried before the rename
	"ji":  true, // Yiddish, under the tag it carried before the rename
	"ks":  true, // Kashmiri
	"nqo": true, // N'Ko
	"prs": true, // Dari
	"ps":  true, // Pashto
	"sd":  true, // Sindhi
	"syr": true, // Syriac
	"ug":  true, // Uyghur
	"ur":  true, // Urdu
	"yi":  true, // Yiddish
}

// rightToLeftScripts are the scripts written right to left.
//
// A script named in the tag answers for any language, which is what makes this
// the set that decides: the same language in a Latin script is read the
// ordinary way, and the same Latin script carries no language that is not.
var rightToLeftScripts = map[string]bool{
	"adlm": true, // Adlam
	"arab": true, // Arabic
	"aran": true, // Nastaliq
	"hebr": true, // Hebrew
	"nkoo": true, // N'Ko
	"rohg": true, // Hanifi Rohingya
	"syrc": true, // Syriac
	"thaa": true, // Thaana
}

const (
	axisShift = iota
	progressionShift
)

// TextDirection is which way text runs.
//
// It is a pair of bits rather than a name per direction: an axis, and which way
// along that axis the text travels. Those are the two questions a layout asks
// -- whether to walk x or y, and from which end -- and answering them out of
// one value keeps a caller from having to know every direction there is a name
// for in order to lay one out.
type TextDirection byte

const (
	// LTR is left-to-right text, and is the zero value.
	LTR TextDirection = TextDirection(Horizontal<<axisShift) | TextDirection(FromOrigin<<progressionShift)
	// RTL is right-to-left text.
	RTL TextDirection = TextDirection(Horizontal<<axisShift) | TextDirection(TowardOrigin<<progressionShift)
)

// Axis returns the axis the text runs along.
func (d TextDirection) Axis() TextAxis {
	return TextAxis((d & (1 << axisShift)) >> axisShift)
}

// Progression returns which way along its axis the text runs, relative to the
// origin.
func (d TextDirection) Progression() TextProgression {
	return TextProgression((d & (1 << progressionShift)) >> progressionShift)
}

// String names the direction.
func (d TextDirection) String() string {
	switch d {
	case RTL:
		return "RTL"
	default:
		return "LTR"
	}
}

// TextAxis is the axis text is laid out along.
type TextAxis byte

const (
	// Horizontal is text that runs along the x axis.
	Horizontal TextAxis = iota
	// Vertical is text that runs along the y axis.
	Vertical
)

// TextProgression is which way text travels along its axis, relative to the
// origin. The origin is the upper left corner of the coordinate space, on both
// axes, whichever way the text on them runs.
type TextProgression byte

const (
	// FromOrigin is text that runs along its axis away from the origin.
	FromOrigin TextProgression = iota
	// TowardOrigin is text that runs along its axis back towards the origin.
	TowardOrigin
)
