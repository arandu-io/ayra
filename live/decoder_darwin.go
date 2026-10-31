//go:build darwin

package live

func newDecoder(FrameSink) (Decoder, error) { return nil, ErrUnsupported }
