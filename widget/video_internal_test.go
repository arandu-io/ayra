package widget

import (
	"image"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

func liveContext() ayra.Context {
	return ayra.Context{
		Context: layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(320, 1000)},
		},
		Theme:  theme.New(theme.Light),
		Shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// TestTheTextureIsMadeOncePerPicture is the cost the widget exists to keep
// down: a brush made from a picture is uploaded to the GPU, and a stream that
// has not produced a new picture must draw the one it has.
func TestTheTextureIsMadeOncePerPicture(t *testing.T) {
	var state Video
	img := image.NewRGBA(image.Rect(0, 0, 32, 18))

	VideoProps{Frame: img, Seq: 1}.Layout(liveContext(), &state)
	first := state.brush
	if first.Size() != image.Pt(32, 18) {
		t.Fatalf("no texture was made: %v", first.Size())
	}

	VideoProps{Frame: img, Seq: 1}.Layout(liveContext(), &state)
	if state.brush != first {
		t.Error("the same picture was made into a new texture")
	}

	other := image.NewRGBA(image.Rect(0, 0, 32, 18))
	VideoProps{Frame: other, Seq: 2}.Layout(liveContext(), &state)
	if state.brush == first {
		t.Error("a new picture reused the old texture")
	}
}

// TestAReusedBufferWithANewSeqIsMadeAgain: a decoder writes into buffers it
// recycles, so the pointer alone says nothing about whether the pixels changed.
func TestAReusedBufferWithANewSeqIsMadeAgain(t *testing.T) {
	var state Video
	img := image.NewRGBA(image.Rect(0, 0, 32, 18))

	VideoProps{Frame: img, Seq: 1}.Layout(liveContext(), &state)
	first := state.brush
	VideoProps{Frame: img, Seq: 2}.Layout(liveContext(), &state)
	if state.brush == first {
		t.Error("a buffer rewritten for a new picture was not uploaded again")
	}
}

func TestLosingThePictureDropsTheTexture(t *testing.T) {
	var state Video
	VideoProps{Frame: image.NewRGBA(image.Rect(0, 0, 32, 18)), Seq: 1}.Layout(liveContext(), &state)
	VideoProps{}.Layout(liveContext(), &state)
	if state.brush.Size() != (image.Point{}) || state.img != nil {
		t.Error("the texture outlived the picture")
	}
}
