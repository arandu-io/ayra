package widget

import (
	"image"
	"image/color"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/icons"
)

// matches answers the entries a query matches, by index, in the order they
// were given.
//
// Case-insensitive substring, and nothing cleverer. A fuzzy match orders
// results by a score nobody can see, so the same three keystrokes put a
// different entry first depending on what else is in the list -- and the entry
// under the cursor when return is pressed is then not the one that was under it
// when the key went down.
//
// An empty query matches everything, which is what makes a list of commands a
// list of commands before anything is typed.
func matches(entries []string, query string) []int {
	query = strings.ToLower(strings.TrimSpace(query))

	found := make([]int, 0, len(entries))
	for index, entry := range entries {
		if query == "" || strings.Contains(strings.ToLower(entry), query) {
			found = append(found, index)
		}
	}
	return found
}

// step moves a cursor through a list of a given length, wrapping at both ends.
//
// Wrapping rather than stopping, because the list is short and reachable from
// either end: pressing up on the first entry to reach the last is how somebody
// gets to "quit" without holding a key down.
func step(at, by, length int) int {
	if length == 0 {
		return -1
	}
	if at < 0 {
		if by < 0 {
			return length - 1
		}
		return 0
	}
	return ((at+by)%length + length) % length
}

// Autocomplete is the state of a field that suggests entries as it is typed in.
type Autocomplete struct {
	input   Input
	open    Disclosure
	options []Button
	// at is the row the keyboard is on, and active says there is one: the zero
	// value of the state is a list with no row under the cursor, so a list that
	// has only just opened does not answer return with an entry nobody chose.
	at     int
	active bool
	chosen int
	// Reported is the choice waiting to be read, held for the caller.
	reported bool
	// seen is the text the cursor was last placed for. Typing moves the cursor
	// off whatever row it was on, because the rows are not the same rows.
	seen string
	// dismissed is the text escape closed the list for. The list stays closed
	// until the text is something else, because a list that escape closed and
	// the next frame opened again is a list that cannot be dismissed.
	dismissed     bool
	dismissedText string
	// fit is how many rows the room under the field took in the last frame, and
	// zero before there was one. The keyboard moves among the rows that are
	// drawn and no others.
	fit int
	// Settled is the text the field was last filled with by something other
	// than typing -- a choice from the list, or the screen filling the field
	// in. While the field still holds exactly that, nothing is suggested.
	//
	// Without it, choosing a suggestion reopens the list on the next frame:
	// the text now matches the entry it came from, so the query finds it and
	// the panel is drawn again over the entry somebody just picked. The panel
	// is closed and immediately reopened, and the only way out is to type.
	settled string
	// Filled says whether settled holds anything, because the empty string is
	// a text a field legitimately holds and cannot stand for "nothing".
	filled bool
}

// Text is what has been typed.
func (a *Autocomplete) Text() string { return a.input.Text() }

// SetText replaces what has been typed, and closes the suggestions.
//
// Closed, because the field is being filled in by the screen and not by a
// person: a list of suggestions for text nobody typed is a panel that opens by
// itself over whatever is under it.
func (a *Autocomplete) SetText(s string) {
	a.input.SetText(s)
	a.settle(s)
}

// Showing reports whether the suggestions are on screen.
func (a *Autocomplete) Showing() bool { return a.open.Showing() }

// Chosen answers the index of the entry that was picked, once, and consumes it.
// It answers -1 when none was.
func (a *Autocomplete) Chosen() int {
	if !a.reported {
		return -1
	}
	a.reported = false
	return a.chosen
}

// AutocompleteProps is a text field with a list of entries under it.
type AutocompleteProps struct {
	// ComponentProps is what the caller changes on this one instance. The field
	// publishes the parts of an [InputProps], restyled under the name
	// "autocomplete".
	ComponentProps
	// Entries are what can be suggested, in the order they are offered.
	Entries []string
	// Placeholder is shown while the field is empty.
	Placeholder string
	// Limit is how many suggestions are drawn at once. Zero draws six.
	Limit int
	// Empty is what stands in when nothing matches. An empty string draws
	// nothing at all, which closes the list.
	Empty string
	// Disabled draws it as unavailable and refuses to open.
	Disabled bool
	// Unfiltered says the entries were already chosen for what is typed, by
	// something that knows more than a substring does -- a server that matched
	// on a code, a name and a neighbourhood. They are offered as they are, in
	// the order given, instead of being filtered again by the text: a second
	// filter would drop the very entries the first one found.
	Unfiltered bool
	// Invalid draws the field as rejected.
	Invalid bool

	// problem says Empty is a failure and not a plain "nothing": it is drawn in
	// the destructive ink.
	problem bool
	// widget is the name the field is restyled under when another widget draws
	// it: the combobox, which is this with a server behind it.
	widget string
	// tail is an icon drawn at the end of the box: the combobox's caret.
	tail icons.Icon
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p AutocompleteProps) PartNames() []string {
	return []string{"root", "focus", "invalid", "placeholder"}
}

