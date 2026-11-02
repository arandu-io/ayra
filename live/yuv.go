package live

import "image"

// yuvPlanes is a decoded picture in the layout a hardware decoder leaves it:
// a luma plane and two chroma planes at half the size each way, the chroma
// either in planes of their own (pixel stride 1) or interleaved (pixel stride
// 2, which is what NV12 and NV21 look like when the two planes are given as
// separate views of one block).
type yuvPlanes struct {
	y, u, v       []byte
	yStride       int
	uvStride      int
	uvPixelStride int
	cropX, cropY  int
	width, height int
	bt709         bool
}

// toRGBA converts the picture into dst, which is width by height. It reads each
// chroma sample once for the four pixels it covers.
//
// The coefficients are the fixed-point ones of the two standards and the usual
// limited range: a picture of 720 lines or more is read as BT.709 and a smaller
// one as BT.601, which is what the encoders that made them assumed.
func (p yuvPlanes) toRGBA(dst *image.RGBA) {
	var rv, gu, gv, bu int32
	if p.bt709 {
		rv, gu, gv, bu = 459, 55, 136, 541
	} else {
		rv, gu, gv, bu = 409, 100, 208, 516
	}
	for row := 0; row < p.height; row++ {
		yRow := p.y[(p.cropY+row)*p.yStride+p.cropX:]
		cRow := ((p.cropY + row) / 2) * p.uvStride
		out := dst.Pix[row*dst.Stride:]
		for col := 0; col < p.width; col++ {
			cIdx := cRow + ((p.cropX+col)/2)*p.uvPixelStride
			yy := (int32(yRow[col]) - 16) * 298
			d := int32(p.u[cIdx]) - 128
			e := int32(p.v[cIdx]) - 128
			o := out[col*4 : col*4+4 : col*4+4]
			o[0] = clamp8((yy + rv*e + 128) >> 8)
			o[1] = clamp8((yy - gu*d - gv*e + 128) >> 8)
			o[2] = clamp8((yy + bu*d + 128) >> 8)
			o[3] = 255
		}
	}
}

func clamp8(v int32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
