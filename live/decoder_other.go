//go:build !darwin && !android

package live

// newDecoder is the desktop answer: there is no decoder.
//
// Windows, Linux and the browser target would each need a decoder of their own
// -- Media Foundation, VA-API or V4L2, and WebCodecs -- and none of them is
// written. A software decoder would work everywhere, and it would put a codec
// in the graph of every application that imports this package; the choice not
// to is deliberate. An application on those targets draws the stream's boxes
// and status over a still picture or none, and [PlayerConfig.Decoder] takes its
// own decoder if one is wanted.
// supported is false: this platform has no decoder.
const supported = false

func newDecoder(FrameSink) (Decoder, error) { return nil, ErrUnsupported }