// Layout draws the field and, while there are suggestions, the list under it.
func (p AutocompleteProps) Layout(c ayra.Context, state *Autocomplete) ayra.Dimensions {
	for len(state.options) < len(p.Entries) {
		state.options = append(state.options, Button{})
	}

	typed := state.input.Text()
	found := matches(p.Entries, typed)
	if p.Unfiltered {
		found = found[:0]
		for index := range p.Entries {
			found = append(found, index)
		}
	}

	switch {
	case typed == "":
		// A field nobody has typed in yet suggests nothing. Every entry
		// matching an empty query is what a command palette wants and what a
		// text field does not: the list would be open from the moment the
		// screen was drawn.
		found = nil
	case state.filled && typed == state.settled:
		// The field holds exactly what was put in it, so there is nothing to
		// suggest yet.
		found = nil
	default:
		// Anything else is somebody typing, and typing is what reopens the
		// list.
		state.filled = false
	}

	shown := found
	if limit := p.limit(); len(shown) > limit {
		shown = shown[:limit]
	}
	if state.fit > 0 && len(shown) > state.fit {
		shown = shown[:state.fit]
	}

	// Typing is a text nobody filled in: what a list with nothing to offer may
	// answer, and what reopens one escape closed.
	typing := typed != "" && !(state.filled && typed == state.settled)
	if p.Disabled {
		state.open.Close()
	} else {
		p.read(c, state, typed, typing, shown)
	}

	dims := InputProps{
		ComponentProps: p.ComponentProps,
		widget:         orDefault(p.widget, "autocomplete"),
		Placeholder:    p.Placeholder,
		Disabled:       p.Disabled,
		Invalid:        p.Invalid,
		tail:           p.tail,
	}.Layout(c, &state.input)

	if !state.open.Showing() {
		state.fit = 0
		return dims
	}

	if len(found) == 0 && p.Empty == "" {
		return dims
	}

	p.list(c, state, found, dims)
	return dims
}

// read takes the keys the list answers to, and opens it when there is
// something to show.
func (p AutocompleteProps) read(c ayra.Context, state *Autocomplete, typed string, typing bool, shown []int) {
	if typed != state.seen {
		state.seen, state.active = typed, false
		state.dismissed = state.dismissed && typed == state.dismissedText
	}

	switch {
	case state.dismissed:
		state.open.Close()
	case len(shown) > 0 || (typing && p.Empty != ""):
		state.open.Open()
	default:
		state.open.Close()
	}

	// The keys are asked for on the field's own editor, which is what holds the
	// keyboard while somebody types. Asked for on the state they would never
	// arrive: the state is not a control and is never focused.
	focus := &state.input.editor
	filters := []event.Filter{
		key.Filter{Focus: focus, Name: key.NameDownArrow},
		key.Filter{Focus: focus, Name: key.NameUpArrow},
		key.Filter{Focus: focus, Name: key.NameEscape},
	}
	if state.open.Showing() && state.active {
		// Return is claimed only while a row is under the cursor. Otherwise it is
		// the field's own, and submits.
		filters = append(filters,
			key.Filter{Focus: focus, Name: key.NameReturn},
			key.Filter{Focus: focus, Name: key.NameEnter},
		)
	}
	for {
		e, ok := c.Event(filters...)
		if !ok {
			break
		}
		press, isKey := e.(key.Event)
		if !isKey || press.State != key.Press {
			continue
		}
		switch press.Name {
		case key.NameDownArrow:
			if !state.open.Showing() || len(shown) == 0 {
				continue
			}
			if !state.active {
				state.active, state.at = true, 0
			} else {
				state.at = step(state.at, 1, len(shown))
			}
		case key.NameUpArrow:
			if !state.open.Showing() || len(shown) == 0 {
				continue
			}
			if !state.active {
				state.active, state.at = true, len(shown)-1
			} else {
				state.at = step(state.at, -1, len(shown))
			}
		case key.NameEscape:
			if state.open.Showing() {
				state.dismissed, state.dismissedText = true, typed
			}
			state.open.Close()
			state.active = false
		case key.NameReturn, key.NameEnter:
			if state.active && state.at >= 0 && state.at < len(shown) {
				index := shown[state.at]
				state.choose(index, p.Entries[index])
			}
		}
	}
	if state.active && state.at >= len(shown) {
		state.at = max(len(shown)-1, 0)
		state.active = len(shown) > 0
	}
}

// limit is how many suggestions are drawn.
func (p AutocompleteProps) limit() int {
	if p.limitSet() {
		return p.Limit
	}
	return 6
}

func (p AutocompleteProps) limitSet() bool { return p.Limit > 0 }

