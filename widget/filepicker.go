package widget

import (
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// FileEntry is one line of a listing: a name, whether it leads somewhere, and
// how big it is.
//
// It is data the caller hands over, and the control never goes looking for it.
// Nothing here opens a directory, and that is the whole design of this control:
// one that read the disk itself could not be drawn in a test, because a test
// has no directory to point it at; could not be driven by a listing that
// arrived over the wire, because the names would have to exist locally first;
// and would do the reading inside the frame, where a volume that has gone to
// sleep stops the application for as long as it takes to wake.
type FileEntry struct {
	// Name is the entry's own name, without anything above it. A listing is of
	// one directory, so the names in it are already distinct, and the picker
	// uses the name as the entry's identity for that reason.
	Name string
	// Dir marks an entry that leads somewhere rather than one that can be
	// chosen.
	Dir bool
	// Size is the entry in bytes. Negative says the size is not known, which
	// draws nothing -- "0 B" is a claim about an empty file, and a listing that
	// made it about every file it had not measured would be wrong on most rows.
	Size int64
}

// FilePicker is the state half: where in the tree the person is, what they
// filtered by, what they chose, and the press on every row.
//
// The caller holds it, and holds one per picker. What it deliberately does not
// hold is the listing: that arrives in the props each frame, from whoever can
// read it.
type FilePicker struct {
	crumbs Crumbs
	filter Input

	// rows is the press state of each drawn row, and listed is the name each
	// one belongs to. The two are kept the same length, which is what makes a
	// press attributable to a file rather than to a position.
	rows   []Button
	listed []string

	selected string

	opened    string
	descended bool

	chosen string
	picked bool

	depth   int
	climbed bool
}

// Selected is the entry currently marked, and empty when none is.
//
// It survives the frame it was chosen on, so the row keeps its mark while the
// caller decides what to do with it, and it is given up when the listing being
// drawn stops containing that name -- a mark belongs to a row, and one kept
// after the row has gone is a selection nothing on the screen shows.
func (f *FilePicker) Selected() string { return f.selected }

// Choose marks one by name, for the state a screen arrives with.
func (f *FilePicker) Choose(name string) { f.selected = name }

// Filter is what the listing is being narrowed by.
func (f *FilePicker) Filter() string { return f.filter.Text() }

// SetFilter writes it, for a screen that arrives with one.
func (f *FilePicker) SetFilter(value string) { f.filter.SetText(value) }

// Opened reports a directory somebody asked to go into, once, and consumes it.
//
// The caller answers it by supplying that directory's listing on the next
// frame. It is reported once because the answer is work -- a read, and possibly
// a request -- and a control that reported the same press every frame would
// have it done sixty times a second for as long as the row stayed under the
// pointer.
//
// What comes back is the entry's own name and not a path. Joining it to the
// segments above would mean choosing a separator, and the separator belongs to
// whatever produced the listing, which need not be the machine this is drawn
// on.
func (f *FilePicker) Opened() (string, bool) {
	name, asked := f.opened, f.descended
	f.opened, f.descended = "", false
	return name, asked
}

// Chosen reports a file that was picked, once, and consumes it.
//
// Consumed for the reason Opened is: what a caller does with a chosen file is
// close the picker, read the file or submit a form, and none of those is
// something to do again on the next frame because the answer has not been
// collected yet.
func (f *FilePicker) Chosen() (string, bool) {
	name, picked := f.chosen, f.picked
	f.chosen, f.picked = "", false
	return name, picked
}

// Up reports that somebody asked to go back up the tree, and to which depth,
// once, and consumes it.
//
// The depth counts segments below the top: nought is the top itself, one is the
// first segment under it. A depth rather than a boolean, because the trail
// makes every level above reachable in one press and a caller told only "up"
// would have to walk there one frame at a time.
//
// This is the only way back up, and there is no separate button for the parent
// directory. One would do what pressing the second-to-last step of the trail
// already does, and two controls for one movement is two things to keep
// agreeing with each other.
func (f *FilePicker) Up() (int, bool) {
	depth, asked := f.depth, f.climbed
	f.depth, f.climbed = 0, false
	return depth, asked
}

// relist rebuilds the press state when the listing being drawn has changed.
//
// The rows are matched to the listing by name, and the comparison is by name
// rather than by length. Two directories holding the same number of entries
// would otherwise keep every press target across the move, and a press begun on
// the third row of the directory left behind would land on the third row of the
// one arrived at -- a different file, reported as chosen by somebody who never
// touched it.
//
// The slices are replaced rather than reset, because a press target is found by
// its address: reusing the array would hand the new rows the very addresses the
// old ones were pressed on, which is the defect this exists to remove.
func (f *FilePicker) relist(entries []FileEntry) {
	unchanged := len(f.listed) == len(entries)
	if unchanged {
		for index := range entries {
			if f.listed[index] != entries[index].Name {
				unchanged = false
				break
			}
		}
	}
	if unchanged {
		return
	}

	f.listed = make([]string, len(entries))
	for index := range entries {
		f.listed[index] = entries[index].Name
	}
	f.rows = make([]Button, len(entries))

	// The mark belongs to a row, so it goes when the row does. Kept, it is a
	// selection nothing on the screen shows, and a caller reading it is told a
	// file is chosen that the listing in front of the person does not contain.
	if !slices.Contains(f.listed, f.selected) {
		f.selected = ""
	}
}

// FilePickerProps is a listing somebody browses and picks a file out of.
//
// It holds no listing of its own and reads nothing: the entries, the trail and
// the name of the top all arrive each frame from whoever can produce them.
//
// There is no multiple selection. Picking several is a different control, not a
// flag on this one: a set is not chosen until somebody says it is finished, so
// it needs a confirming press this has no room for, and without one the same
// press on a row would mean "this is my answer" in one picker and "add this to
// my answer" in the next.
//
// It draws where it is put and opens nothing over the screen. A picker that
// wanted to be modal is this inside a [DialogProps], which already owns the
// scrim, the escape key and the press outside.
type FilePickerProps struct {
	// Entries are the listing of the directory currently open, in the order
	// they should be read. The order inside each kind is kept as it arrives,
	// because a caller that sorted by date already made that decision and a
	// control that sorted again would overrule it.
	Entries []FileEntry
	// Path is the segments open below the top, outermost first. It is what the
	// trail is drawn from, and the caller owns it: the picker reports where
	// somebody asked to go and never moves itself, so a move that failed leaves
	// the screen showing where it actually is.
	Path []string
	// Root is what the top is called -- "Files", a volume, a bucket. Empty
	// says "Files".
	Root string
	// Disabled draws the picker as unavailable and stops every part of it
	// answering.
	Disabled bool
}

// Layout draws the picker and returns the room it took, which is the width it
// was given.
//
// The whole width, rather than as much as the longest name needs, because a
// listing that ended where its content did would move its right edge every time
// somebody went into a directory -- and the sizes on the right would move with
// it.
func (p FilePickerProps) Layout(c ayra.Context, state *FilePicker) ayra.Dimensions {
	entries := p.visible(state.Filter())
	state.relist(entries)

	if !p.Disabled {
		p.read(c, state, entries)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return BreadcrumbProps{Steps: p.steps()}.Layout(c.With(gtx), &state.crumbs)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return InputProps{Placeholder: "Filter", Disabled: p.Disabled}.Layout(c.With(gtx), &state.filter)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.listing(c.With(gtx), state, entries)
		}),
	)
}

