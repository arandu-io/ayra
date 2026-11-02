package frame_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The buttons in two states the first pictures never held: every variant
// unavailable, and the sizes side by side.
//
// Found on 02/11/2026, looking at the Alpr.sh theme at the width of a phone: the
// unavailable secondary, outline and ghost buttons in the dark scheme could not
// be told from the available ones, and the large button set its label at the
// heading size. (The focused button and the touch area are proved in
// widget/button_display_test.go: a picture has no way to move the focus or to
// show an area that answers and does not draw.)

// displayPicture draws one screen of the display widgets and compares it.
func displayPicture(t *testing.T, name string, height int, screen func() ayra.Widget) {
	t.Helper()
	for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
		for _, density := range []float32{1, 2} {
			picture := pictureName(name, scheme, density)
			t.Run(picture, func(t *testing.T) {
				got, err := frame.Draw(frame.Options{Width: 360, Height: height, Density: density, Scheme: scheme, Fonts: fonts()}, screen())
				if err != nil {
					if noSurface(err) {
						t.Skipf("no drawing surface on this machine: %v", err)
					}
					t.Fatalf("the screen did not draw: %v", err)
				}
				compare(t, picture, got)
			})
		}
	}
}

// looseRow draws widgets side by side at the width each one asks for.
func looseRow(c ayra.Context, children ...ayra.Widget) ayra.Dimensions {
	c.Constraints.Min.X = 0
	items := make([]layout.FlexChild, 0, len(children)*2)
	for i, child := range children {
		child := child
		if i > 0 {
			items = append(items, layout.Rigid(layout.Spacer{Width: 8}.Layout))
		}
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return child(c.With(gtx)) }))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context, items...)
}

func displayButtons() ayra.Widget {
	variants := []widget.Variant{widget.Default, widget.Secondary, widget.Outline, widget.Ghost, widget.Destructive, widget.Link}
	off := make([]widget.Button, len(variants))
	sizes := make([]widget.Button, 5)

	return func(c ayra.Context) ayra.Dimensions {
		var unavailable, bySize []ayra.Widget
		for i, v := range variants {
			i, v := i, v
			unavailable = append(unavailable, func(c ayra.Context) ayra.Dimensions {
				return widget.ButtonProps{Label: "Salvar", Variant: v, Size: widget.Small, Disabled: true}.Layout(c, &off[i])
			})
		}
		for i, s := range []widget.Size{widget.ExtraSmall, widget.Small, widget.Medium, widget.Large, widget.Icon} {
			i, s := i, s
			bySize = append(bySize, func(c ayra.Context) ayra.Dimensions {
				p := widget.ButtonProps{Label: "Salvar", Size: s}
				if s == widget.Icon {
					p = widget.ButtonProps{Label: "Excluir", Icon: icons.Trash(), IconOnly: true, Size: s}
				}
				return p.Layout(c, &sizes[i])
			})
		}
		return column(c, 330,
			line("Unavailable, sizes", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions { return looseRow(c, unavailable[:3]...) },
			spacer(8),
			func(c ayra.Context) ayra.Dimensions { return looseRow(c, unavailable[3:]...) },
			spacer(12),
			func(c ayra.Context) ayra.Dimensions { return looseRow(c, bySize...) },
		)
	}
}

func TestDisplayButtonsLookLikeTheyDid(t *testing.T) {
	displayPicture(t, "display-buttons", 220, displayButtons)
}
