package widget

import (
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/io/clipboard"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Value is what is copied.
	Value string
	// Label is the control's own text. Empty says "Copy".
	Label string
	// Done is what it says afterwards. Empty says "Copied".
	Done string
	// Size is how much room it takes.
	Size Size
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p CopyProps) PartNames() []string { return []string{"root"} }

// Layout draws the control and copies when it is pressed.
func (p CopyProps) Layout(c ayra.Context, state *Copier) ayra.Dimensions {
	return withRoot(c, "copy", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p CopyProps) layout(c ayra.Context, state *Copier) ayra.Dimensions {
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
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Options are the choices, in order.
	Options []string
	// Size is how much room each takes.
	Size Size
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SegmentedProps) PartNames() []string { return []string{"root"} }

// Layout draws the control and returns the room it took.
func (p SegmentedProps) Layout(c ayra.Context, state *Segments) ayra.Dimensions {
	return withRoot(c, "segmented", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p SegmentedProps) layout(c ayra.Context, state *Segments) ayra.Dimensions {
	if chosen := state.group.Clicked(c); chosen >= 0 {
		state.selected = chosen
	}
	for len(state.group.buttons) < len(p.Options) {
		state.group.buttons = append(state.group.buttons, Button{})
	}

	t := c.Theme
	size := p.Size
	if size == Medium || size == Icon {
		// A square segment is a button that holds one mark; a bar of words has no
		// use for it, and its width is the widest word's.
		size = Small
	}

	// Kyse's bar: a muted track with the segments in it, every one as wide as
	// the widest, and the chosen one raised on the page's own colour. Joined
	// buttons of the widths of their words, with the chosen one a grey a shade
	// from the track, were a bar in which the choice could not be told.
	pad, gap := c.Dp(unit.Dp(3)), c.Dp(unit.Dp(2))
	inner := c
	inner.Constraints.Min = image.Point{}

	widest := 0
	for _, label := range p.Options {
		props := ButtonProps{Label: label, Variant: Ghost, Size: size}
		style := props.style(t)
		// Measured as the chosen one is drawn, in the heavier weight: the widest
		// word is wider in it, and a chosen segment sized for the lighter one ran
		// out of its slot and past the end of the track.
		style.Font.Weight = theme.SemiBold
		measured := op.Record(inner.Ops)
		text := buttonLabel(inner, label, style)
		measured.Stop()
		widest = max(widest, text.Size.X+c.Dp(unit.Dp(style.Padding.Left+style.Padding.Right)))
	}
	if n := len(p.Options); n > 0 && c.Constraints.Max.X < 1<<20 {
		widest = min(widest, (c.Constraints.Max.X-2*pad-(n-1)*gap)/n)
	}

	segments := make([]op.CallOp, len(p.Options))
	height := 0
	for index, label := range p.Options {
		style := theme.Style{Foreground: theme.Colour(t.Colours.MutedForeground)}
		if index == state.selected {
			edge := t.Colours.Foreground
			edge.A = 64
			style = theme.Style{
				Background: theme.Colour(t.Colours.Background),
				Foreground: theme.Colour(t.Colours.Foreground),
				Border:     theme.Colour(edge),
				Radius:     theme.Pt(max(t.Metrics.Radius-1, 0)),
				// The size is the button's own: a Font with only a weight in it is a
				// font with no size, and the chosen label was drawn a size larger.
				Font: theme.Font{Size: theme.Pt(ButtonProps{Size: size}.textSize(t)), Weight: theme.SemiBold},
			}
		} else {
			style.Radius = theme.Pt(max(t.Metrics.Radius-1, 0))
		}
		// The least width is what the content is asked for, and the button's own
		// padding is added to it: asking for the whole slot made a segment as wide
		// as the slot and the padding over.
		padding := ButtonProps{Size: size}.style(t).Padding
		cell := inner
		cell.Constraints.Min.X = max(widest-c.Dp(unit.Dp(padding.Left+padding.Right)), 0)
		record := op.Record(inner.Ops)
		dims := ButtonProps{ComponentProps: ComponentProps{Style: style}, Label: label, Variant: Ghost, Size: size}.Layout(cell, &state.group.buttons[index])
		segments[index] = record.Stop()
		height = max(height, dims.Size.Y)
	}

	width := 2*pad + max(len(p.Options)-1, 0)*gap + widest*len(p.Options)
	total := image.Pt(width, height+2*pad)
	paintShape(c, total, t.Colours.Muted, color.NRGBA{}, controlRadius(c), 0)
	for index := range segments {
		at := op.Offset(image.Pt(pad+index*(widest+gap), pad)).Push(c.Ops)
		segments[index].Add(c.Ops)
		at.Pop()
	}
	return ayra.Dimensions{Size: total}
}

// SidebarProps is the column of places a screen can go.
type SidebarProps struct {
	// ComponentProps is what the caller changes on this one instance: a variant
	// of the product's own, and the style of each named part. The widget
	// publishes "root", the outermost shape it draws, and the parts its
	// PartNames lists.
	ComponentProps
	// Entries are the destinations, in order.
	Entries []string
	// Title is the heading above them. Empty draws none.
	Title string
	// Width is how wide the column is. Zero takes one that fits a name.
	Width unit.Dp
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p SidebarProps) PartNames() []string { return []string{"root"} }

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
	return withRoot(c, "sidebar", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return p.layout(c, state)
	})
}

// layout is Layout without the root part around it.
func (p SidebarProps) layout(c ayra.Context, state *Sidebar) ayra.Dimensions {
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
