package widget

import (
	"image"
	"io"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// reader wraps a string as the stream the clipboard takes.
//
// The platform reads the value rather than being handed it, because a paste on
// some systems happens long after the copy -- so what is handed over has to
// still be readable then.
func reader(value string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(value))
}

// Copier is the state half of a control that puts something on the clipboard.
type Copier struct {
	button Button
	copied time.Time
}

// Copied reports whether the confirmation is still showing.
func (c *Copier) Copied(now time.Time) bool {
	return !c.copied.IsZero() && now.Sub(c.copied) < copiedFor
}

// copiedFor is how long the control says it worked.
//
// Long enough to be read and short enough that somebody who pressed it twice
// sees the second press take. A confirmation that never cleared would be a
// control whose label is wrong from then on.
const copiedFor = 2 * time.Second

// CopyProps is a control that copies a value.
//
// The confirmation is the whole point. A copy that gives no sign is a copy
// somebody does twice, then pastes to find out whether it worked.
type CopyProps struct {
	// Value is what is copied.
	Value string
	// Label is the control's own text. Empty says "Copy".
	Label string
	// Done is what it says afterwards. Empty says "Copied".
	Done string
	// Size is how much room it takes.
	Size Size
}

// Layout draws the control and copies when it is pressed.
func (p CopyProps) Layout(c ayra.Context, state *Copier) ayra.Dimensions {
	if state.button.Clicked(c) {
		c.Execute(clipboard.WriteCmd{Type: "application/text", Data: reader(p.Value)})
		state.copied = c.Now
	}

	label := p.Label
	if label == "" {
		label = "Copy"
	}
	if state.Copied(c.Now) {
		label = p.Done
		if label == "" {
			label = "Copied"
		}
	}

	c.Constraints.Min = image.Point{}
	return ButtonProps{Label: label, Variant: Outline, Size: p.Size}.Layout(c, &state.button)
}

// Segments is the state half of a segmented control: which one, and the
// presses.
type Segments struct {
	group    Group
	selected int
}

// Selected is the index chosen.
func (s *Segments) Selected() int { return s.selected }

// Choose picks one, for the state a screen arrives with.
func (s *Segments) Choose(index int) { s.selected = index }

// SegmentedProps is a set of exclusive choices drawn as one object.
//
// It is a radio group for a small, fixed set where every option fits on one
// row: a filter, a unit, a view. Past four or five it becomes a [SelectProps],
// because a row of segments that wraps is a row nobody can read as a set.
type SegmentedProps struct {
	// Options are the choices, in order.
	Options []string
	// Size is how much room each takes.
	Size Size
}

// Layout draws the control and returns the room it took.
func (p SegmentedProps) Layout(c ayra.Context, state *Segments) ayra.Dimensions {
	if chosen := state.group.Clicked(c); chosen >= 0 {
		state.selected = chosen
	}
	return ButtonGroupProps{Labels: p.Options, Selected: state.selected, Size: p.Size}.Layout(c, &state.group)
}

// SidebarProps is the column of places a screen can go.
type SidebarProps struct {
	// Entries are the destinations, in order.
	Entries []string
	// Title is the heading above them. Empty draws none.
	Title string
	// Width is how wide the column is. Zero takes one that fits a name.
	Width unit.Dp
}

// Sidebar is the state half: which entry is current, and the press on each.
type Sidebar struct {
	current int
	entries []Button
}

// Current is the entry showing.
func (s *Sidebar) Current() int { return s.current }

// Show marks one as current, for the state a screen arrives with.
func (s *Sidebar) Show(index int) { s.current = index }

// Chosen reports which entry was pressed, and -1 when none was.
//
// It does not move the current one itself: which entry is showing follows from
// what the screen navigated to, and a sidebar that marked itself would show the
// new place before the screen had it -- then stay wrong if the navigation
// failed.
func (s *Sidebar) Chosen(c ayra.Context) int {
	for index := range s.entries {
		if s.entries[index].Clicked(c) {
			return index
		}
	}
	return -1
}

// Layout draws the column and returns the room it took.
func (p SidebarProps) Layout(c ayra.Context, state *Sidebar) ayra.Dimensions {
	for len(state.entries) < len(p.Entries) {
		state.entries = append(state.entries, Button{})
	}

	width := p.Width
	if width == 0 {
		width = 220
	}
	if maximum := c.Dp(width); c.Constraints.Max.X > maximum {
		c.Constraints.Max.X = maximum
	}
	c.Constraints.Min.X = c.Constraints.Max.X

	children := make([]layout.FlexChild, 0, len(p.Entries)+2)
	if p.Title != "" {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 12, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), p.Title, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, semibold())
				})
			}),
		)
	}

	for index, entry := range p.Entries {
		index, entry := index, entry
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ItemProps{Title: entry, Pressable: true, Selected: index == state.current}.
				Layout(c.With(gtx), &state.entries[index], nil)
		}))
	}

	measure := op.Record(c.Ops)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
	drawn := measure.Stop()
	drawn.Add(c.Ops)

	return dims
}
