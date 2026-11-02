package widget_test

import (
	"image"
	"slices"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// picture is a frame of a given size, as a decoder would leave one.
func videoPicture(w, h int, shade uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = shade, shade, shade, 255
	}
	return img
}

func TestALiveVideoWithNoPictureHoldsASixteenByNineShape(t *testing.T) {
	c, _ := frame(t, theme.Light, 640)
	var state widget.Video

	dims := widget.VideoProps{}.Layout(c, &state)

	if dims.Size.X != 640 || dims.Size.Y != 360 {
		t.Errorf("an empty live video took %v, want 640x360", dims.Size)
	}
}

func TestALiveVideoTakesTheShapeOfItsPicture(t *testing.T) {
	c, _ := frame(t, theme.Light, 640)
	var state widget.Video

	dims := widget.VideoProps{Frame: videoPicture(400, 300, 90), Seq: 1}.Layout(c, &state)

	if dims.Size.X != 640 || dims.Size.Y != 480 {
		t.Errorf("a 4:3 picture took %v, want 640x480", dims.Size)
	}
}

// TestALiveVideoNeverCropsToFitTheRoom: when the room is shorter than the
// picture's shape needs, the video gets smaller, not cut.
func TestALiveVideoNeverCropsToFitTheRoom(t *testing.T) {
	c, _ := frame(t, theme.Light, 1000)
	c.Constraints.Max.Y = 200
	var state widget.Video

	dims := widget.VideoProps{Frame: videoPicture(1920, 1080, 10), Seq: 1}.Layout(c, &state)

	if dims.Size.Y > 200 || dims.Size.X*9 != dims.Size.Y*16 && (dims.Size.X*9-dims.Size.Y*16 > 16 || dims.Size.Y*16-dims.Size.X*9 > 16) {
		t.Errorf("a 16:9 picture in 1000x200 took %v", dims.Size)
	}
	if dims.Size.X > 1000 {
		t.Errorf("the video is wider than the room: %v", dims.Size)
	}
}

func TestSizeHoldsTheShapeUntilThePictureArrives(t *testing.T) {
	c, _ := frame(t, theme.Light, 600)
	var state widget.Video

	dims := widget.VideoProps{Size: image.Pt(4, 3)}.Layout(c, &state)

	if dims.Size.X != 600 || dims.Size.Y != 450 {
		t.Errorf("a 4:3 placeholder took %v", dims.Size)
	}
}

func TestADegenerateRoomDrawsNothingRatherThanFailing(t *testing.T) {
	c, _ := frame(t, theme.Light, 0)
	c.Constraints.Max.Y = 0
	var state widget.Video

	if dims := (widget.VideoProps{}).Layout(c, &state); dims.Size.X != 0 || dims.Size.Y != 0 {
		t.Errorf("no room still took %v", dims.Size)
	}
}

// TestTheOverlayIsToldWhereThePictureWas is what lets the application draw the
// boxes of the plates: they arrive in the camera's pixels, and the widget says
// how those map onto the screen.
func TestTheOverlayIsToldWhereThePictureWas(t *testing.T) {
	c, _ := frame(t, theme.Light, 640)
	var state widget.Video
	var got widget.VideoArea
	calls := 0

	dims := widget.VideoProps{
		Frame: videoPicture(1280, 720, 40), Seq: 1,
		Overlay: func(c ayra.Context, area widget.VideoArea) {
			got = area
			calls++
			if c.Constraints.Max != area.Picture.Size() {
				t.Errorf("the overlay's room is %v, want the picture's %v", c.Constraints.Max, area.Picture.Size())
			}
		},
	}.Layout(c, &state)

	if calls != 1 {
		t.Fatalf("the overlay ran %d times", calls)
	}
	if got.Picture.Size() != dims.Size || got.Source != image.Pt(1280, 720) {
		t.Errorf("area %+v for dims %v", got, dims.Size)
	}
	if got.Scale() != 0.5 {
		t.Errorf("scale %v, want 0.5", got.Scale())
	}
	if box := got.Rect(image.Rect(100, 200, 300, 260)); box != image.Rect(50, 100, 150, 130) {
		t.Errorf("a box in the camera's pixels mapped to %v", box)
	}
	if (widget.VideoArea{}).Scale() != 0 {
		t.Error("an empty area has a scale")
	}
}

func TestACaptionOverAPictureIsDrawnAndTheVideoIsAnnounced(t *testing.T) {
	var state widget.Video
	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		return widget.VideoProps{
			Frame: videoPicture(160, 90, 200), Seq: 1,
			Label: "Gate camera, live", Caption: "No signal from the camera",
		}.Layout(c, &state)
	})

	said := spoken(nodes)
	if !slices.Contains(said, "Gate camera, live") {
		t.Errorf("the video was not announced; the tree says %v", said)
	}
	if !slices.Contains(said, "No signal from the camera") {
		t.Errorf("the caption was not read; the tree says %v", said)
	}
}

func TestAVideoWithNoLabelAnnouncesNothingOfItsOwn(t *testing.T) {
	var state widget.Video
	nodes := tree(t, func(c ayra.Context) ayra.Dimensions {
		return widget.VideoProps{Frame: videoPicture(160, 90, 200), Seq: 1}.Layout(c, &state)
	})
	if said := spoken(nodes); len(said) != 0 {
		t.Errorf("an unlabelled video announced %v", said)
	}
}