// list draws the suggestions under the field, or above it when there is no
// room under it.
//
// The room under the field is what the layout above it left: a screen is laid
// out in the window, and the window is what the keyboard has not covered. A
// field low in it is a field whose list would run off the bottom, so the list
// keeps to the rows that fit; with fewer than a few fitting it goes above the
// field instead. It is drawn last, over whatever the screen put below the field
// -- the help line, the next field -- because a list that those drew over is a
// list nobody can read.
func (p AutocompleteProps) list(c ayra.Context, state *Autocomplete, found []int, field ayra.Dimensions) {
	inner := c
	inner.Constraints.Min.X = field.Size.X
	inner.Constraints.Max.X = field.Size.X
	inner.Constraints.Min.Y = 0
	// The panel is as tall as its rows. The room the field was given is what
	// decides how many rows there are, below; it is not a ceiling on the panel,
	// which a flip above the field would otherwise be cut by.
	inner.Constraints.Max.Y = 1 << 20

	gap := c.Dp(unit.Dp(4))
	pad := c.Dp(unit.Dp(4))

	shown := found
	if limit := p.limit(); len(shown) > limit {
		shown = shown[:limit]
	}

	above := false
	state.fit = 0
	if c.Constraints.Max.Y < boundedHeight {
		// What a row takes is measured on a press state of its own, which is
		// thrown away: the real ones must not have a press read by a drawing that
		// is not shown.
		probe := op.Record(c.Ops)
		rowHeight := suggestion(inner, &Button{}, "Ag", false).Size.Y
		probe.Stop()
		if rowHeight > 0 {
			room := c.Constraints.Max.Y - field.Size.Y - gap - 2*pad - 2*hairline(c)
			fits := max(room/rowHeight, 0)
			want := max(len(shown), 1)
			if fits < min(want, 3) {
				above = true
			} else if fits < len(shown) {
				shown = shown[:fits]
				state.fit = fits
			}
		}
	}

	panel := op.Record(c.Ops)
	size := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
		return layout.Inset{Top: 4, Bottom: 4}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			c := c.With(gtx)
			if len(shown) == 0 {
				return layout.UniformInset(10).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
					ink := c.Theme.Colours.MutedForeground
					if p.problem {
						ink = c.Theme.Colours.Destructive
					}
					return drawText(c.With(gtx), p.Empty, unit.Sp(c.Theme.Type.Small), ink, 0, text.Start, plain())
				})
			}

			rows := make([]layout.FlexChild, 0, len(shown))
			for position, index := range shown {
				position, index := position, index
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					row := c.With(gtx)
					button := &state.options[index]
					if button.Clicked(row) {
						state.choose(index, p.Entries[index])
					}
					return suggestion(row, button, p.Entries[index], state.active && position == state.at)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
		})
	}).Size
	drawn := panel.Stop()

	top := field.Size.Y + gap
	if above {
		top = -size.Y - gap
	}
	placed := op.Record(c.Ops)
	offset := op.Offset(image.Pt(0, top)).Push(c.Ops)
	clipped := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	drawn.Add(c.Ops)
	clipped.Pop()
	offset.Pop()
	op.Defer(c.Ops, placed.Stop())
}

// choose fills the field in with an entry and closes the list.
func (a *Autocomplete) choose(index int, entry string) {
	a.input.SetText(entry)
	a.settle(entry)
	a.chosen, a.reported = index, true
}

// settle closes the list and records the text it must stay closed for.
func (a *Autocomplete) settle(text string) {
	a.open.Close()
	a.at, a.active = 0, false
	a.settled, a.filled = text, true
}

// suggestion draws one row of a list of suggestions: the text at the left, as
// Kyse's options are, in a row as tall as a finger.
//
// The row the keyboard is on is drawn the way a chosen one is, because there is
// no pointer over it to say so. Without this, the arrow keys move something
// invisible and return picks an entry nobody saw chosen.
func suggestion(c ayra.Context, state *Button, entry string, under bool) ayra.Dimensions {
	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		c := c.With(gtx)
		c.Constraints.Min.X = c.Constraints.Max.X
		c.Constraints.Min.Y = c.Dp(unit.Dp(44))

		var fill color.NRGBA
		if under {
			fill = c.Theme.Colours.Accent
		}
		return surfaceWidth(c, fill, fill, controlRadius(c), 0, func(c ayra.Context) ayra.Dimensions {
			// West sits the text at the left and in the middle of the row's
			// height, which Min alone does not do: the room is the row's, and
			// the text is laid out in its own.
			return layout.W.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), entry, unit.Sp(c.Theme.Type.Body), c.Theme.Colours.Foreground, 0, text.Start, plain())
				})
			})
		})
	})
}

