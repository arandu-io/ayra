package widget

import (
	"image"
	"image/color"
	"strings"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// FigureProps is something shown with a caption under it.
//
// The caption is part of the figure rather than a line somebody puts after it,
// because a caption that drifts from what it describes is a caption about the
// wrong thing -- and in a column of them, drifting by one is the failure that
// reads as correct.
type FigureProps struct {
	// Caption is the line under it.
	Caption string
	// Bordered draws a rule around the content, for something whose own edges
	// are pale.
	Bordered bool
}

// Layout draws the content with its caption.
func (p FigureProps) Layout(c ayra.Context, content ayra.Widget) ayra.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			if !p.Bordered {
				return content(inner)
			}
			return surface(inner, inner.Theme.Colours.Background, inner.Theme.Colours.Border, controlRadius(inner), content)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.Caption == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), p.Caption, unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 0, text.Start, plain())
			})
		}),
	)
}

// HighlightProps is a line with one part of it marked.
//
// It is what a search result is drawn with, and the marking is done by this
// rather than by the caller splitting the string: a caller that split it would
// decide what matching means, and two screens would decide differently.
type HighlightProps struct {
	// Text is the whole line.
	Text string
	// Match is the part to mark. Empty marks nothing, which is what a result
	// with no query should look like.
	Match string
	// Role is what size the line is set at.
	Role Role
}

// Layout draws the line with the match behind a wash.
func (p HighlightProps) Layout(c ayra.Context) ayra.Dimensions {
	size := p.Role.size(c.Theme)
	ink := c.Theme.Colours.Foreground

	before, match, after := p.split()
	if match == "" {
		return drawText(c, p.Text, size, ink, 1, text.Start, plain())
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return drawText(inner, before, size, ink, 1, text.Start, plain())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return marked(inner, match, size, ink)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			inner.Constraints.Min = image.Point{}
			return drawText(inner, after, size, ink, 1, text.Start, plain())
		}),
	)
}

// split answers the line in three parts, around the first match.
//
// Case-insensitive, because a search that only matched the capitalisation
// somebody typed would mark nothing on most results -- and one that marked
// nothing looks like a search that found the wrong thing.
func (p HighlightProps) split() (before, match, after string) {
	if p.Match == "" {
		return p.Text, "", ""
	}
	at := strings.Index(strings.ToLower(p.Text), strings.ToLower(p.Match))
	if at < 0 {
		return p.Text, "", ""
	}
	return p.Text[:at], p.Text[at : at+len(p.Match)], p.Text[at+len(p.Match):]
}

// marked draws the matched part over a wash.
//
// A background rather than a colour on the letters, because the letters have to
// stay the colour of the line around them: a match drawn in a second colour
// reads as a different kind of word rather than as the same word, found.
func marked(c ayra.Context, match string, size unit.Sp, ink color.NRGBA) ayra.Dimensions {
	measure := op.Record(c.Ops)
	dims := drawText(c, match, size, ink, 1, text.Start, semibold())
	drawn := measure.Stop()

	paint.FillShape(c.Ops, wash(c.Theme.Colours.Primary), clip.Rect(image.Rectangle{Max: dims.Size}).Op())
	drawn.Add(c.Ops)
	return dims
}

// FeedProps is a column of entries in time order.
//
// It draws the rail and the marks, and the entries are the caller's: what goes
// in one is a record, an event or a message, and none of those is something a
// control should know the shape of.
type FeedProps struct {
	// Count is how many entries there are.
	Count int
}

// Layout draws each entry against the rail.
func (p FeedProps) Layout(c ayra.Context, entry func(int) ayra.Widget) ayra.Dimensions {
	children := make([]layout.FlexChild, 0, p.Count)

	for index := 0; index < p.Count; index++ {
		index := index
		last := index == p.Count-1

		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)

			// The entry is measured before the rail is drawn, because the line
			// down to the next mark has to be as tall as what it runs beside --
			// and a flex child cannot ask its sibling. Read from the
			// constraints instead it is zero, and the picture showed marks with
			// nothing joining them.
			rails := inner.Dp(unit.Dp(16))
			body := entry(index)

			measure := op.Record(inner.Ops)
			beside := inner
			beside.Constraints.Max.X = max(beside.Constraints.Max.X-rails-inner.Dp(unit.Dp(12)), 0)
			beside.Constraints.Min.X = beside.Constraints.Max.X

			size := image.Point{}
			if body != nil {
				size = layout.Inset{Bottom: 16}.Layout(beside.Context, func(gtx layout.Context) layout.Dimensions {
					return body(beside.With(gtx))
				}).Size
			}
			drawn := measure.Stop()

			railed := inner
			railed.Constraints.Min.Y = size.Y
			rail(railed, last, size.Y)

			offset := op.Offset(image.Pt(rails+inner.Dp(unit.Dp(12)), 0)).Push(inner.Ops)
			drawn.Add(inner.Ops)
			offset.Pop()

			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, size.Y)}
		}))
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}

// rail draws the mark and the line down to the next entry.
func rail(c ayra.Context, last bool, height int) ayra.Dimensions {
	dot := c.Dp(unit.Dp(8))
	width := c.Dp(unit.Dp(16))

	centre := width / 2
	mark := image.Rect(centre-dot/2, 0, centre+dot/2, dot)
	paint.FillShape(c.Ops, c.Theme.Colours.Border, clip.UniformRRect(mark, dot/2).Op(c.Ops))

	if !last && height > dot {
		// The line stops at the last entry. One drawn past it is a feed that
		// says there is more below when there is not.
		thickness := max(hairline(c), 1)
		line := image.Rect(centre-thickness/2, dot, centre+thickness/2, height)
		paint.FillShape(c.Ops, c.Theme.Colours.Border, clip.Rect(line).Op())
	}

	return ayra.Dimensions{Size: image.Pt(width, max(height, dot))}
}