// read turns the presses of the last frame into the intents a caller collects.
//
// It runs before anything is drawn, so that what a press changes is on screen
// in the same frame. Read afterwards, every press costs a frame, and a frame of
// lag on a row is what reads as a listing that ignored the first click.
func (p FilePickerProps) read(c ayra.Context, state *FilePicker, entries []FileEntry) {
	// The trail reports the step that was pressed, and the step's index is the
	// depth: nought is the top.
	if depth := state.crumbs.Clicked(c); p.ascends(depth) {
		state.depth, state.climbed = depth, true
	}

	for index := range entries {
		if state.rows[index].Clicked(c) {
			state.act(entries[index])
		}
	}
}

// ascends reports whether a pressed step of the trail is one to go back to.
//
// Two steps are refused. The last is where the person already is, and reporting
// it would ask the caller to move to the directory it has not moved from --
// work, and a redraw, for a press that changed nothing. Anything past the end
// is a target the trail kept from a deeper path: the trail grows a target per
// step it has ever drawn and does not give them back when the path gets
// shorter, so one left over would name a step that is no longer on the screen.
func (p FilePickerProps) ascends(depth int) bool {
	return depth >= 0 && depth < len(p.steps())-1
}

// act records what a press on one entry means, which is two different things.
//
// A directory is a request the caller has to answer with another listing, and
// nothing is chosen by pressing it -- a directory is not a file, and a picker
// that reported one as chosen would hand back a name the caller cannot open. A
// file is the answer, and it is also what the row is marked with, so the
// listing shows what was picked while the caller decides what to do about it.
func (f *FilePicker) act(entry FileEntry) {
	if entry.Dir {
		f.opened, f.descended = entry.Name, true
		return
	}
	f.selected = entry.Name
	f.chosen, f.picked = entry.Name, true
}

// steps answers the trail, with the name of the top at its head.
func (p FilePickerProps) steps() []string {
	root := p.Root
	if root == "" {
		root = "Files"
	}
	return append([]string{root}, p.Path...)
}

