package frame_test

import (
	"flag"
	"image"
	gocolor "image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// update rewrites the approved pictures instead of comparing against them.
//
// Reviewing the change is the point of this whole gate, so it is a flag and
// never an environment variable: a flag appears in the command somebody typed
// and in the shell history afterwards, and cannot be left set in a profile
// where it silently approves everything for a year.
var update = flag.Bool("update", false, "rewrite the approved pictures")

// The pictures are drawn with the fonts this repository carries rather than
// with whatever the machine has installed. A golden file compared against a
// system font is a golden file that passes on the machine it was made on and
// fails everywhere else, which teaches everyone to run with -update.
func fonts() []text.FontFace { return gofont.Collection() }

// TestScreensLookLikeTheyDid draws each screen and compares it with the
// picture that was approved.
//
// This is the gate a build does not give. Everything below the layout call --
// the shaper finding a face, a control computing a size, the GPU accepting the
// operations, a colour reaching the pixels it was meant to reach -- compiles
// whether or not it works. Three real faults were found the first time these
// pictures were looked at: nothing painted the window's ground, so the dark
// scheme drew light text on white; a button's label sat against its left edge;
// and the text field drew its placeholder through the button's own function,
// so centring one centred the other.
func TestScreensLookLikeTheyDid(t *testing.T) {
	for _, screen := range screens() {
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			for _, density := range []float32{1, 2} {
				name := pictureName(screen.name, scheme, density)

				t.Run(name, func(t *testing.T) {
					got, err := frame.Draw(frame.Options{
						Width:   screen.width,
						Height:  screen.height,
						Density: density,
						Scheme:  scheme,
						Fonts:   fonts(),
					}, screen.draw())
					if err != nil {
						if noSurface(err) {
							t.Skipf("no drawing surface on this machine: %v", err)
						}
						t.Fatalf("the screen did not draw: %v", err)
					}

					compare(t, name, got)
				})
			}
		}
	}
}

// screen is one picture this gate keeps.
//
// draw returns a fresh widget per picture rather than one shared across them,
// because a control holds its state: the same button drawn into four pictures
// is one button that was hovered in the first of them.
type screen struct {
	name          string
	width, height int
	draw          func() ayra.Widget
}

func screens() []screen {
	return []screen{
		{name: "sign-in", width: 360, height: 420, draw: signIn},
		{name: "controls", width: 360, height: 560, draw: controls},
	}
}

// signIn is the screen a person meets first.
func signIn() ayra.Widget {
	var email, password widget.Input
	var submit widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 300,
			line("Sign in", widget.Heading, true),
			spacer(16),
			field(widget.InputProps{Placeholder: "you@example.com", Kind: widget.Email}, &email),
			spacer(12),
			field(widget.InputProps{Placeholder: "Your password", Kind: widget.Password}, &password),
			spacer(16),
			button(widget.ButtonProps{Label: "Sign in", Size: widget.Large}, &submit),
		)
	}
}

// controls draws one of every variant, which is what catches a colour that
// reads on one scheme and vanishes on the other.
func controls() ayra.Widget {
	states := make([]widget.Button, 7)
	var disabled, invalid widget.Input

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Controls", widget.Heading, true),
			spacer(12),
			button(widget.ButtonProps{Label: "Default"}, &states[0]),
			spacer(8),
			button(widget.ButtonProps{Label: "Secondary", Variant: widget.Secondary}, &states[1]),
			spacer(8),
			button(widget.ButtonProps{Label: "Outline", Variant: widget.Outline}, &states[2]),
			spacer(8),
			button(widget.ButtonProps{Label: "Ghost", Variant: widget.Ghost}, &states[3]),
			spacer(8),
			button(widget.ButtonProps{Label: "Destructive", Variant: widget.Destructive}, &states[4]),
			spacer(8),
			button(widget.ButtonProps{Label: "Link", Variant: widget.Link}, &states[5]),
			spacer(8),
			button(widget.ButtonProps{Label: "Unavailable", Disabled: true}, &states[6]),
			spacer(16),
			field(widget.InputProps{Placeholder: "Unavailable", Disabled: true}, &disabled),
			spacer(8),
			field(widget.InputProps{Placeholder: "Rejected", Invalid: true}, &invalid),
		)
	}
}

// The pieces the screens above are built from. They are the test's own, and
// not the library's: a helper shared with the code under test would draw the
// same mistake on both sides of the comparison.

func line(content string, role widget.Role, bold bool) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return widget.TextProps{Content: content, Role: role, Bold: bold}.Layout(c)
	}
}

