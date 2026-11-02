package widget

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// RelativeTimeProps is a moment written the way a person says it: "3 minutes ago",
// "in 2 days" -- the counterpart of Kyse's RelativeTime.
//
// The moment it is relative to is passed in, for the reason every date control
// here takes its day as an argument: a control that read the clock would draw a
// different picture every second, and a test could not pin one. A screen passes
// the frame's time and the text moves with it.
type RelativeTimeProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root", whose foreground and font are the text's.
	ComponentProps
	// At is the moment.
	At time.Time
	// Now is what it is relative to. Zero is a mistake the control refuses to
	// guess at, and it draws At's date.
	Now time.Time
	// Words are the sentences, in the application's language. Zero values draw
	// English.
	Words RelativeTimeWords
}

// RelativeTimeWords are the sentences of a relative time. Each is a template with
// {n} where the number goes; the singular is used for one.
type RelativeTimeWords struct {
	JustNow         string
	Second, Seconds string
	Minute, Minutes string
	Hour, Hours     string
	Day, Days       string
	Week, Weeks     string
	Month, Months   string
	Year, Years     string
	Past, Future    string // "{} ago", "in {}"
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p RelativeTimeProps) PartNames() []string { return []string{"root"} }

// Text is the sentence, for a caller that wants the words and not the drawing.
func (p RelativeTimeProps) Text() string {
	if p.Now.IsZero() {
		return p.At.Format(DateFormat)
	}
	d := p.Now.Sub(p.At)
	past := d >= 0
	if !past {
		d = -d
	}
	w := p.Words

	var unit string
	n := 0
	switch {
	case d < 10*time.Second:
		return orDefault(w.JustNow, "just now")
	case d < time.Minute:
		n, unit = int(d/time.Second), pick(w.Second, w.Seconds, "{n} second", "{n} seconds", int(d/time.Second))
	case d < time.Hour:
		n, unit = int(d/time.Minute), pick(w.Minute, w.Minutes, "{n} minute", "{n} minutes", int(d/time.Minute))
	case d < 24*time.Hour:
		n, unit = int(d/time.Hour), pick(w.Hour, w.Hours, "{n} hour", "{n} hours", int(d/time.Hour))
	case d < 7*24*time.Hour:
		n, unit = int(d/(24*time.Hour)), pick(w.Day, w.Days, "{n} day", "{n} days", int(d/(24*time.Hour)))
	case d < 30*24*time.Hour:
		n, unit = int(d/(7*24*time.Hour)), pick(w.Week, w.Weeks, "{n} week", "{n} weeks", int(d/(7*24*time.Hour)))
	case d < 365*24*time.Hour:
		n, unit = int(d/(30*24*time.Hour)), pick(w.Month, w.Months, "{n} month", "{n} months", int(d/(30*24*time.Hour)))
	default:
		n, unit = int(d/(365*24*time.Hour)), pick(w.Year, w.Years, "{n} year", "{n} years", int(d/(365*24*time.Hour)))
	}
	unit = strings.ReplaceAll(unit, "{n}", fmt.Sprint(n))

	frame := orDefault(w.Future, "in {}")
	if past {
		frame = orDefault(w.Past, "{} ago")
	}
	return strings.ReplaceAll(frame, "{}", unit)
}

// pick is the singular for one and the plural for the rest, each in the
// application's words or the English fallback.
func pick(singular, plural, defaultSingular, defaultPlural string, n int) string {
	if n == 1 {
		return orDefault(singular, defaultSingular)
	}
	return orDefault(plural, defaultPlural)
}

// Layout draws the sentence and returns the room it took.
func (p RelativeTimeProps) Layout(c ayra.Context) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve("relativetime", "root", p.VariantName("default"), theme.Style{
		Foreground: theme.Colour(t.Colours.MutedForeground),
		Font:       theme.Font{Size: theme.Pt(t.Type.Small)},
	}, p.ComponentProps)
	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		return drawPart(c, p.Text(), style, unit.Sp(t.Type.Small), 1, text.Start, plain())
	})
}

// LoadMore is the state of the control at the foot of a list that asks for the
// next page.
type LoadMore struct {
	button Button
}

// Clicked reports a press, once. It is asked before the control is drawn.
func (l *LoadMore) Clicked(c ayra.Context) bool { return l.button.Clicked(c) }

// LoadMoreProps is the button that loads more of a list -- the counterpart of
// Kyse's LoadMore. While the next page is on its way it says so and answers
// nothing, so that a second press cannot ask for the same page twice.
type LoadMoreProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes the part of a [ButtonProps].
	ComponentProps
	// Label is what it says, and Loading what it says while the page is coming.
	Label, Loading string
	// IsLoading is whether the next page is on its way.
	IsLoading bool
	// Done draws nothing: there is no more to load.
	Done bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p LoadMoreProps) PartNames() []string { return []string{"root"} }

