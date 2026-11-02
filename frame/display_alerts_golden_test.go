package frame_test

import (
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/widget"
)

// A failure alert beside a note, on the surfaces the pages are drawn on. Found on
// 02/11/2026: the failure was the red over a tint of the same red, and its small
// message was under the contrast a line that size needs, in both schemes.
func displayAlerts() ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 330,
			line("Alerts", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AlertProps{Title: "Não foi possível salvar", Body: "A leitura não foi gravada: o formato da placa é inválido.", Severity: widget.Failure}.Layout(c)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AlertProps{Title: "Câmera conectada", Body: "O enquadramento já está ao vivo."}.Layout(c)
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return widget.AlertProps{Title: "Sem conexão com o servidor", Severity: widget.Failure}.Layout(c)
			},
		)
	}
}

func TestDisplayAlertsLookLikeTheyDid(t *testing.T) {
	displayPicture(t, "display-alerts", 300, displayAlerts)
}
