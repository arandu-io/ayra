//go:build android

package live

func newDecoder(FrameSink) (Decoder, error) { return nil, ErrUnsupported }
