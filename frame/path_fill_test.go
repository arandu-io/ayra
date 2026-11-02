package frame_test

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/frame"
	"github.com/arandu-io/ayra/theme"
)

// A filled path leaves nothing outside its outline.
//
// The mark at the top of the Alpr.sh app is one path of two contours, an eye
// whose iris is a hole, with thin overlapping spurs at its corners, and it drew
// faint vertical streaks under the eye that were plain on the dark scheme: the
// path accumulates its coverage in a half-precision target, a spur that crosses
// the winding of the rest leaves a few thousandths where everything cancels, and
// the engine bent those thousandths into 7 to 12 of 255 on black. Blended in the
// space a picture is written in the same residue stays at one or two steps, which
// is what is held here: against a coverage computed on the CPU, no pixel that the
// shape does not touch is lit by more than two of 255. (A single-precision target
// would remove the last of it, and is not portable to the phones that have to draw
// this.) The picture of a ring with a
// bar is the shape most likely to go wrong in the same way, and is kept in the
// three densities a phone has.

// shape is a closed path as the segments it was written with.
type shape []seg

type seg struct {
	op byte
	v  [6]float32
}

func parsePath(s string) shape {
	var out shape
	for i := 0; i < len(s); {
		ch := s[i]
		if ch == ' ' || ch == ',' {
			i++
			continue
		}
		n := map[byte]int{'M': 2, 'L': 2, 'C': 6, 'Z': 0}[ch]
		i++
		sg := seg{op: ch}
		for k := 0; k < n; k++ {
			for i < len(s) && (s[i] == ' ' || s[i] == ',') {
				i++
			}
			j := i
			for j < len(s) && (s[j] == '-' || s[j] == '.' || (s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			v, _ := strconv.ParseFloat(s[i:j], 32)
			sg.v[k] = float32(v)
			i = j
		}
		out = append(out, sg)
	}
	return out
}

// ringWithBar is a ring, a vertical bar through it and a spur that doubles back
// over the ring's edge, in a 100 by 100 box: the contours of the Alpr.sh mark in
// the smallest form that has them.
func ringWithBar() shape {
	circle := func(cx, cy, r float32, clockwise bool) shape {
		const k = 0.5522847498
		pts := [][2]float32{{cx + r, cy}, {cx, cy + r}, {cx - r, cy}, {cx, cy - r}}
		if !clockwise {
			pts = [][2]float32{{cx + r, cy}, {cx, cy - r}, {cx - r, cy}, {cx, cy + r}}
		}
		s := shape{{'M', [6]float32{pts[0][0], pts[0][1]}}}
		for i := 0; i < 4; i++ {
			a, b := pts[i], pts[(i+1)%4]
			// The control points lie a quarter of the way round each side of the
			// arc: along the tangent at each end.
			ta := [2]float32{-(a[1] - cy), a[0] - cx}
			tb := [2]float32{-(b[1] - cy), b[0] - cx}
			if !clockwise {
				ta[0], ta[1], tb[0], tb[1] = -ta[0], -ta[1], -tb[0], -tb[1]
			}
			s = append(s, seg{'C', [6]float32{
				a[0] + ta[0]*k, a[1] + ta[1]*k,
				b[0] - tb[0]*k, b[1] - tb[1]*k,
				b[0], b[1],
			}})
		}
		return append(s, seg{op: 'Z'})
	}
	s := circle(50, 50, 44, true)
	s = append(s, circle(50, 50, 26, false)...)
	// The bar, the same way round as the ring, so that where it crosses the hole it
	// fills it, and a spur from its foot back over the ring.
	s = append(s,
		seg{'M', [6]float32{45, 4}}, seg{'L', [6]float32{55, 4}}, seg{'L', [6]float32{55, 96}},
		seg{'L', [6]float32{45, 96}}, seg{op: 'Z'},
		seg{'M', [6]float32{18, 80}}, seg{'L', [6]float32{20.5, 79.4}}, seg{'L', [6]float32{19.2, 94}}, seg{op: 'Z'},
	)
	return s
}

// build writes the shape into the ops, scaled by k.
func (s shape) build(c ayra.Context, k float32) clip.PathSpec {
	var p clip.Path
	p.Begin(c.Ops)
	for _, sg := range s {
		switch sg.op {
		case 'M':
			p.MoveTo(f32.Pt(sg.v[0]*k, sg.v[1]*k))
		case 'L':
			p.LineTo(f32.Pt(sg.v[0]*k, sg.v[1]*k))
		case 'C':
			p.CubeTo(f32.Pt(sg.v[0]*k, sg.v[1]*k), f32.Pt(sg.v[2]*k, sg.v[3]*k), f32.Pt(sg.v[4]*k, sg.v[5]*k))
		case 'Z':
			p.Close()
		}
	}
	return p.End()
}

// touched says which pixels of a w by h picture the shape, scaled by k, comes
// within a pixel of, by filling it on the CPU with the non-zero rule at 8 by 8
// samples to the pixel.
func (s shape) touched(w, h int, k float32) []bool {
	const n = 8
	type edge struct{ x0, y0, x1, y1 float64 }
	var edges []edge
	var cur, start [2]float64
	line := func(x, y float64) {
		edges = append(edges, edge{cur[0], cur[1], x, y})
		cur = [2]float64{x, y}
	}
	for _, sg := range s {
		v := [6]float64{}
		for i := range v {
			v[i] = float64(sg.v[i] * k)
		}
		switch sg.op {
		case 'M':
			cur, start = [2]float64{v[0], v[1]}, [2]float64{v[0], v[1]}
		case 'L':
			line(v[0], v[1])
		case 'C':
			p0 := cur
			for i := 1; i <= 48; i++ {
				t := float64(i) / 48
				u := 1 - t
				line(u*u*u*p0[0]+3*u*u*t*v[0]+3*u*t*t*v[2]+t*t*t*v[4], u*u*u*p0[1]+3*u*u*t*v[1]+3*u*t*t*v[3]+t*t*t*v[5])
			}
		case 'Z':
			line(start[0], start[1])
		}
	}
	inside := make([]bool, w*n*h*n)
	type crossing struct {
		x   float64
		dir int
	}
	for sy := 0; sy < h*n; sy++ {
		y := (float64(sy) + 0.5) / n
		var cs []crossing
		for _, e := range edges {
			if (e.y0 <= y) == (e.y1 <= y) {
				continue
			}
			x := e.x0 + (y-e.y0)/(e.y1-e.y0)*(e.x1-e.x0)
			d := 1
			if e.y1 < e.y0 {
				d = -1
			}
			cs = append(cs, crossing{x, d})
		}
		sort.Slice(cs, func(a, b int) bool { return cs[a].x < cs[b].x })
		wind := 0
		for i := 0; i < len(cs)-1; i++ {
			wind += cs[i].dir
			if wind == 0 {
				continue
			}
			from := int(math.Ceil(cs[i].x*n - 0.5))
			to := int(math.Ceil(cs[i+1].x*n - 0.5))
			for sx := max(from, 0); sx < min(to, w*n); sx++ {
				inside[sy*w*n+sx] = true
			}
		}
	}
	covered := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for sy := 0; sy < n && !covered[y*w+x]; sy++ {
				for sx := 0; sx < n; sx++ {
					if inside[(y*n+sy)*w*n+x*n+sx] {
						covered[y*w+x] = true
						break
					}
				}
			}
		}
	}
	// A pixel beside a covered one may carry the edge's antialiasing.
	near := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if xx, yy := x+dx, y+dy; xx >= 0 && yy >= 0 && xx < w && yy < h && covered[yy*w+xx] {
						near[y*w+x] = true
					}
				}
			}
		}
	}
	return near
}

