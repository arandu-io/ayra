package headless

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"runtime"
	"testing"

	"github.com/arandu-io/ayra/engine/gpu/internal/driver"
	"github.com/arandu-io/ayra/engine/internal/byteslice"
	"github.com/arandu-io/ayra/engine/internal/f32color"
	"github.com/arandu-io/ayra/engine/shader"
	"github.com/arandu-io/ayra/engine/shader/compiled"
)

var dumpImages = flag.Bool("saveimages", false, "save test images")

var (
	clearCol       = color.NRGBA{A: 0xff, R: 0xde, G: 0xad, B: 0xbe}
	clearColExpect = color.RGBA(clearCol)
)

func TestFramebufferClear(t *testing.T) {
	b := newDriver(t)
	sz := image.Point{X: 800, Y: 600}
	fbo := newFBO(t, b, sz)
	d := driver.LoadDesc{
		// The frame is written and blended in the space a picture is written
		// in, so the clear colour is written as it is.
		ClearColor: f32color.PremulFromSRGB(clearCol),
		Action:     driver.LoadActionClear,
	}
	b.BeginRenderPass(fbo, d)
	b.EndRenderPass()
	img := screenshot(t, b, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
}

func TestInputShader(t *testing.T) {
	b := newDriver(t)
	sz := image.Point{X: 800, Y: 600}
	vsh, fsh, err := newShaders(b, compiled.Shader_input_vert, compiled.Shader_simple_frag)
	if err != nil {
		t.Fatal(err)
	}
	defer vsh.Release()
	defer fsh.Release()
	layout := driver.VertexLayout{
		Inputs: []driver.InputDesc{
			{
				Type:   shader.DataTypeFloat,
				Size:   4,
				Offset: 0,
			},
		},
		Stride: 4 * 4,
	}
	fbo := newFBO(t, b, sz)
	pipe, err := b.NewPipeline(driver.PipelineDesc{
		VertexShader:   vsh,
		FragmentShader: fsh,
		VertexLayout:   layout,
		PixelFormat:    driver.TextureFormatRGBA8,
		Topology:       driver.TopologyTriangles,
		BlendDesc: driver.BlendDesc{
			SrcFactor: driver.BlendFactorOne,
			DstFactor: driver.BlendFactorZero,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Release()
	buf, err := b.NewImmutableBuffer(driver.BufferBindingVertices,
		byteslice.Slice([]float32{
			0, -.5, .5, 1,
			-.5, +.5, .5, 1,
			.5, +.5, .5, 1,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Release()
	d := driver.LoadDesc{
		ClearColor: f32color.PremulFromSRGB(clearCol),
		Action:     driver.LoadActionClear,
	}
	b.BeginRenderPass(fbo, d)
	b.Viewport(0, 0, sz.X, sz.Y)
	b.BindPipeline(pipe)
	b.BindVertexBuffer(buf, 0)
	b.DrawArrays(0, 3)
	b.EndRenderPass()
	img := screenshot(t, b, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
	cx, cy := 300, 400
	// What the shader writes is what the target holds, byte for byte.
	if got, exp := img.RGBAAt(cx, cy), (color.RGBA{R: 64, G: 140, B: 191, A: 255}); got != exp {
		t.Errorf("got color %v, expected %v", got, exp)
	}
}

func newShaders(ctx driver.Device, vsrc, fsrc shader.Sources) (vert driver.VertexShader, frag driver.FragmentShader, err error) {
	vert, err = ctx.NewVertexShader(vsrc)
	if err != nil {
		return
	}
	frag, err = ctx.NewFragmentShader(fsrc)
	if err != nil {
		vert.Release()
	}
	return
}

func TestFramebuffers(t *testing.T) {
	b := newDriver(t)
	sz := image.Point{X: 800, Y: 600}
	var (
		col1 = color.NRGBA{R: 0xac, G: 0xbd, B: 0xef, A: 0xde}
		col2 = color.NRGBA{R: 0xfe, G: 0xbb, B: 0xbe, A: 0xca}
	)
	fbo1 := newFBO(t, b, sz)
	fbo2 := newFBO(t, b, sz)
	fcol1, fcol2 := f32color.PremulFromSRGB(col1), f32color.PremulFromSRGB(col2)
	d := driver.LoadDesc{Action: driver.LoadActionClear,
		ClearColor: fcol1}
	b.BeginRenderPass(fbo1, d)
	b.EndRenderPass()
	d.ClearColor = fcol2
	b.BeginRenderPass(fbo2, d)
	b.EndRenderPass()
	img := screenshot(t, b, fbo1, sz)
	if got, want := img.RGBAAt(0, 0), premultiplied(col1); got != want {
		t.Errorf("got color %v, expected %v", got, want)
	}
	img = screenshot(t, b, fbo2, sz)
	if got, want := img.RGBAAt(0, 0), premultiplied(col2); got != want {
		t.Errorf("got color %v, expected %v", got, want)
	}
}

// premultiplied is the colour as the target holds it: each channel multiplied by
// the alpha, in the space the bytes are in.
func premultiplied(c color.NRGBA) color.RGBA {
	p := f32color.PremulFromSRGB(c)
	return color.RGBA{R: uint8(p.R*255 + 0.5), G: uint8(p.G*255 + 0.5), B: uint8(p.B*255 + 0.5), A: c.A}
}

func newFBO(t *testing.T, b driver.Device, size image.Point) driver.Texture {
	fboTex, err := b.NewTexture(
		driver.TextureFormatRGBA8,
		size.X, size.Y,
		driver.FilterNearest, driver.FilterNearest,
		driver.BufferBindingFramebuffer,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fboTex.Release()
	})
	return fboTex
}

func newDriver(t *testing.T) driver.Device {
	ctx, err := newContext()
	if err != nil {
		t.Skipf("no context available: %v", err)
	}
	if err := ctx.MakeCurrent(); err != nil {
		t.Fatal(err)
	}
	b, err := driver.NewDevice(ctx.API())
	if err != nil {
		t.Fatal(err)
	}
	b.BeginFrame(nil, true, image.Pt(1, 1))
	t.Cleanup(func() {
		b.EndFrame()
		b.Release()
		ctx.ReleaseCurrent()
		runtime.UnlockOSThread()
		ctx.Release()
	})
	return b
}

func screenshot(t *testing.T, d driver.Device, fbo driver.Texture, size image.Point) *image.RGBA {
	img := image.NewRGBA(image.Rectangle{Max: size})
	err := driver.DownloadImage(d, fbo, img)
	if err != nil {
		t.Fatal(err)
	}
	if *dumpImages {
		if err := saveImage(t.Name()+".png", img); err != nil {
			t.Error(err)
		}
	}
	return img
}

func saveImage(file string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0o666)
}
