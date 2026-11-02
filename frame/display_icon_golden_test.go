package frame_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/widget"
)

// An icon set beside the word it belongs to, in a row aligned by baseline, at the
// three sizes of text. Found on 02/11/2026: the icon reported its baseline at its
// top edge, so the whole square hung below the line of the word.
func displayIconLine() ayra.Widget {
	next := func(c ayra.Context, role widget.Role, icon icons.Icon, label string) ayra.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return widget.IconProps{Icon: icon}.Layout(c.With(gtx)) }),
			layout.Rigid(layout.Spacer{Width: 6}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.TextProps{Content: label, Role: role}.Layout(c.With(gtx))
			}),
		)
	}
	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 330,
			line("Icon on the line", widget.Heading, true),
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return next(c, widget.Body, icons.Trash(), "Excluir a leitura, gjpqy")
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return next(c, widget.Caption, icons.Info(), "Detalhe da câmera, gjpqy")
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return next(c, widget.Heading, icons.Check(), "Câmera conectada")
			},
		)
	}
}

func TestDisplayIconLineLooksLikeItDid(t *testing.T) {
	displayPicture(t, "display-icon-line", 200, displayIconLine)
}