// Layout draws the button and returns the room it took.
func (p LoadMoreProps) Layout(c ayra.Context, state *LoadMore) ayra.Dimensions {
	if p.Done {
		return ayra.Dimensions{}
	}
	label := orDefault(p.Label, "Load more")
	if p.IsLoading {
		label = orDefault(p.Loading, "Loading...")
	}
	// The part is resolved here, under this widget's name, and handed to the
	// button as its own root style: a restyle of "loadmore" reaches the button
	// without the caller having to know a button is what draws it.
	root := c.Theme.Resolve("loadmore", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	return ButtonProps{
		ComponentProps: ComponentProps{Style: root}, Label: label, Variant: Outline, Disabled: p.IsLoading,
	}.Layout(c, &state.button)
}

// EditInPlace is the state of a text that becomes a field when it is pressed.
type EditInPlace struct {
	editing bool
	input   Input
	open    Button
	save    Button
	cancel  Button
	saved   bool
	value   string
}

// Edit opens the field, for a screen that opens it by some other means than a
// press on the text.
func (e *EditInPlace) Edit(value string) {
	e.editing = true
	e.input.SetText(value)
}

// Editing reports whether the field is open.
func (e *EditInPlace) Editing() bool { return e.editing }

// Saved reports the text that was saved, once, and consumes it. The second
// result is false when nothing was.
func (e *EditInPlace) Saved() (string, bool) {
	if !e.saved {
		return "", false
	}
	e.saved = false
	return e.value, true
}

// EditInPlaceProps is a text that is edited where it stands -- the counterpart of
// Kyse's EditInPlace. Pressing it opens a field holding the text, return saves,
// and the cancel button puts the old text back. What saving means is the
// screen's: the new text is reported and the caller decides what to do with it.
type EditInPlaceProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root".
	ComponentProps
	// Value is the text as it stands, and what the field opens holding.
	Value string
	// Placeholder is shown in the field while it is empty, and in place of the
	// text when there is none.
	Placeholder string
	// EditLabel, SaveLabel and CancelLabel are the three buttons. EditLabel is
	// the word beside the text, as Kyse draws it; empty says "Edit".
	EditLabel, SaveLabel, CancelLabel string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p EditInPlaceProps) PartNames() []string { return []string{"root"} }

// Layout draws the text, or the field while it is open.
func (p EditInPlaceProps) Layout(c ayra.Context, state *EditInPlace) ayra.Dimensions {
	t := c.Theme

	if state.open.Clicked(c) && !state.editing {
		state.editing = true
		state.input.SetText(p.Value)
	}
	if state.editing {
		if state.input.Submitted(c) || state.save.Clicked(c) {
			state.editing, state.saved, state.value = false, true, state.input.Text()
		}
		if state.cancel.Clicked(c) {
			state.editing = false
		}
	}

	root := t.Resolve("editinplace", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		if !state.editing {
			return state.open.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						shown, ink := p.Value, t.Colours.Foreground
						if shown == "" {
							shown, ink = p.Placeholder, t.Colours.MutedForeground
						}
						inner := c.With(gtx)
						inner.Constraints.Min = layout.Constraints{}.Min
						return drawText(inner, shown, unit.Sp(t.Type.Body), ink, 1, text.Start, plain())
					}),
					layout.Rigid(layout.Spacer{Width: 6}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return drawText(c.With(gtx), orDefault(p.EditLabel, "Edit"), unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, text.Start, plain())
					}),
				)
			})
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return InputProps{Placeholder: p.Placeholder}.Layout(c.With(gtx), &state.input)
			}),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = layout.Constraints{}.Min
				return ButtonProps{Label: orDefault(p.SaveLabel, "Save"), Size: Small}.Layout(inner, &state.save)
			}),
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = layout.Constraints{}.Min
				return ButtonProps{Label: orDefault(p.CancelLabel, "Cancel"), Size: Small, Variant: Ghost}.Layout(inner, &state.cancel)
			}),
		)
	})
}

// InputGroupProps is a row of controls that share one border -- a field with a
// button at its end, a prefix before a number -- the counterpart of Kyse's
// InputGroup. It draws the border and the corners once, around whatever the
// caller puts in the row; the controls inside are drawn with [GroupedParts] so
// that they do not draw their own.
type InputGroupProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root", the shared border.
	ComponentProps
	// Focused draws the group's border as the focus ring does. The group cannot
	// know that a control inside it holds the keyboard, and the controls inside
	// draw no border of their own, so the caller says it: [Input.Focused] and
	// [Masked.Focused] answer it.
	Focused bool
	// Invalid draws the group's border as rejected.
	Invalid bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p InputGroupProps) PartNames() []string { return []string{"root"} }