// visible answers the entries that are drawn, in the order they are drawn.
//
// Directories come before files, and the two halves each keep the order they
// arrived in. Interleaved, the way down out of a directory is scattered through
// the things that are not a way down, and somebody looking for a file that is
// not in this one has to read every line to find out.
//
// The filter narrows both kinds. Sparing the directories would leave a screen
// that is supposed to have got shorter still carrying every one of them, which
// is the half of a long listing a filter is usually being used to get past.
func (p FilePickerProps) visible(filter string) []FileEntry {
	wanted := strings.ToLower(filter)

	kept := make([]FileEntry, 0, len(p.Entries))
	for _, entry := range p.Entries {
		if wanted != "" && !strings.Contains(strings.ToLower(entry.Name), wanted) {
			continue
		}
		kept = append(kept, entry)
	}

	ordered := make([]FileEntry, 0, len(kept))
	for _, entry := range kept {
		if entry.Dir {
			ordered = append(ordered, entry)
		}
	}
	for _, entry := range kept {
		if !entry.Dir {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

// listing draws the rows inside their panel, or says why there are none.
//
// The panel is held to the full width it was offered, which matters on the
// frame it has nothing to draw: a message is as wide as its own sentence, and a
// panel that shrank to it would be a bordered box that changed size every time
// somebody typed another letter into the filter.
func (p FilePickerProps) listing(c ayra.Context, state *FilePicker, entries []FileEntry) ayra.Dimensions {
	c.Constraints.Min.X = c.Constraints.Max.X

	return surface(c, c.Theme.Colours.Background, c.Theme.Colours.Border, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
		return layout.UniformInset(4).Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			if len(entries) == 0 {
				return p.nothing(inner, state)
			}

			children := make([]layout.FlexChild, 0, len(entries))
			for index, entry := range entries {
				index, entry := index, entry
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return p.row(inner.With(gtx), entry, &state.rows[index], entry.Name == state.selected)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(inner.Context, children...)
		})
	})
}

// nothing says why the panel is empty, which is two different facts.
//
// A directory with nothing in it and a filter that matched nothing look
// identical and mean opposite things: the first is somewhere there is nothing
// to find, and the second is somewhere there is, behind a word somebody typed.
func (p FilePickerProps) nothing(c ayra.Context, state *FilePicker) ayra.Dimensions {
	if state.Filter() != "" {
		return EmptyProps{Title: "No matches", Body: "Nothing in this directory matches the filter."}.Layout(c)
	}
	return EmptyProps{Title: "Empty", Body: "There is nothing in this directory."}.Layout(c)
}

// row draws one entry as a pressable line.
func (p FilePickerProps) row(c ayra.Context, entry FileEntry, state *Button, selected bool) ayra.Dimensions {
	draw := func(c ayra.Context) ayra.Dimensions {
		fill := c.Theme.Colours.Background
		if selected {
			fill = c.Theme.Colours.Accent
		}
		return surface(c, fill, fill, controlRadius(c), func(c ayra.Context) ayra.Dimensions {
			return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
				return p.line(c.With(gtx), entry)
			})
		})
	}

	if p.Disabled {
		c.Context = c.Context.Disabled()
		return draw(c)
	}
	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		return draw(c.With(gtx))
	})
}

// line draws the mark, the name and the size of one entry.
//
// The name and the size are laid out before the mark is drawn, because the
// mark's box is as tall as the line beside it and a flex child is never told
// how tall its siblings are. Taken from the constraints instead, that height is
// nought -- Constraints.Min.Y of a flex child is nought -- and the row collapses
// to the height of the mark, with the name drawn down over whatever the next row
// put there.
//
// The room the name is measured in is narrowed by the mark and the gap first,
// so the measurement is of the line as it will be drawn. Measured in the whole
// width and then moved across, a name long enough to fill the row would be laid
// out to an edge it never reaches.
func (p FilePickerProps) line(c ayra.Context, entry FileEntry) ayra.Dimensions {
	side := c.Dp(unit.Dp(14))
	gap := c.Dp(unit.Dp(10))

	beside := c
	beside.Constraints.Max.X = max(c.Constraints.Max.X-side-gap, 0)
	beside.Constraints.Min = image.Pt(beside.Constraints.Max.X, 0)

	measure := op.Record(c.Ops)
	dims := p.names(beside, entry)
	drawn := measure.Stop()

	ink := c.Theme.Colours.MutedForeground
	if entry.Dir {
		ink = c.Theme.Colours.Foreground
	}
	if p.Disabled {
		ink = fade(ink)
	}

	mark := entryMark(c, entry.Dir, side, dims.Size.Y, ink)

	beyond := op.Offset(image.Pt(mark.Size.X+gap, 0)).Push(c.Ops)
	drawn.Add(c.Ops)
	beyond.Pop()

	return ayra.Dimensions{Size: image.Pt(mark.Size.X+gap+dims.Size.X, mark.Size.Y)}
}

