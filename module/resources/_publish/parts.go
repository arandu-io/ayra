package native

import (
	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/widget"
)

// The pieces the screens are built from, and the lengths they are spaced with.
// They are yours: what a heading looks like in this product, and how much air
// a form has, is this product's decision and not a library's.

// The spacing this application lays out with, in points.
const (
	// gap is the room between two things that belong to the same group: a
	// label and its field, two lines of the same paragraph.
	gapSmall unit.Dp = 6
	// gap is the room between two groups: one field and the next.
	gap unit.Dp = 16
	// formWidth is as wide as a form is allowed to get. A form stretched
	// across a desktop window is a line of text a metre long; the same form on
	// a phone should use every point it has, which is what a maximum does and
	// a size does not.
	formWidth unit.Dp = 360
	// readingWidth is the same idea for a screen that is mostly text.
	readingWidth unit.Dp = 480
)

// heading draws a screen's title.
func heading(text string) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return widget.TextProps{Content: text, Role: widget.Heading, Bold: true}.Layout(c)
	}
}

// field draws one labelled input.
//
// The label is above the field and not inside it: a field whose only label is
// its placeholder loses it the moment somebody types, and what is left is a box
// with no name -- an annoyance for everyone and an impossibility for a person
// who cannot see it.
func field(name string, props widget.InputProps, state *widget.Input) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return column(c, gapSmall,
			func(c ayra.Context) ayra.Dimensions {
				return widget.TextProps{Content: name, Role: widget.Caption, Tone: widget.Muted}.Layout(c)
			},
			func(c ayra.Context) ayra.Dimensions {
				return props.Layout(c, state)
			},
		)
	}
}

// problem says what went wrong, and takes no room when nothing did.
//
// Two sources, one line: what the server said about the values, and what the
// attempt itself answered with. They are different failures -- a password that
// was refused, and a server that could not be reached -- and a person holding a
// phone with no signal needs to be told which one they have.
func problem(failure error, fromServer string) ayra.Widget {
	message := fromServer
	if failure != nil {
		message = describe(failure)
	}

	return func(c ayra.Context) ayra.Dimensions {
		if message == "" {
			return ayra.Dimensions{}
		}
		return widget.TextProps{Content: message, Role: widget.Caption, Tone: widget.Danger}.Layout(c)
	}
}

// describe turns a failure into something worth reading on a screen.
//
// The refusals the server can answer with are named, because each is a
// different thing for the person to do next. Anything else came from the
// transport, and there the original message is the useful part -- it is the one
// that says the name did not resolve or the certificate was refused.
func describe(err error) string {
	status := client.Status(err)
	switch {
	case status == 401:
		return "Those details were not accepted."
	case status == 403:
		return "This account cannot do that."
	case status == 404:
		return "That is no longer here."
	case status == 429:
		return "Too many attempts. Wait a moment and try again."
	case status >= 500:
		return "The server had a problem. Try again."
	}
	return err.Error()
}

// centred draws a child no wider than width, in the middle of the room there is.
func centred(c ayra.Context, width unit.Dp, child ayra.Widget) ayra.Dimensions {
	return layout.Center.Layout(c.Context, func(gtx layout.Context) ayra.Dimensions {
		if max := c.Dp(width); gtx.Constraints.Max.X > max {
			gtx.Constraints.Max.X = max
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return child(c.With(gtx))
	})
}

// column stacks children with the same room between each.
func column(c ayra.Context, between unit.Dp, children ...ayra.Widget) ayra.Dimensions {
	items := make([]layout.FlexChild, 0, len(children)*2)
	for i, child := range children {
		child := child
		if i > 0 {
			items = append(items, layout.Rigid(layout.Spacer{Height: between}.Layout))
		}
		items = append(items, layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
			return child(c.With(gtx))
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, items...)
}
