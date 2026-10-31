//go:build android

package live

import (
	"errors"
	"fmt"
	"runtime/cgo"
	"sync"
	"time"

	"github.com/arandu-io/ayra/internal/jni"
)

// The Android decoder is MediaCodec, which decodes in the phone's hardware. The
// Java half is io.arandu.ayra.live.AyraDecoder, which the packager compiles from
// this package's android directory when the package is in the graph.
//
// A picture is written by Go straight into the codec's own input buffer, with
// the length prefixes of the node's format replaced by the start codes the codec
// reads, so there is one copy between the socket and the hardware. A decoded
// picture is read by Go from the codec's image planes in place and converted to
// RGBA into the buffer the window draws from, so there is one between the
// hardware and the screen. At 1080p the conversion is the cost that matters,
// and it is paid on the CPU: handing the codec a surface and drawing it as an
// external texture would remove it, and needs the engine's GPU backend to import
// one.
const decoderClass = "io.arandu.ayra.live.AyraDecoder"

// startCode is what separates NAL units in the stream a codec reads.
var startCode = []byte{0, 0, 0, 1}

type mcDecoder struct {
	sink   FrameSink
	handle cgo.Handle

	mu       sync.Mutex
	obj      jni.Ref
	class    jni.Class
	lengthSz int
	closed   bool
}

func newDecoder(sink FrameSink) (Decoder, error) {
	d := &mcDecoder{sink: sink}
	d.handle = cgo.NewHandle(d)
	err := jni.Do(func(env jni.Env) error {
		class, err := env.Class(decoderClass)
		if err != nil {
			return err
		}
		local, err := env.CallStaticObject(class, "create", "(J)Lio/arandu/ayra/live/AyraDecoder;", jni.Long(int64(d.handle)))
		if err != nil {
			return err
		}
		d.class, d.obj = class, env.Global(local)
		env.DeleteLocal(local)
		return nil
	})
	if err != nil {
		d.handle.Delete()
		return nil, err
	}
	return d, nil
}

// mimeFor answers the MediaCodec name of a track's codec.
func mimeFor(t Track) (string, error) {
	switch t.Family() {
	case CodecH264:
		return "video/avc", nil
	case CodecHEVC:
		return "video/hevc", nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupported, t.Codec)
}

// Configure implements [Decoder].
func (d *mcDecoder) Configure(t Track) error {
	mime, err := mimeFor(t)
	if err != nil {
		return err
	}
	sets, err := t.ParameterSets()
	if err != nil {
		return err
	}
	length, err := t.NALLengthSize()
	if err != nil {
		return err
	}

	// H.264 takes the sequence and the picture set as two buffers; H.265 takes
	// its three as one.
	var csd [][]byte
	if t.Family() == CodecH264 {
		for _, set := range sets {
			csd = append(csd, append(append([]byte(nil), startCode...), set...))
		}
	} else {
		var all []byte
		for _, set := range sets {
			all = append(append(all, startCode...), set...)
		}
		csd = [][]byte{all}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return errors.New("live: the decoder is closed")
	}
	d.lengthSz = length
	return jni.Do(func(env jni.Env) error {
		array, err := env.ByteArrays(csd)
		if err != nil {
			return err
		}
		defer env.DeleteLocal(array)
		jmime := env.String(mime)
		defer env.DeleteLocal(jmime)
		reason, err := env.CallObject(d.obj, d.class, "configure", "(Ljava/lang/String;II[[B)Ljava/lang/String;",
			jni.Obj(jmime), jni.Int(t.Width), jni.Int(t.Height), jni.Obj(array))
		if err != nil {
			return err
		}
		defer env.DeleteLocal(reason)
		if msg := env.GoString(reason); msg != "" {
			return fmt.Errorf("live: MediaCodec could not start for %s: %s", t.Codec, msg)
		}
		return nil
	})
}

// Decode implements [Decoder].
func (d *mcDecoder) Decode(s Sample) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return errors.New("live: the decoder is closed")
	}
	units, err := s.NALUnits(d.lengthSz)
	if err != nil {
		return err
	}
	return jni.Do(func(env jni.Env) error {
		buffer, err := env.CallObject(d.obj, d.class, "acquire", "()Ljava/nio/ByteBuffer;")
		if err != nil {
			return err
		}
		if buffer == 0 {
			failed, err := env.CallBool(d.obj, d.class, "failed", "()Z")
			if err != nil {
				return err
			}
			if failed {
				return d.failure(env)
			}
			return ErrBusy
		}
		defer env.DeleteLocal(buffer)

		memory := env.Buffer(buffer)
		n, ok := writeAnnexB(memory, units)
		if !ok {
			return fmt.Errorf("live: a picture of %d bytes does not fit the codec's %d-byte input buffer", len(s.Data), len(memory))
		}
		queued, err := env.CallBool(d.obj, d.class, "queue", "(IJZ)Z", jni.Int(n), jni.Long(int64(s.PTS/time.Microsecond)), jni.Bool(s.Key))
		if err != nil {
			return err
		}
		if !queued {
			return d.failure(env)
		}
		return nil
	})
}

// failure reads why the codec gave up.
func (d *mcDecoder) failure(env jni.Env) error {
	reason, err := env.CallObject(d.obj, d.class, "reason", "()Ljava/lang/String;")
	if err != nil {
		return err
	}
	defer env.DeleteLocal(reason)
	return fmt.Errorf("live: MediaCodec failed: %s", env.GoString(reason))
}

// writeAnnexB writes the units into memory, each behind a start code, and
// answers how many bytes it wrote.
func writeAnnexB(memory []byte, units [][]byte) (int, bool) {
	n := 0
	for _, u := range units {
		if n+len(startCode)+len(u) > len(memory) {
			return 0, false
		}
		n += copy(memory[n:], startCode)
		n += copy(memory[n:], u)
	}
	return n, true
}

// Close implements [Decoder].
func (d *mcDecoder) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	obj, class := d.obj, d.class
	d.mu.Unlock()

	err := jni.Do(func(env jni.Env) error {
		defer env.Release(obj)
		return env.CallVoid(obj, class, "close", "()V")
	})
	d.handle.Delete()
	return err
}

// frame converts one decoded picture into the sink's buffer. It is called from
// the codec's drain thread, with the planes valid until it returns.
func (d *mcDecoder) frame(p yuvPlanes, pts time.Duration) {
	img := d.sink.Acquire(p.width, p.height)
	if img == nil {
		return
	}
	// A picture of 720 lines or more is BT.709 and a smaller one BT.601, which
	// is what the encoders that made them assumed.
	p.bt709 = p.height >= 720
	p.toRGBA(img)
	d.sink.Present(img, pts)
}