// names draws the entry's name, with its size at the far edge.
func (p FilePickerProps) names(c ayra.Context, entry FileEntry) ayra.Dimensions {
	ink := c.Theme.Colours.Foreground
	quiet := c.Theme.Colours.MutedForeground
	if p.Disabled {
		ink, quiet = fade(ink), fade(quiet)
	}

	figure := entryFigure(entry)

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return drawText(c.With(gtx), entry.Name, unit.Sp(c.Theme.Type.Body), ink, 1, text.Start, plain())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if figure == "" {
				return layout.Dimensions{}
			}
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return layout.Inset{Left: 12}.Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
				return drawText(inner.With(gtx), figure, unit.Sp(c.Theme.Type.Small), quiet, 1, text.End, mono())
			})
		}),
	)
}

// entryFigure answers what goes at the far edge of a row, and empty when
// nothing does.
//
// A directory reports no size. What could go there is the size of everything
// inside it, which is not known without opening every one of them -- and a
// nought would be a claim about a directory that may hold a great deal.
func entryFigure(entry FileEntry) string {
	if entry.Dir {
		return ""
	}
	return formatSize(entry.Size)
}

// entryMark draws the shape that says which kind of entry a row is, centred in
// a box as tall as the row, and answers the room that box took.
//
// Drawn rather than written as a character. The glyphs that would say this are
// ones many faces do not carry, and a face that has not got one draws a hollow
// box instead -- which, repeated down every row of a listing, reads as a set of
// files the application failed to identify.
//
// Both shapes take the same square, whatever they put in it. A directory mark a
// point wider than a file mark would start every directory's name a point
// further right than every file's, and a column of names that does not line up
// is the kind of wrong that is felt long before it is seen.
//
// The height is asked for rather than assumed, because the shape is smaller
// than the line it sits beside: a mark that took only its own square would sit
// against the top of the row, level with the ascenders of the name rather than
// with the word.
func entryMark(c ayra.Context, directory bool, side, height int, ink color.NRGBA) ayra.Dimensions {
	box := image.Pt(side, max(height, side))
	width := float32(side)

	offset := op.Offset(image.Pt(0, (box.Y-side)/2)).Push(c.Ops)
	defer offset.Pop()

	var shape clip.Path
	shape.Begin(c.Ops)
	if directory {
		// A folder: a body with a raised tab over its left half.
		shape.MoveTo(f32.Pt(0, width*0.22))
		shape.LineTo(f32.Pt(width*0.40, width*0.22))
		shape.LineTo(f32.Pt(width*0.50, width*0.36))
		shape.LineTo(f32.Pt(width, width*0.36))
		shape.LineTo(f32.Pt(width, width*0.84))
		shape.LineTo(f32.Pt(0, width*0.84))
	} else {
		// A page with its top corner turned down.
		shape.MoveTo(f32.Pt(width*0.18, width*0.12))
		shape.LineTo(f32.Pt(width*0.62, width*0.12))
		shape.LineTo(f32.Pt(width*0.84, width*0.34))
		shape.LineTo(f32.Pt(width*0.84, width*0.88))
		shape.LineTo(f32.Pt(width*0.18, width*0.88))
	}
	shape.Close()

	paint.FillShape(c.Ops, ink, clip.Outline{Path: shape.End()}.Op())
	return ayra.Dimensions{Size: box}
}

// sizeStep is how many bytes make the next unit.
//
// A thousand, not a thousand and twenty-four. The figure beside a file is read
// against a quota, a transfer limit and a price per gigabyte, and every one of
// those is quoted in thousands; a listing that called a file 1.0 GB where the
// invoice says 1.07 leaves somebody converting in their head to find out
// whether the two are about the same thing.
const sizeStep = 1000

// formatSize writes a count of bytes as a figure somebody reads at a glance.
//
// Below the first step the exact count is written, because a number of bytes
// that small is exact and rounding it to "1.0 kB" throws away the only
// information it had. Above it, one decimal place: two is a precision the
// figure no longer has once it has been divided, and none makes a file of
// 1.4 MB and one of 1.9 MB the same size.
func formatSize(bytes int64) string {
	if bytes < 0 {
		return ""
	}
	if bytes < sizeStep {
		return strconv.FormatInt(bytes, 10) + " B"
	}

	units := [...]string{"kB", "MB", "GB", "TB", "PB", "EB"}
	value := float64(bytes)

	for index, unit := range units {
		value /= sizeStep
		// The carry is tested at 999.95 and not at the step itself, because one
		// decimal place rounds anything above that up to 1000.0 -- and
		// "1000.0 kB" is exactly the figure the next unit exists to prevent.
		if value < 999.95 || index == len(units)-1 {
			return strconv.FormatFloat(value, 'f', 1, 64) + " " + unit
		}
	}
	return ""
}
