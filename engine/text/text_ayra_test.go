package text

import (
	"testing"

	"github.com/arandu-io/ayra/engine/io/system"
	"golang.org/x/image/math/fixed"
)

func TestAlignmentNamesEveryValueWithoutPanicking(t *testing.T) {
	for _, tc := range []struct {
		alignment Alignment
		want      string
	}{
		{Start, "Start"},
		{End, "End"},
		{Middle, "Middle"},
		{Alignment(99), "Alignment(99)"},
	} {
		if got := tc.alignment.String(); got != tc.want {
			t.Errorf("alignment %d is named %q, want %q", tc.alignment, got, tc.want)
		}
	}
}

func TestAlignmentFollowsTextDirection(t *testing.T) {
	const width = 100
	textWidth := fixed.I(20)
	for _, tc := range []struct {
		alignment Alignment
		direction system.TextDirection
		want      fixed.Int26_6
	}{
		{Start, system.LTR, 0},
		{End, system.LTR, fixed.I(80)},
		{Middle, system.LTR, fixed.I(40)},
		{Start, system.RTL, fixed.I(80)},
		{End, system.RTL, 0},
		{Middle, system.RTL, fixed.I(40)},
		{Alignment(99), system.LTR, 0},
	} {
		if got := tc.alignment.Align(tc.direction, textWidth, width); got != tc.want {
			t.Errorf("%v in %v placed text at %v, want %v", tc.alignment, tc.direction, got, tc.want)
		}
	}
}