// Palette is the state of a list of commands opened over the whole screen.
type Palette struct {
	dialog   Dialog
	input    Input
	options  []Button
	at       int
	chosen   int
	reported bool
	// focus is whether the field still has to be given the keyboard: the dialog
	// takes it when it opens, and the field takes it from the dialog.
	focus bool
}

// Open puts the palette up, with the query it had last time cleared.
func (p *Palette) Open() {
	p.dialog.Open()
	p.input.SetText("")
	p.at = 0
	p.focus = true
}

// Close takes it down.
func (p *Palette) Close() { p.dialog.Close() }

// Showing reports whether it is up.
func (p *Palette) Showing() bool { return p.dialog.Showing() }

// Query is what has been typed into it.
func (p *Palette) Query() string { return p.input.Text() }

// Chosen answers the index of the command that was run, once, and consumes it.
// It answers -1 when none was.
func (p *Palette) Chosen() int {
	if !p.reported {
		return -1
	}
	p.reported = false
	return p.chosen
}

// PaletteProps is a searchable list of what an application can do.
type PaletteProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Commands are what can be run, in the order they are offered.
	Commands []string
	// Placeholder is shown in the search field while it is empty.
	Placeholder string
	// Empty is what stands in when nothing matches.
	Empty string
	// Width is how wide the panel is. Zero takes one a command name fits in.
	Width unit.Dp
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p PaletteProps) PartNames() []string { return []string{"root"} }

// Layout draws the palette while it is open, and nothing while it is not.
func (p PaletteProps) Layout(c ayra.Context, state *Palette) ayra.Dimensions {
	return withRoot(c, "palette", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p PaletteProps) layout(c ayra.Context, state *Palette) ayra.Dimensions {
	if !state.dialog.Showing() {
		return ayra.Dimensions{}
	}

	for len(state.options) < len(p.Commands) {
		state.options = append(state.options, Button{})
	}

	found := matches(p.Commands, state.input.Text())
	p.read(c, state, found)

	width := p.Width
	if width == 0 {
		width = 480
	}

	return DialogProps{Width: width, Modal: true, Dismissible: true}.Layout(c, &state.dialog, func(c ayra.Context) ayra.Dimensions {
		if state.focus {
			state.focus = false
			c.Execute(key.FocusCmd{Tag: &state.input.editor})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return InputProps{Placeholder: p.placeholder(), lead: innerIcon("command", "MagnifyingGlass")}.Layout(c.With(gtx), &state.input)
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.list(c.With(gtx), state, found)
			}),
		)
	})
}

// placeholder is what the search field says before anything is typed.
func (p PaletteProps) placeholder() string {
	if p.Placeholder != "" {
		return p.Placeholder
	}
	return "Type a command"
}

// read takes the keys that move the cursor and run a command.
//
// Return is read here rather than from the field, because what it means is not
// "this text is finished" but "run the one under the cursor" -- and the cursor
// belongs to the list.
func (p PaletteProps) read(c ayra.Context, state *Palette, found []int) {
	if state.at >= len(found) {
		// The list shortened under the cursor, which happens on every
		// keystroke that narrows it. Left alone, return runs whatever ends up
		// at that position next.
		state.at = 0
	}

	// The keys are the field's: it is what holds the keyboard while a command is
	// typed, and a filter on the palette's own state, which is never focused,
	// was answered by nothing.
	focus := &state.input.editor
	for {
		e, ok := c.Event(
			key.Filter{Focus: focus, Name: key.NameDownArrow},
			key.Filter{Focus: focus, Name: key.NameUpArrow},
			key.Filter{Focus: focus, Name: key.NameReturn},
			key.Filter{Focus: focus, Name: key.NameEnter},
		)
		if !ok {
			break
		}
		press, isKey := e.(key.Event)
		if !isKey || press.State != key.Press {
			continue
		}
		switch press.Name {
		case key.NameDownArrow:
			state.at = step(state.at, 1, len(found))
		case key.NameUpArrow:
			state.at = step(state.at, -1, len(found))
		case key.NameReturn, key.NameEnter:
			if state.at >= 0 && state.at < len(found) {
				state.run(found[state.at])
			}
		}
	}
}

// list draws the commands that match.
func (p PaletteProps) list(c ayra.Context, state *Palette, found []int) ayra.Dimensions {
	if len(found) == 0 {
		empty := p.Empty
		if empty == "" {
			empty = "Nothing matches"
		}
		return layout.UniformInset(10).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), empty, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
		})
	}

	rows := make([]layout.FlexChild, 0, len(found))
	for position, index := range found {
		position, index := position, index
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := c.With(gtx)
			button := &state.options[index]
			if button.Clicked(row) {
				state.run(index)
			}
			return suggestion(row, button, p.Commands[index], position == state.at)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
}

// run reports a command and takes the palette down.
func (p *Palette) run(index int) {
	p.chosen, p.reported = index, true
	p.dialog.Close()
}