func field(props widget.InputProps, state *widget.Input) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

func button(props widget.ButtonProps, state *widget.Button) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions { return props.Layout(c, state) }
}

func spacer(height unit.Dp) ayra.Widget {
	return func(c ayra.Context) ayra.Dimensions {
		return layout.Spacer{Height: height}.Layout(c.Context)
	}
}

func column(c ayra.Context, width unit.Dp, children ...ayra.Widget) ayra.Dimensions {
	return layout.Center.Layout(c.Context, func(gtx layout.Context) ayra.Dimensions {
		if max := c.Dp(width); gtx.Constraints.Max.X > max {
			gtx.Constraints.Max.X = max
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		inner := c.With(gtx)

		items := make([]layout.FlexChild, 0, len(children))
		for _, child := range children {
			child := child
			items = append(items, layout.Rigid(func(gtx layout.Context) ayra.Dimensions {
				return child(inner.With(gtx))
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(inner.Context, items...)
	})
}

// pictureName is what a picture is filed under: the screen, the scheme and the
// density, in that order, so the directory sorts by screen.
func pictureName(screen string, scheme theme.Scheme, density float32) string {
	suffix := "1x"
	if density == 2 {
		suffix = "2x"
	}
	return screen + "-" + scheme.String() + "-" + suffix
}

// compare checks a drawing against the approved picture, or writes it when
// -update was passed.
func compare(t *testing.T, name string, got *image.RGBA) {
	t.Helper()

	path := filepath.Join("testdata", name+".png")

	if *update {
		write(t, path, got)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("%s has no approved picture: draw it with `go test ./frame -update` and look at it before committing", name)
	}
	defer file.Close()

	decoded, err := png.Decode(file)
	if err != nil {
		t.Fatalf("%s could not be read: %v", path, err)
	}

	if diff := difference(got, decoded); diff > 0 {
		failed := filepath.Join("testdata", name+".failed.png")
		write(t, failed, got)
		t.Errorf("%s changed in %d pixels.\n  approved: %s\n  drawn:    %s\nLook at both. If the change is the one you meant, `go test ./frame -update`.",
			name, diff, path, failed)
	}
}

// tolerance is how far one channel may differ before a pixel counts as
// changed, out of 255.
//
// It is not a comfort margin, it is the width of a measured gap. The same
// screens drawn by this machine's GPU and by the software renderer a build
// server uses differ in five percent of their pixels, and the measurement of
// that difference is decisive: ninety-nine percent of it is three or less, the
// largest is nine, and across all eight pictures not one pixel differs by more
// than sixteen. A real change is not in that range at all -- text that moved,
// a ground that was not painted and a rule under a word are all whole colours
// replacing whole colours.
//
// Comparing exactly would mean approved pictures that pass only on the machine
// that drew them, which teaches everyone to run with -update, which is the same
// as having no gate. Comparing loosely by a percentage of the picture would
// hide the small true changes: the rule under a link is twenty-seven pixels.
const tolerance = 16

// difference counts the pixels that do not match.
//
// A count rather than a boolean, because the number is the first thing worth
// knowing: four pixels is an anti-aliased edge that moved, and forty thousand
// is a screen that went blank.
func difference(got *image.RGBA, want image.Image) int {
	if got.Bounds() != want.Bounds() {
		return got.Bounds().Dx() * got.Bounds().Dy()
	}

	var count int
	bounds := got.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if changed(got.At(x, y), want.At(x, y)) {
				count++
			}
		}
	}
	return count
}

// changed reports whether two pixels differ by more than the tolerance in any
// channel.
func changed(got, want gocolor.Color) bool {
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()

	for _, pair := range [4][2]uint32{{gr, wr}, {gg, wg}, {gb, wb}, {ga, wa}} {
		// RGBA answers sixteen bits per channel; the tolerance is in eight, so
		// both sides come down before the subtraction rather than the constant
		// going up, which would compare a rounded number against an exact one.
		first, second := int(pair[0]>>8), int(pair[1]>>8)
		if first-second > tolerance || second-first > tolerance {
			return true
		}
	}
	return false
}

// write saves a picture, creating the directory if this is the first one.
func write(t *testing.T, path string, picture *image.RGBA) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
}

// noSurface reports whether the failure is a machine with nowhere to draw
// rather than a fault in the drawing.
//
// A machine without a GPU cannot run this gate, and there is no software
// rasteriser to fall back to. Skipping says so; failing would teach whoever
// hits it that the gate is unreliable, which is how a red suite gets ignored.
func noSurface(err error) bool {
	return strings.Contains(err.Error(), "no drawing surface")
}
