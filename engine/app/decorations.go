package app

import (
	"image"
	"image/color"

	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/io/semantic"
	"github.com/arandu-io/ayra/engine/io/system"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/theme"
)

// decorations draws the title bar this package falls back to when the platform
// does not draw one.
//
// It exists here, rather than being taken from a widget set, for the reason
// this package draws it at all: the bar is part of the window and appears
// before any application code runs. A theme brought in for it would decide
// what the first thing a person sees looks like, and it would decide it for
// every application -- including the ones that never asked for that theme.
//
// The colours come from the palette every other control on this side reads, so
// the bar and the screen under it agree.
type decorations struct {
	// state is the hit testing and the drag handling, which is shared with the
	// platform paths that do draw their own bar.
	state *widget.Decorations
	// actions are the buttons this bar offers, which the window recomputes as
	// its mode changes.
	actions system.Action
	// title is what the bar says.
	title string
	// shaper turns the title into glyphs. It is held rather than made per
	// frame because the atlas behind it is the expensive part.
	shaper *text.Shaper
	// scheme is which palette to draw with.
	scheme theme.Scheme
}

// height of one icon, and the room around and inside it.
const (
	winIconSize   = unit.Dp(20)
	winIconMargin = unit.Dp(4)
	winIconStroke = unit.Dp(2)
)

// Layout draws the bar and returns the room it took.
func (d decorations) Layout(gtx layout.Context) layout.Dimensions {
	palette := theme.New(d.scheme).Colours

	recorded := op.Record(gtx.Ops)
	dims := d.contents(gtx, palette.Foreground, palette.MutedForeground)
	drawn := recorded.Stop()

	paint.FillShape(gtx.Ops, palette.Muted, clip.Rect{Max: dims.Size}.Op())
	drawn.Add(gtx.Ops)
	return dims
}

// contents lays the title against the buttons.
func (d decorations) contents(gtx layout.Context, ink, marks color.NRGBA) layout.Dimensions {
	gtx.Constraints.Min.Y = 0
	inset := layout.UniformInset(10)
	scale := theme.New(d.scheme)

	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
		Spacing:   layout.SpaceBetween,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return d.state.LayoutMove(gtx, func(gtx layout.Context) layout.Dimensions {
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					material := op.Record(gtx.Ops)
					paint.ColorOp{Color: ink}.Add(gtx.Ops)
					stop := material.Stop()
					return widget.Label{MaxLines: 1}.Layout(gtx, d.shaper, font.Font{}, unit.Sp(scale.Type.Small), d.title, stop)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.buttons(gtx, inset, marks)
		}),
	)
}

// buttons draws one mark per action the window offers.
func (d decorations) buttons(gtx layout.Context, inset layout.Inset, marks color.NRGBA) layout.Dimensions {
	// Unmaximize is drawn by maximize, which swaps its own mark, so offering
	// both would be two buttons for one state.
	actions := d.actions &^ system.ActionUnmaximize

	var size image.Point
	for a := system.Action(1); actions != 0; a <<= 1 {
		if a&actions == 0 {
			continue
		}
		actions &^= a

		var mark layout.Widget
		switch a {
		case system.ActionMinimize:
			mark = minimizeWindow
		case system.ActionMaximize:
			mark = maximizeWindow
			if d.state.Maximized {
				mark = maximizedWindow
			}
		case system.ActionClose:
			mark = closeWindow
		default:
			continue
		}

		click := d.state.Clickable(a)
		dims := click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			system.ActionInputOp(a).Add(gtx.Ops)
			paint.ColorOp{Color: marks}.Add(gtx.Ops)
			return inset.Layout(gtx, mark)
		})

		size.X += dims.Size.X
		if size.Y < dims.Size.Y {
			size.Y = dims.Size.Y
		}
		op.Offset(image.Pt(dims.Size.X, 0)).Add(gtx.Ops)
	}
	return layout.Dimensions{Size: size}
}

// minimizeWindow draws a line.
func minimizeWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(winIconSize)
	edge := float32(size)
	margin := float32(gtx.Dp(winIconMargin))

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: margin, Y: edge - margin})
	p.LineTo(f32.Point{X: edge - 2*margin, Y: edge - margin})
	stroke := clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(winIconStroke))}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stroke.Pop()

	return layout.Dimensions{Size: image.Pt(size, size)}
}

// maximizeWindow draws a rectangle with a heavier top edge, which is a window.
func maximizeWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(winIconSize)
	margin := gtx.Dp(winIconMargin)
	width := float32(gtx.Dp(winIconStroke))

	r := clip.RRect{Rect: image.Rect(margin, margin, size-margin, size-margin)}
	stroke := clip.Stroke{Path: r.Path(gtx.Ops), Width: width}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stroke.Pop()

	r.Rect.Max = image.Pt(size-margin, 2*margin)
	filled := clip.Outline{Path: r.Path(gtx.Ops)}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	filled.Pop()

	return layout.Dimensions{Size: image.Pt(size, size)}
}

// maximizedWindow draws two overlapping rectangles, which is a window that
// would come back to a smaller one.
func maximizedWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(winIconSize)
	margin := gtx.Dp(winIconMargin)
	width := float32(gtx.Dp(winIconStroke))

	behind := clip.RRect{Rect: image.Rect(margin, margin, size-2*margin, size-2*margin)}
	stroke := clip.Stroke{Path: behind.Path(gtx.Ops), Width: width}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stroke.Pop()

	front := clip.RRect{Rect: image.Rect(2*margin, 2*margin, size-margin, size-margin)}
	stroke = clip.Stroke{Path: front.Path(gtx.Ops), Width: width}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stroke.Pop()

	return layout.Dimensions{Size: image.Pt(size, size)}
}

// closeWindow draws a cross.
func closeWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(winIconSize)
	edge := float32(size)
	margin := float32(gtx.Dp(winIconMargin))

	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: margin, Y: margin})
	p.LineTo(f32.Point{X: edge - margin, Y: edge - margin})
	p.MoveTo(f32.Point{X: edge - margin, Y: margin})
	p.LineTo(f32.Point{X: margin, Y: edge - margin})
	stroke := clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(winIconStroke))}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stroke.Pop()

	return layout.Dimensions{Size: image.Pt(size, size)}
}