// paintShape draws the shape in white over black, size points tall, and returns
// the picture and the CPU's idea of where the shape is.
func paintShape(t *testing.T, s shape, box float32, size int, density float32) (*image.RGBA, []bool) {
	t.Helper()
	pxW, pxH := int(float32(size)*density*box/100)+int(2*density), int(float32(size)*density)+int(2*density)
	k := float32(size) * density / box
	screen := func(c ayra.Context) ayra.Dimensions {
		area := clip.Rect(image.Rect(0, 0, pxW+8, pxH+8)).Op()
		paint.FillShape(c.Ops, color.NRGBA{A: 255}, area)
		paint.FillShape(c.Ops, color.NRGBA{R: 245, G: 245, B: 245, A: 255}, clip.Outline{Path: s.build(c, k)}.Op())
		return ayra.Dimensions{Size: image.Pt(pxW, pxH)}
	}
	w, h := int(float32(pxW)/density)+1, int(float32(pxH)/density)+1
	img, err := frame.Draw(frame.Options{Width: w, Height: h, Density: density, Scheme: theme.Dark, Fonts: fonts()}, screen)
	if err != nil {
		if noSurface(err) {
			t.Skipf("no drawing surface on this machine: %v", err)
		}
		t.Fatal(err)
	}
	return img, s.touched(img.Bounds().Dx(), img.Bounds().Dy(), k)
}

func TestAPathLeavesNothingLitOutsideItsOutline(t *testing.T) {
	raw, err := os.ReadFile("testdata/visor-path.txt")
	if err != nil {
		t.Fatal(err)
	}
	visor := parsePath(strings.TrimSpace(string(raw)))

	for name, test := range map[string]struct {
		s   shape
		box float32
	}{
		"the Alpr.sh mark":  {visor, 587.0 / 371 * 100},
		"a ring with a bar": {ringWithBar(), 100},
	} {
		for _, density := range []float32{1, 2, 3} {
			// The mark is 371 tall, the ring 100: both are scaled to 24 points.
			box := test.box
			if name == "the Alpr.sh mark" {
				box = 371
			}
			img, near := paintShape(t, test.s, box, 24, density)
			worst, stray := 0, 0
			var at image.Point
			b := img.Bounds()
			for y := 0; y < b.Dy(); y++ {
				for x := 0; x < b.Dx(); x++ {
					if near[y*b.Dx()+x] {
						continue
					}
					if v := int(img.RGBAAt(x, y).R); v > worst {
						worst, at = v, image.Pt(x, y)
					}
					if img.RGBAAt(x, y).R > 2 {
						stray++
					}
				}
			}
			if worst > 2 {
				t.Errorf("%s at %gx: %d pixels the shape does not touch are lit, the worst %d of 255 at %v", name, density, stray, worst, at)
			}
		}
	}
}

func TestARingWithABarLooksLikeItDid(t *testing.T) {
	for _, density := range []float32{1, 2, 3} {
		name := fmt.Sprintf("ring-with-bar-dark-%gx", density)
		t.Run(name, func(t *testing.T) {
			img, _ := paintShape(t, ringWithBar(), 100, 48, density)
			compare(t, name, img)
		})
	}
}
