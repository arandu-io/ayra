package widget

import (
	"image"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Align is where a cell's content sits in its column.
//
// It exists for one reason: a column of figures read down is only legible when
// the digits line up, which means right-aligned and tabular. A table that
// aligned everything left is a table nobody can add up by eye.
type Align uint8

const (
	// Start is the reading edge. Zero, because most columns are words.
	Start Align = iota
	// Middle centres, for a short status or a mark.
	Middle
	// End is the far edge, and is where a number goes.
	End
)

// alignment answers the text alignment this maps to.
func (a Align) alignment() text.Alignment {
	switch a {
	case Middle:
		return text.Middle
	case End:
		return text.End
	}
	return text.Start
}

// Column is one column of a table.
type Column struct {
	// Title is the heading.
	Title string
	// Align is where the content of the column sits. A column of figures ends
	// at the same edge on every row, which is what makes the digits line up.
	Align Align
	// Flex is the share of the leftover width this column takes. Zero takes an
	// equal share with the other zeroes.
	Flex float32
	// Numeric draws the cells in the face whose digits are all one width, so a
	// column of them lines up whatever the values are.
	Numeric bool
}

// TableProps is a grid of values with a heading.
//
// It draws what it is given and holds no rows of its own: a table that owned
// its data would need the whole set in memory to draw the ten lines somebody
// can see. What it does own is the shape -- the columns, their widths and their
// alignment -- which is what every row has to agree about.
type TableProps struct {
	// Columns are the headings and how each one is drawn.
	Columns []Column
	// Rows are the values, one slice per row, in column order. A row with
	// fewer values than there are columns draws blanks rather than failing:
	// the alternative is a page that will not render because one record is
	// missing a field.
	Rows [][]string
	// Dense halves the room above and below each cell, for a table read rather than
	// operated.
	Dense bool
}

// Layout draws the table and returns the room it took.
func (p TableProps) Layout(c ayra.Context) ayra.Dimensions {
	padding := unit.Dp(12)
	if p.Dense {
		padding = 6
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.row(c.With(gtx), p.titles(), padding, true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return SeparatorProps{}.Layout(c.With(gtx))
		}),
	}

	for index, values := range p.Rows {
		index, values := index, values
		if index > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SeparatorProps{}.Layout(c.With(gtx))
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.row(c.With(gtx), values, padding, false)
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}

// titles answers the heading as a row of values.
func (p TableProps) titles() []string {
	titles := make([]string, len(p.Columns))
	for index, column := range p.Columns {
		titles[index] = column.Title
	}
	return titles
}

// row draws one line of the table.
func (p TableProps) row(c ayra.Context, values []string, padding unit.Dp, heading bool) ayra.Dimensions {
	children := make([]layout.FlexChild, 0, len(p.Columns))

	for index, column := range p.Columns {
		index, column := index, column

		value := ""
		if index < len(values) {
			value = values[index]
		}

		share := column.Flex
		if share == 0 {
			share = 1
		}

		children = append(children, layout.Flexed(share, func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Inset{Top: padding, Bottom: padding, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ink, face := inner.Theme.Colours.Foreground, plain()
				if heading {
					ink, face = inner.Theme.Colours.MutedForeground, semibold()
				} else if column.Numeric {
					face = mono()
				}

				size := unit.Sp(inner.Theme.Type.Body)
				if heading {
					size = unit.Sp(inner.Theme.Type.Small)
				}
				return drawText(inner.With(gtx), value, size, ink, 1, column.Align.alignment(), face)
			})
		}))
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, children...)
}

// ToolbarProps is a row of controls above the thing they act on.
//
// It is a layout rather than a set of buttons: what it owns is the rule under
// it and the room around it, so every toolbar in a product sits the same
// distance from what it commands.
type ToolbarProps struct {
	// Divided draws the rule under it. A toolbar over a table wants one; one
	// inside a card that already has a border does not.
	Divided bool
}

// Layout draws the row, with whatever the caller puts at each end.
func (p ToolbarProps) Layout(c ayra.Context, leading, trailing ayra.Widget) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			return layout.Inset{Top: 8, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if leading == nil {
							return layout.Dimensions{}
						}
						return leading(inner.With(gtx))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if trailing == nil {
							return layout.Dimensions{}
						}
						side := inner.With(gtx)
						side.Constraints.Min = image.Point{}
						return trailing(side)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !p.Divided {
				return layout.Dimensions{}
			}
			return SeparatorProps{}.Layout(c.With(gtx))
		}),
	)
}
