package widget

import (
	"image"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
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
	at      int
	chosen  int
	// Reported is the choice waiting to be read, held for the caller.
	reported bool
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
}

// Layout draws the field and, while there are suggestions, the list under it.
func (p AutocompleteProps) Layout(c ayra.Context, state *Autocomplete) ayra.Dimensions {
	for len(state.options) < len(p.Entries) {
		state.options = append(state.options, Button{})
	}

	typed := state.input.Text()
	found := matches(p.Entries, typed)

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

	if p.Disabled {
		state.open.Close()
	} else {
		p.read(c, state, len(found))
	}

	dims := InputProps{
		Placeholder: p.Placeholder,
		Disabled:    p.Disabled,
	}.Layout(c, &state.input)

	if !state.open.Showing() {
		return dims
	}

	shown := found
	if limit := p.limit(); len(shown) > limit {
		shown = shown[:limit]
	}
	if len(shown) == 0 && p.Empty == "" {
		return dims
	}

	p.list(c, state, shown, dims)
	return dims
}

// read takes the keys the list answers to, and opens it when there is
// something to show.
func (p AutocompleteProps) read(c ayra.Context, state *Autocomplete, found int) {
	if found > 0 {
		state.open.Open()
	} else if p.Empty == "" {
		state.open.Close()
	}

	event.Op(c.Ops, state)
	for {
		e, ok := c.Event(
			key.Filter{Focus: state, Name: key.NameDownArrow},
			key.Filter{Focus: state, Name: key.NameUpArrow},
			key.Filter{Focus: state, Name: key.NameEscape},
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
			state.at = step(state.at, 1, found)
		case key.NameUpArrow:
			state.at = step(state.at, -1, found)
		case key.NameEscape:
			state.open.Close()
			state.at = -1
		}
	}
}

// limit is how many suggestions are drawn.
func (p AutocompleteProps) limit() int {
	if p.Limit > 0 {
		return p.Limit
	}
	return 6
}

// list draws the suggestions under the field.
func (p AutocompleteProps) list(c ayra.Context, state *Autocomplete, shown []int, field ayra.Dimensions) {
	inner := c
	inner.Constraints.Min.X = field.Size.X
	inner.Constraints.Max.X = field.Size.X

	panel := op.Record(c.Ops)
	size := surface(inner, inner.Theme.Colours.Popover, inner.Theme.Colours.Border, controlRadius(inner), func(c ayra.Context) ayra.Dimensions {
		if len(shown) == 0 {
			return layout.UniformInset(10).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), p.Empty, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
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
				return suggestion(row, button, p.Entries[index], position == state.at)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
	}).Size
	drawn := panel.Stop()

	offset := op.Offset(image.Pt(0, field.Size.Y+c.Dp(unit.Dp(4)))).Push(c.Ops)
	clipped := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	drawn.Add(c.Ops)
	clipped.Pop()
	offset.Pop()
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
	a.at = -1
	a.settled, a.filled = text, true
}

// suggestion draws one row of a list of suggestions.
func suggestion(c ayra.Context, state *Button, entry string, under bool) ayra.Dimensions {
	variant := Ghost
	if under {
		// The row the keyboard is on is drawn the way a pressed one is, because
		// there is no pointer over it to say so. Without this, the arrow keys
		// move something invisible and return picks an entry nobody saw chosen.
		variant = Secondary
	}
	return ButtonProps{Label: entry, Variant: variant, Size: Small}.Layout(c, state)
}

// Palette is the state of a list of commands opened over the whole screen.
type Palette struct {
	dialog   Dialog
	input    Input
	options  []Button
	at       int
	chosen   int
	reported bool
}

// Open puts the palette up, with the query it had last time cleared.
func (p *Palette) Open() {
	p.dialog.Open()
	p.input.SetText("")
	p.at = 0
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
	// Commands are what can be run, in the order they are offered.
	Commands []string
	// Placeholder is shown in the search field while it is empty.
	Placeholder string
	// Empty is what stands in when nothing matches.
	Empty string
	// Width is how wide the panel is. Zero takes one a command name fits in.
	Width unit.Dp
}

// Layout draws the palette while it is open, and nothing while it is not.
func (p PaletteProps) Layout(c ayra.Context, state *Palette) ayra.Dimensions {
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
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return InputProps{Placeholder: p.placeholder()}.Layout(c.With(gtx), &state.input)
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

	event.Op(c.Ops, state)
	for {
		e, ok := c.Event(
			key.Filter{Focus: state, Name: key.NameDownArrow},
			key.Filter{Focus: state, Name: key.NameUpArrow},
			key.Filter{Focus: state, Name: key.NameReturn},
			key.Filter{Focus: state, Name: key.NameEnter},
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
