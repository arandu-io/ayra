package widget_test

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

func TestAnIconTakesTheSizeOfBodyText(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)

	got := widget.IconProps{Icon: icons.Trash()}.Layout(ctx).Size
	if want := int(ctx.Theme.Type.Body); got.X != want || got.Y != want {
		t.Errorf("an icon took %v, want %dx%d: the size of a line of text", got, want, want)
	}
	if big := (widget.IconProps{Icon: icons.Trash(), Size: 32}).Layout(ctx).Size; big.X != 32 {
		t.Errorf("a size of 32 took %v", big)
	}
	if zero := (widget.IconProps{}).Layout(ctx).Size; zero.X != 0 {
		t.Errorf("the zero icon took %v, and draws nothing", zero)
	}
}

func TestAnIconIsRestyledLikeAnyPart(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	plain := widget.IconProps{Icon: icons.Gear()}.Layout(ctx).Size
	big := widget.IconProps{Icon: icons.Gear(), ComponentProps: widget.ComponentProps{
		Parts: theme.Parts{"root": {Font: theme.Font{Size: theme.Pt(40)}}},
	}}.Layout(ctx).Size
	if big.X <= plain.X {
		t.Errorf("a type size of 40 left the icon at %v from %v", big, plain)
	}
}

func TestAButtonCarriesAnIcon(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var a, b widget.Button
	without := widget.ButtonProps{Label: "Delete"}.Layout(ctx, &a).Size
	with := widget.ButtonProps{Label: "Delete", Icon: icons.Trash()}.Layout(ctx, &b).Size
	if with.X <= without.X {
		t.Errorf("an icon before the label left the button at %v from %v", with, without)
	}

	var c widget.Button
	only := widget.ButtonProps{Icon: icons.Gear(), Size: widget.Icon}.Layout(ctx, &c).Size
	if only.X != only.Y {
		t.Errorf("an icon-only button is %v, and must be square", only)
	}
}

func picture(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 120, A: 255})
		}
	}
	return img
}

func TestAPictureTakesItsOwnProportionsOnceItHasLoaded(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var state widget.Image
	props := widget.ImageProps{Width: 200, Source: widget.ImageSourceFunc(func(context.Context) (image.Image, error) {
		return picture(400, 100), nil
	})}

	before := props.Layout(ctx, &state).Size
	waitFor(t, "the picture", func() bool { return state.Picture() != nil })
	after := props.Layout(ctx, &state).Size

	if before.Y != 200*9/16 {
		t.Errorf("while loading the box is %v, want the wide default", before)
	}
	if after.X != 200 || after.Y != 50 {
		t.Errorf("a 400x100 picture in 200 across is %v, want 200x50", after)
	}
}

func TestAPictureThatFailsSaysSo(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var state widget.Image
	props := widget.ImageProps{Alt: "Gate camera", Width: 200, Height: 100, Source: widget.ImageSourceFunc(func(context.Context) (image.Image, error) {
		return nil, errors.New("404")
	})}

	props.Layout(ctx, &state)
	waitFor(t, "the failure", func() bool { return state.Err() != nil })
	if got := props.Layout(ctx, &state).Size; got.X != 200 || got.Y != 100 {
		t.Errorf("a failed picture left its box at %v", got)
	}
}

func TestAPictureIsLoadedOnceAndNotOnEveryFrame(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var state widget.Image
	loads := make(chan struct{}, 10)
	props := widget.ImageProps{Source: widget.ImageSourceFunc(func(context.Context) (image.Image, error) {
		loads <- struct{}{}
		return picture(10, 10), nil
	})}

	for i := 0; i < 5; i++ {
		props.Layout(ctx, &state)
	}
	waitFor(t, "the picture", func() bool { return state.Picture() != nil })
	time.Sleep(10 * time.Millisecond)
	if len(loads) != 1 {
		t.Errorf("the picture was loaded %d times", len(loads))
	}
}

func TestAnAvatarDrawsItsPhotoInThePlaceOfItsInitials(t *testing.T) {
	ctx, _ := frame(t, theme.Light, 400)
	var photo widget.Image
	photo.Set(picture(64, 64))

	with := widget.AvatarProps{Initials: "PL", Photo: &photo}.Layout(ctx).Size
	without := widget.AvatarProps{Initials: "PL"}.Layout(ctx).Size
	if with != without {
		t.Errorf("a photo changed the avatar from %v to %v", without, with)
	}
}
