package live

import (
	"image"
	"testing"
)

// rgbToYUV is the forward transform of the same standard and range the decoder
// reverses, so a round trip is the test.
func rgbToYUV(r, g, b int, bt709 bool) (y, u, v int) {
	var kr, kb float64
	if bt709 {
		kr, kb = 0.2126, 0.0722
	} else {
		kr, kb = 0.299, 0.114
	}
	kg := 1 - kr - kb
	fy := kr*float64(r) + kg*float64(g) + kb*float64(b)
	fu := (float64(b) - fy) / (2 * (1 - kb))
	fv := (float64(r) - fy) / (2 * (1 - kr))
	return int(16 + 219*fy/255 + 0.5), int(128 + 224*fu/255 + 0.5), int(128 + 224*fv/255 + 0.5)
}

// scene is a picture with flat areas of strongly different colours, so that a
// chroma sample read from the wrong place is a visible error.
func scene(w, h int) *image.RGBA {
	colours := [][3]int{{235, 30, 30}, {30, 200, 60}, {40, 60, 230}, {240, 240, 240}, {20, 20, 20}, {220, 180, 30}}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := colours[((x/8)+(y/8))%len(colours)]
			i := y*img.Stride + x*4
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(c[0]), uint8(c[1]), uint8(c[2]), 255
		}
	}
	return img
}

// encode lays a picture out in planes the way a decoder would, with padding in
// the rows and an optional crop, and interleaved chroma when asked.
func encode(src *image.RGBA, bt709, interleaved bool, cropX, cropY, padY, padUV int) yuvPlanes {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	codedW, codedH := w+cropX+6, h+cropY+4
	codedW, codedH = codedW+codedW%2, codedH+codedH%2
	yStride := codedW + padY
	p := yuvPlanes{
		yStride: yStride, cropX: cropX, cropY: cropY, width: w, height: h, bt709: bt709,
		y: make([]byte, yStride*codedH),
	}
	chromaW, chromaH := codedW/2, codedH/2
	if interleaved {
		p.uvPixelStride = 2
		p.uvStride = chromaW*2 + padUV
		block := make([]byte, p.uvStride*chromaH+1)
		p.u, p.v = block[0:], block[1:] // two views of one block, as NV12 is given
	} else {
		p.uvPixelStride = 1
		p.uvStride = chromaW + padUV
		p.u = make([]byte, p.uvStride*chromaH)
		p.v = make([]byte, p.uvStride*chromaH)
	}
	for y := 0; y < codedH; y++ {
		for x := 0; x < codedW; x++ {
			sx, sy := min(max(x-cropX, 0), w-1), min(max(y-cropY, 0), h-1)
			i := sy*src.Stride + sx*4
			yy, uu, vv := rgbToYUV(int(src.Pix[i]), int(src.Pix[i+1]), int(src.Pix[i+2]), bt709)
			p.y[y*yStride+x] = byte(yy)
			if x%2 == 0 && y%2 == 0 {
				c := (y/2)*p.uvStride + (x/2)*p.uvPixelStride
				p.u[c], p.v[c] = byte(uu), byte(vv)
			}
		}
	}
	return p
}

func maxChannelError(a, b *image.RGBA) (int, int, int) {
	worst, wx, wy := 0, 0, 0
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			for c := 0; c < 3; c++ {
				d := int(a.Pix[y*a.Stride+x*4+c]) - int(b.Pix[y*b.Stride+x*4+c])
				if d < 0 {
					d = -d
				}
				if d > worst {
					worst, wx, wy = d, x, y
				}
			}
		}
	}
	return worst, wx, wy
}

// TestThePlanesBecomeTheSamePictureInEveryLayout fixes the layouts a decoder
// answers with: separate chroma planes and interleaved ones, rows padded past
// the width, and a crop inside a larger coded picture.
func TestThePlanesBecomeTheSamePictureInEveryLayout(t *testing.T) {
	src := scene(64, 48)
	cases := []struct {
		name                      string
		bt709, interleaved        bool
		cropX, cropY, padY, padUV int
	}{
		{"I420, BT.601", false, false, 0, 0, 0, 0},
		{"I420, BT.709", true, false, 0, 0, 0, 0},
		{"NV12, BT.709", true, true, 0, 0, 0, 0},
		{"padded rows", false, false, 0, 0, 24, 12},
		{"padded and interleaved", true, true, 0, 0, 32, 16},
		{"cropped from a larger picture", true, false, 4, 2, 8, 4},
		{"cropped and interleaved", false, true, 6, 4, 0, 0},
	}
	for _, c := range cases {
		planes := encode(src, c.bt709, c.interleaved, c.cropX, c.cropY, c.padY, c.padUV)
		got := image.NewRGBA(image.Rect(0, 0, 64, 48))
		planes.toRGBA(got)
		// Chroma is sampled at half resolution, so a hard edge between two colours
		// is blended across one pixel; the flat areas are what must be exact.
		worst, x, y := maxChannelError(src, got)
		if worst > 40 {
			t.Errorf("%s: the picture differs by %d at (%d,%d)", c.name, worst, x, y)
		}
		mid := func(img *image.RGBA, x, y int) [3]uint8 {
			i := y*img.Stride + x*4
			return [3]uint8{img.Pix[i], img.Pix[i+1], img.Pix[i+2]}
		}
		for _, at := range [][2]int{{3, 3}, {11, 3}, {19, 11}, {35, 27}} {
			want, have := mid(src, at[0], at[1]), mid(got, at[0], at[1])
			for ch := 0; ch < 3; ch++ {
				d := int(want[ch]) - int(have[ch])
				if d < -4 || d > 4 {
					t.Errorf("%s: flat area at %v is %v, want %v", c.name, at, have, want)
					break
				}
			}
		}
	}
}

func TestTheStandardsAreNotInterchangeable(t *testing.T) {
	src := scene(32, 32)
	planes := encode(src, true, false, 0, 0, 0, 0)
	right, wrong := image.NewRGBA(image.Rect(0, 0, 32, 32)), image.NewRGBA(image.Rect(0, 0, 32, 32))
	planes.toRGBA(right)
	planes.bt709 = false
	planes.toRGBA(wrong)
	if worst, _, _ := maxChannelError(right, wrong); worst < 8 {
		t.Errorf("BT.601 and BT.709 gave the same picture (%d apart); the choice does nothing", worst)
	}
}

func TestBlackAndWhiteSitAtTheEndsOfTheRange(t *testing.T) {
	for _, tc := range []struct {
		y    byte
		want uint8
	}{{16, 0}, {235, 255}, {0, 0}, {255, 255}} {
		p := yuvPlanes{y: []byte{tc.y, tc.y, tc.y, tc.y}, u: []byte{128}, v: []byte{128}, yStride: 2, uvStride: 1, uvPixelStride: 1, width: 2, height: 2}
		img := image.NewRGBA(image.Rect(0, 0, 2, 2))
		p.toRGBA(img)
		if img.Pix[0] != tc.want || img.Pix[1] != tc.want || img.Pix[2] != tc.want {
			t.Errorf("luma %d gave %v, want grey %d", tc.y, img.Pix[:3], tc.want)
		}
	}
}
