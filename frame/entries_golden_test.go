package frame_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/mask"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The widgets that arrived with the parity work have pictures of their own: the
// icons, whose paths are parsed from the same data the web half draws, the
// pictures and avatars, the masked fields and the row of squares.

func iconRow(c ayra.Context, set ...icons.Icon) ayra.Dimensions {
	children := make([]layout.FlexChild, 0, len(set)*2)
	for _, icon := range set {
		icon := icon
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return widget.IconProps{Icon: icon, Size: 26}.Layout(c.With(gtx))
		}), layout.Rigid(layout.Spacer{Width: 10}.Layout))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context, children...)
}

func gradient(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			stripe := uint8(0)
			if (x/16+y/16)%2 == 0 {
				stripe = 40
			}
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 150 + stripe/2, A: 255})
		}
	}
	return img
}

func visuals() ayra.Widget {
	var photo, wide, tall widget.Image
	photo.Set(gradient(96, 96))
	wide.Set(gradient(240, 90))
	tall.Set(gradient(90, 240))
	var save, only widget.Button

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Icons", widget.Heading, true),
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return iconRow(c, icons.Trash(), icons.Gear(), icons.Camera(), icons.MagnifyingGlass(), icons.Check(), icons.Warning(), icons.Car(), icons.Bell())
			},
			spacer(8),
			func(c ayra.Context) ayra.Dimensions {
				return iconRow(c, icons.ArrowRight(), icons.CloudArrowUp(), icons.Funnel(), icons.House(), icons.User(), icons.Lock(), icons.Eye(), icons.Pencil())
			},
			spacer(10),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.ButtonProps{Label: "Save", Icon: icons.Check()}.Layout(c.With(gtx), &save)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.ButtonProps{Icon: icons.Gear(), Size: widget.Icon, Variant: widget.Outline}.Layout(c.With(gtx), &only)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.AvatarProps{Initials: "PL", Photo: &photo}.Layout(c.With(gtx))
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.AvatarProps{Initials: "PL"}.Layout(c.With(gtx))
					}),
				)
			},
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(c.Context,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.ImageProps{Width: 150, Height: 90, Alt: "cover"}.Layout(c.With(gtx), &wide)
					}),
					layout.Rigid(layout.Spacer{Width: 10}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return widget.ImageProps{Width: 150, Height: 90, Fit: widget.Contain, Alt: "contain"}.Layout(c.With(gtx), &tall)
					}),
				)
			},
		)
	}
}

func entries() ayra.Widget {
	var code widget.OneTimeCode
	code.Set("4829")
	var cpf, phone, plate, money, date widget.Masked
	cpf.SetValue(brazil.CPF, "12345678909")
	phone.SetValue(brazil.Phone, "11987654321")
	plate.SetValue(brazil.Plate, "ABC1D23")
	money.SetValue(brazil.Real, "123456")
	date.SetValue(mask.Date, "10092026")

	field := func(label string, by mask.Mask, state *widget.Masked) ayra.Widget {
		return func(c ayra.Context) ayra.Dimensions {
			return widget.FieldProps{Label: label}.Layout(c, func(c ayra.Context) ayra.Dimensions {
				return widget.MaskedProps{Mask: by}.Layout(c, state)
			})
		}
	}

	return func(c ayra.Context) ayra.Dimensions {
		return column(c, 320,
			line("Entries", widget.Heading, true),
			spacer(8),
			field("CPF", brazil.CPF, &cpf),
			spacer(8),
			field("Phone", brazil.Phone, &phone),
			spacer(8),
			field("Plate", brazil.Plate, &plate),
			spacer(8),
			field("Amount", brazil.Real, &money),
			spacer(8),
			field("Date", mask.Date, &date),
			spacer(12),
			func(c ayra.Context) ayra.Dimensions {
				return widget.OneTimeCodeProps{}.Layout(c, &code)
			},
		)
	}
}

func TestTheParityScreensLookLikeTheyDid(t *testing.T) {
	for _, screen := range []screen{
		{name: "visuals", width: 360, height: 400, draw: visuals},
		{name: "entries", width: 360, height: 500, draw: entries},
	} {
		for _, scheme := range []theme.Scheme{theme.Light, theme.Dark} {
			for _, density := range []float32{1, 2} {
				name := pictureName(screen.name, scheme, density)
				t.Run(name, func(t *testing.T) {
					got, err := frame.Draw(frame.Options{
						Width: screen.width, Height: screen.height, Density: density, Scheme: scheme, Fonts: fonts(),
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
