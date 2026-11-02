package live

import (
	"errors"
	"image"
	"time"
)

// Errors a decoder answers with.
var (
	// ErrUnsupported is a platform with no decoder, or a codec the platform's
	// decoder does not play. The desktop targets answer it for every codec: the
	// hardware decoders are written for the two phone platforms and macOS, and
	// a software one is deliberately not carried.
	ErrUnsupported = errors.New("live: this platform has no decoder for the stream")
	// ErrBusy is a decoder with no room for another picture. The picture is
	// dropped, and the player waits for the next keyframe before it sends
	// another, because a picture decoded without the one before it is not a
	// picture.
	ErrBusy = errors.New("live: the decoder has no room for another picture")
)

// Decoder turns coded pictures into frames.
//
// A backend is written once per platform: VideoToolbox on the Apple platforms
// and MediaCodec on Android. It decodes in hardware and writes into the buffer
// the [FrameSink] hands it, so that nothing is copied between the decoder's
// output and the buffer the window draws from but the one conversion to RGBA
// the GPU upload needs.
type Decoder interface {
	// Configure starts the decoder for a track. It is called again, on the same
	// decoder, when the stream starts over with a new configuration.
	Configure(Track) error
	// Decode takes one picture. It must not wait for the picture to come out:
	// frames are delivered to the sink when they are ready, possibly from
	// another thread, and in presentation order.
	Decode(Sample) error
	// Close releases the hardware. The decoder is not used afterwards.
	Close() error
}

// FrameSink is where a decoder puts what it decoded.
type FrameSink interface {
	// Acquire answers a buffer of the size the decoder is about to write into,
	// or nil when no buffer is free, in which case the decoder drops the
	// picture. A decoder holds at most one buffer at a time: it presents the
	// one it has before it asks for another.
	Acquire(width, height int) *image.RGBA
	// Present hands the buffer back with the picture written into it. pts is the
	// presentation time of the sample it came from.
	Present(img *image.RGBA, pts time.Duration)
}

// NewDecoder opens the platform's decoder, writing into sink.
//
// It is the default for [PlayerConfig.Decoder]. Where the platform has none it
// answers [ErrUnsupported], and a [Player] over it reports a failed stream with
// that error rather than a stream that connects and shows nothing.
func NewDecoder(sink FrameSink) (Decoder, error) { return newDecoder(sink) }

// Supported reports whether this platform has a decoder, so that a screen can
// choose between the moving picture and a still one before it opens a socket.
// Android and the Apple platforms do; the other targets answer false, and a
// [Player] there would only report [ErrUnsupported].
func Supported() bool { return supported }
