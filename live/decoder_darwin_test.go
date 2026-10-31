//go:build darwin

package live_test

import (
	"image"
	"image/png"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/arandu-io/ayra/live"
	"github.com/arandu-io/ayra/live/internal/wstest"
)

// These run the real decoder. VideoToolbox is the same framework on a Mac and on
// an iPhone, so what is proved here is the code that runs on the phone, up to
// the hardware underneath it.

// collector is a sink that keeps every frame.
type collector struct {
	mu     sync.Mutex
	frames []*image.RGBA
	pts    []time.Duration
}

func (c *collector) Acquire(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }

func (c *collector) Present(img *image.RGBA, pts time.Duration) {
	c.mu.Lock()
	c.frames = append(c.frames, img)
	c.pts = append(c.pts, pts)
	c.mu.Unlock()
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames)
}

// meanError is the average difference per colour channel between a decoded
// frame and the picture the fixture was encoded from. A swapped red and blue
// is a difference of dozens on this pattern, which is what the bound is for.
func meanError(t testing.TB, got *image.RGBA) float64 {
	t.Helper()
	f, err := os.Open("testdata/frame0.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if want.Bounds().Size() != got.Bounds().Size() {
		t.Fatalf("the decoded frame is %v, the reference %v", got.Bounds().Size(), want.Bounds().Size())
	}
	var sum, n float64
	for y := 0; y < got.Bounds().Dy(); y++ {
		for x := 0; x < got.Bounds().Dx(); x++ {
			wr, wg, wb, _ := want.At(x, y).RGBA()
			g := got.RGBAAt(x, y)
			for _, d := range []float64{float64(wr>>8) - float64(g.R), float64(wg>>8) - float64(g.G), float64(wb>>8) - float64(g.B)} {
				if d < 0 {
					d = -d
				}
				sum += d
				n++
			}
		}
	}
	return sum / n
}

func TestTheHardwareDecoderTurnsTheFixturesIntoPictures(t *testing.T) {
	for _, name := range []string{"h264", "hevc"} {
		t.Run(name, func(t *testing.T) {
			track, samples := load(t, name)
			sink := &collector{}
			decoder, err := live.NewDecoder(sink)
			if err != nil {
				t.Fatal(err)
			}
			defer decoder.Close()
			if err := decoder.Configure(track); err != nil {
				t.Skipf("this machine has no %s decoder: %v", name, err)
			}

			for _, s := range samples {
				for {
					err := decoder.Decode(s)
					if err == nil {
						break
					}
					if err != live.ErrBusy {
						t.Fatalf("decode: %v", err)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
			eventually(t, "every picture", func() bool { return sink.count() == len(samples) })

			sink.mu.Lock()
			defer sink.mu.Unlock()
			first := sink.frames[0]
			if first.Bounds().Dx() != track.Width || first.Bounds().Dy() != track.Height {
				t.Fatalf("a %dx%d picture came out as %v", track.Width, track.Height, first.Bounds())
			}
			if mae := meanError(t, first); mae > 12 {
				t.Errorf("the first decoded picture differs from the source by %.1f per channel; a swapped red and blue would be far more, so the picture is wrong", mae)
			}
			for i := 1; i < len(sink.pts); i++ {
				if sink.pts[i] <= sink.pts[i-1] {
					t.Errorf("pictures out of order: %v", sink.pts)
					break
				}
			}
			if first.RGBAAt(0, 0).A != 255 {
				t.Error("a decoded picture is not opaque")
			}
		})
	}
}

// TestADecoderReconfiguredForANewTrackKeepsDecoding is the camera whose stream
// starts over: the same decoder, a new configuration.
func TestADecoderReconfiguredForANewTrackKeepsDecoding(t *testing.T) {
	h264, s264 := load(t, "h264")
	hevc, s265 := load(t, "hevc")
	sink := &collector{}
	decoder, _ := live.NewDecoder(sink)
	defer decoder.Close()

	if err := decoder.Configure(h264); err != nil {
		t.Skip(err)
	}
	decoder.Decode(s264[0])
	eventually(t, "the first picture", func() bool { return sink.count() == 1 })

	if err := decoder.Configure(hevc); err != nil {
		t.Skip(err)
	}
	decoder.Decode(s265[0])
	eventually(t, "a picture after the change", func() bool { return sink.count() == 2 })
}

func TestACodecNobodyPlaysIsRefused(t *testing.T) {
	decoder, _ := live.NewDecoder(&collector{})
	defer decoder.Close()
	err := decoder.Configure(live.Track{Codec: "vp09.00.10.08", Width: 16, Height: 16, Description: []byte{1}})
	if err == nil {
		t.Fatal("a codec the platform does not play was accepted")
	}
}

func TestADecoderThatWasNotConfiguredRefusesPictures(t *testing.T) {
	_, samples := load(t, "h264")
	decoder, _ := live.NewDecoder(&collector{})
	defer decoder.Close()
	if err := decoder.Decode(samples[0]); err == nil {
		t.Error("a picture was accepted before any configuration")
	}
}

// TestAPlayerOverTheRealDecoderShowsTheCamera is the whole path on the machine:
// a socket speaking the node's format, the stream, the hardware decoder and the
// frame the window would draw.
func TestAPlayerOverTheRealDecoderShowsTheCamera(t *testing.T) {
	track, samples := load(t, "h264")
	server := wstest.New(t, func(s *wstest.Session) {
		sendClip(t, s, track, samples, 700) // keyframes in pieces
		time.Sleep(time.Second)
	})
	var p passes
	player := live.NewPlayer(live.PlayerConfig{Endpoint: server.URL("/live/cam"), Pass: p.ask, AutoPause: -1})
	t.Cleanup(player.Stop)
	player.Start()

	var got live.Frame
	eventually(t, "the last picture", func() bool {
		got = player.Frame()
		return got.Image != nil && got.PTS == samples[len(samples)-1].PTS
	})
	if got.Size().X != track.Width || got.Size().Y != track.Height {
		t.Errorf("the frame is %v", got.Size())
	}
	if got.Wall.IsZero() {
		t.Error("the frame lost the node's wall clock")
	}
	if st := player.Stats(); st.Dropped != 0 {
		t.Errorf("%d pictures were dropped on a clean stream: %+v", st.Dropped, st)
	}
}
