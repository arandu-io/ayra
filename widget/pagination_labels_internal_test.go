package widget

import (
	"strings"
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// The ends of a pager are said in the application's words.
func TestAPagerSaysItsEndsInTheApplicationsWords(t *testing.T) {
	c, _ := field(t, theme.Light, 800)
	width := func(p PaginationProps) int { return p.Layout(c, &Pages{}).Size.X }

	base := width(PaginationProps{Total: 3})
	if got := width(PaginationProps{Total: 3, PreviousLabel: strings.Repeat("W", 20)}); got <= base {
		t.Errorf("a pager with a long Previous label is %d wide, no wider than the default (%d): the label is not drawn", got, base)
	}
	if got := width(PaginationProps{Total: 3, NextLabel: strings.Repeat("W", 20)}); got <= base {
		t.Errorf("a pager with a long Next label is %d wide, no wider than the default (%d): the label is not drawn", got, base)
	}
}