// InputAddonProps is a prefix or a suffix of an [InputGroupProps]: a currency
// sign before an amount, a domain after a name -- Kyse's "addon". It is the text
// in the muted ink with the room Kyse gives it at either side, which a bare text
// put in the group had not: the text touched the group's border.
type InputAddonProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root".
	ComponentProps
	// Text is what is written.
	Text string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p InputAddonProps) PartNames() []string { return []string{"root"} }

// Layout draws the addon and returns the room it took.
func (p InputAddonProps) Layout(c ayra.Context) ayra.Dimensions {
	style := c.Theme.Resolve("inputaddon", "root", p.VariantName("default"), theme.Style{
		Foreground: theme.Colour(c.Theme.Colours.MutedForeground),
		Padding:    theme.InsetXY(12, 0),
	}, p.ComponentProps)
	c.Constraints.Min = image.Point{}
	return insetOf(style.Padding).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return drawPart(c.With(gtx), p.Text, style, unit.Sp(c.Theme.Type.Body), 1, text.Start, plain())
	})
}

// GroupedParts is the style a control takes inside a group: no border, no
// background, no corners of its own.
func GroupedParts() ComponentProps {
	// Focus and invalid are bare too: a control that held the keyboard drew a
	// square ring inside the group's rounded one.
	bare := theme.Style{Border: theme.Transparent, Background: theme.Transparent, Radius: theme.Pt(0)}
	return ComponentProps{Parts: theme.Parts{"root": bare, "focus": bare, "invalid": bare}}
}

// Layout draws the group. The first widget takes the room the others leave.
func (p InputGroupProps) Layout(c ayra.Context, first ayra.Widget, rest ...ayra.Widget) ayra.Dimensions {
	t := c.Theme
	root := t.Resolve("inputgroup", "root", p.VariantName("default"), theme.Style{
		Background: fieldFill(t),
		Border:     theme.Colour(t.Colours.Input),
		Radius:     theme.Pt(t.Metrics.Radius),
	}, p.ComponentProps)
	switch {
	case p.Invalid && !p.ComponentProps.Parts["root"].Border.IsSet():
		root.Border = theme.Colour(t.Colours.Destructive)
	case p.Focused && !p.ComponentProps.Parts["root"].Border.IsSet():
		root.Border = theme.Colour(t.Colours.Ring)
	}

	ring := t.Colours.Ring
	if p.Invalid {
		ring = t.Colours.Destructive
	}
	dims := Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		children := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return first(c.With(gtx)) })}
		for _, widget := range rest {
			widget := widget
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// A rule between two controls is as tall as a control and not
					// as tall as the room: a vertical separator fills the axis it
					// runs along, and in a column with no height that is the whole
					// window.
					inner := c.With(gtx)
					inner.Constraints.Max.Y = c.Dp(unit.Dp(24))
					return SeparatorProps{Vertical: true}.Layout(inner)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = layout.Constraints{}.Min
					return widget(inner)
				}),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
	})
	if p.Focused {
		// The ring of a group that holds the keyboard, in the destructive colour
		// when it was rejected: Kyse draws it around the group, from has-[:focus].
		focusRing(c, dims.Size, c.Dp(unit.Dp(root.Radius.Points())), ring)
	}
	return dims
}

// ThemeToggleProps is the button that switches between the light and the dark
// palette -- the counterpart of Kyse's ThemeToggle. It asks the window to draw in
// the other scheme, and the whole application follows from the next frame; it
// remembers nothing, because which scheme the application opens in is the
// application's to keep.
type ThemeToggleProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes the part of a [ButtonProps].
	ComponentProps
	// Label names the button for a screen reader. It is the icon's label.
	Label string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ThemeToggleProps) PartNames() []string { return []string{"root"} }

// Layout draws the button, a sun in the dark scheme and a moon in the light one,
// and switches the scheme when it is pressed.
func (p ThemeToggleProps) Layout(c ayra.Context, state *Button) ayra.Dimensions {
	next, icon := theme.Dark, innerIcon("theme-toggle", "Moon")
	if c.Theme.Scheme == theme.Dark {
		next, icon = theme.Light, innerIcon("theme-toggle", "Sun")
	}
	if state.Clicked(c) {
		c.ChangeScheme(next)
	}
	root := c.Theme.Resolve("themetoggle", "root", p.VariantName(c.Theme.Scheme.String()), theme.Style{}, p.ComponentProps)
	return ButtonProps{ComponentProps: ComponentProps{Style: root}, Icon: icon, Size: Icon, Variant: Ghost}.Layout(c, state)
}
