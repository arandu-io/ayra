// Package live plays the live video a camera's node serves over a WebSocket.
//
// The node writes coded pictures, not a file and not a playlist: one message
// per picture, length-prefixed NAL units, with the decoder's configuration sent
// ahead of the first of them. This package reads that stream, joins the pieces
// of a picture that did not fit in one message, and hands the pictures to a
// decoder the platform provides. It does not draw; [github.com/arandu-io/ayra/widget.LiveProps]
// draws the frame this leaves in a [Player].
//
// The wire format is the one the node's server documents, and it is repeated in
// the doc comments here only as far as a reader of this package needs:
//
//	text    {"type":"video","codec":"avc1.640028","width":1920,"height":1080,
//	         "timescale":90000,"description":"<base64 avcC or hvcC>"}
//	text    {"type":"signal","state":"waiting"|"starting"|"lost"|"busy"|"full"}
//	text    {"type":"ping"}
//	binary  byte 0 = 1, byte 1 = flags (bit 0 key, bit 1 more follows),
//	        bytes 2..9 wall clock in Unix ms, bytes 10..17 presentation time in
//	        microseconds, then the picture
//	binary  byte 0 = 2, byte 1 = flags (bit 1 more follows), then the next bytes
//	        of the picture
//
// Any other text message belongs to the application and is relayed untouched.
package live

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// The binary message kinds and the layout of their headers.
const (
	// SampleMessage starts a picture.
	SampleMessage byte = 1
	// ContinuationMessage carries the next bytes of a picture that did not fit
	// in one message.
	ContinuationMessage byte = 2

	// SampleHeader is the length of a [SampleMessage] before the picture.
	SampleHeader = 18
	// ContinuationHeader is the length of a [ContinuationMessage] before the
	// picture's bytes.
	ContinuationHeader = 2

	// KeyFlag is set in the flags byte of a keyframe.
	KeyFlag byte = 1
	// MoreFlag is set while more pieces of the same picture follow.
	MoreFlag byte = 2
)

// StatusSessionEnded is the WebSocket close code the node uses when a session
// reached its limit. It is not a failure: the viewer asks for a new pass and
// opens another socket.
const StatusSessionEnded = 4001

// DefaultMaxPicture is the largest picture [Parser] assembles. A keyframe of a
// camera at 4K is a few hundred kilobytes; the limit is what stops a peer that
// never clears the "more" flag from growing the buffer without end.
const DefaultMaxPicture = 8 << 20

// Errors the parser answers with.
var (
	// ErrShort is a binary message too short to hold its own header.
	ErrShort = errors.New("live: the message is shorter than its header")
	// ErrOrphan is a continuation with no picture open, which happens when the
	// first piece was lost or refused.
	ErrOrphan = errors.New("live: a continuation arrived with no picture open")
	// ErrInterrupted is a picture that was left unfinished when a new one
	// began. The unfinished one is dropped and the new one is read.
	ErrInterrupted = errors.New("live: a picture began before the last one was complete")
	// ErrTooLarge is a picture over the parser's limit. It is dropped.
	ErrTooLarge = errors.New("live: the picture is larger than the limit")
	// ErrUnknownKind is a binary message that is neither a sample nor a
	// continuation.
	ErrUnknownKind = errors.New("live: unknown binary message kind")
	// ErrDescription is a decoder configuration that cannot be read.
	ErrDescription = errors.New("live: the decoder configuration cannot be read")
)

// Codec names the family of a track. The node sends H.264 as it came from the
// camera and converts everything else to it, but an H.265 track is read too.
type Codec int

const (
	// CodecUnknown is a codec string this package does not recognise.
	CodecUnknown Codec = iota
	// CodecH264 is AVC, configured by an avcC record.
	CodecH264
	// CodecHEVC is H.265, configured by an hvcC record.
	CodecHEVC
)

// String names the codec for a diagnostic.
func (c Codec) String() string {
	switch c {
	case CodecH264:
		return "h264"
	case CodecHEVC:
		return "hevc"
	}
	return "unknown"
}

// Track is the decoder's configuration, sent before the first picture and again
// whenever the camera's stream starts over.
type Track struct {
	// Codec is the RFC 6381 string, such as avc1.640028 or hvc1.1.6.L120.90.
	Codec string
	// Width and Height are the coded picture size in pixels.
	Width, Height int
	// Timescale is the clock the stream was muxed at. Presentation times on the
	// wire are in microseconds regardless.
	Timescale uint32
	// Description is the avcC or hvcC record.
	Description []byte
}

// Family answers the codec family the Codec string names.
func (t Track) Family() Codec {
	name := strings.ToLower(t.Codec)
	switch {
	case strings.HasPrefix(name, "avc1"), strings.HasPrefix(name, "avc3"):
		return CodecH264
	case strings.HasPrefix(name, "hvc1"), strings.HasPrefix(name, "hev1"):
		return CodecHEVC
	}
	return CodecUnknown
}

// NALLengthSize is the width in bytes of the length prefix in front of every
// NAL unit of a picture: 1, 2, 3 or 4.
func (t Track) NALLengthSize() (int, error) {
	switch t.Family() {
	case CodecH264:
		if len(t.Description) < 7 {
			return 0, fmt.Errorf("%w: avcC has %d bytes", ErrDescription, len(t.Description))
		}
		return int(t.Description[4]&3) + 1, nil
	case CodecHEVC:
		if len(t.Description) < 23 {
			return 0, fmt.Errorf("%w: hvcC has %d bytes", ErrDescription, len(t.Description))
		}
		return int(t.Description[21]&3) + 1, nil
	}
	return 0, fmt.Errorf("%w: %q is not a codec this plays", ErrDescription, t.Codec)
}

// ParameterSets answers the parameter sets the description carries, in the
// order a decoder is configured with: sequence, picture and, for H.265, video
// parameter sets first.
//
// A hardware decoder is configured from these and not from the record that
// holds them, so reading the record is this package's job rather than every
// backend's.
func (t Track) ParameterSets() ([][]byte, error) {
	switch t.Family() {
	case CodecH264:
		return avcSets(t.Description)
	case CodecHEVC:
		return hevcSets(t.Description)
	}
	return nil, fmt.Errorf("%w: %q is not a codec this plays", ErrDescription, t.Codec)
}

// avcSets reads the sequence and picture parameter sets out of an
// AVCDecoderConfigurationRecord.
func avcSets(rec []byte) ([][]byte, error) {
	if len(rec) < 7 || rec[0] != 1 {
		return nil, fmt.Errorf("%w: not an avcC record", ErrDescription)
	}
	var sets [][]byte
	at := 5
	for pass := 0; pass < 2; pass++ {
		if at >= len(rec) {
			return nil, fmt.Errorf("%w: avcC ends early", ErrDescription)
		}
		count := int(rec[at])
		if pass == 0 {
			count &= 0x1F
		}
		at++
		for i := 0; i < count; i++ {
			if at+2 > len(rec) {
				return nil, fmt.Errorf("%w: avcC ends early", ErrDescription)
			}
			n := int(binary.BigEndian.Uint16(rec[at:]))
			at += 2
			if n == 0 || at+n > len(rec) {
				return nil, fmt.Errorf("%w: avcC set of %d bytes does not fit", ErrDescription, n)
			}
			sets = append(sets, rec[at:at+n])
			at += n
		}
	}
	if len(sets) < 2 {
		return nil, fmt.Errorf("%w: avcC holds %d parameter sets", ErrDescription, len(sets))
	}
	return sets, nil
}

// hevcSets reads the video, sequence and picture parameter sets out of an
// HEVCDecoderConfigurationRecord.
func hevcSets(rec []byte) ([][]byte, error) {
	if len(rec) < 23 || rec[0] != 1 {
		return nil, fmt.Errorf("%w: not an hvcC record", ErrDescription)
	}
	arrays := int(rec[22])
	at := 23
	var sets [][]byte
	for i := 0; i < arrays; i++ {
		if at+3 > len(rec) {
			return nil, fmt.Errorf("%w: hvcC ends early", ErrDescription)
		}
		count := int(binary.BigEndian.Uint16(rec[at+1:]))
		at += 3
		for j := 0; j < count; j++ {
			if at+2 > len(rec) {
				return nil, fmt.Errorf("%w: hvcC ends early", ErrDescription)
			}
			n := int(binary.BigEndian.Uint16(rec[at:]))
			at += 2
			if n == 0 || at+n > len(rec) {
				return nil, fmt.Errorf("%w: hvcC set of %d bytes does not fit", ErrDescription, n)
			}
			sets = append(sets, rec[at:at+n])
			at += n
		}
	}
	if len(sets) < 3 {
		return nil, fmt.Errorf("%w: hvcC holds %d parameter sets", ErrDescription, len(sets))
	}
	return sets, nil
}

// Sample is one coded picture.
type Sample struct {
	// Key is set on a picture a decoder can start from.
	Key bool
	// Wall is when the node had the picture, which is the clock an overlay
	// from the same node is aligned against.
	Wall time.Time
	// PTS is the presentation time from the stream's start.
	PTS time.Duration
	// Data is the picture as length-prefixed NAL units, in the width the
	// track's description says.
	Data []byte
}

// NALUnits splits the picture into its NAL units, without the length prefixes.
// The slices alias Data.
func (s Sample) NALUnits(lengthSize int) ([][]byte, error) {
	if lengthSize < 1 || lengthSize > 4 {
		return nil, fmt.Errorf("live: NAL length size %d", lengthSize)
	}
	var units [][]byte
	for at := 0; at < len(s.Data); {
		if at+lengthSize > len(s.Data) {
			return nil, errors.New("live: a NAL length prefix is cut short")
		}
		n := 0
		for _, b := range s.Data[at : at+lengthSize] {
			n = n<<8 | int(b)
		}
		at += lengthSize
		if n > len(s.Data)-at {
			return nil, fmt.Errorf("live: a NAL unit of %d bytes does not fit in the %d that remain", n, len(s.Data)-at)
		}
		units = append(units, s.Data[at:at+n])
		at += n
	}
	return units, nil
}

// Signal is the state the node reports about the camera.
type Signal string

// The signals a node sends.
const (
	// SignalWaiting is a camera with no picture yet.
	SignalWaiting Signal = "waiting"
	// SignalStarting is a picture that is being converted for the viewer.
	SignalStarting Signal = "starting"
	// SignalLost is a camera that had a picture and has lost it. The socket
	// stays open and the video returns on its own.
	SignalLost Signal = "lost"
	// SignalBusy is a node that cannot convert the camera now. The node closes
	// the socket after sending it.
	SignalBusy Signal = "busy"
	// SignalFull is a camera that has all the viewers it allows. The node
	// closes the socket after sending it.
	SignalFull Signal = "full"
)

// Kind says which part of an [Event] is filled.
type Kind int

// The kinds of event the parser answers with.
const (
	// KindTrack carries a [Track].
	KindTrack Kind = iota + 1
	// KindSignal carries a [Signal].
	KindSignal
	// KindPing is the keepalive.
	KindPing
	// KindApplication carries a text message the application attached to the
	// feed, in Raw.
	KindApplication
	// KindSample carries a complete [Sample].
	KindSample
)

// Event is one thing the socket said.
type Event struct {
	Kind   Kind
	Track  Track
	Signal Signal
	Sample Sample
	// Raw is the message as it arrived, for [KindApplication].
	Raw json.RawMessage
}

// Parser reads the socket's messages one at a time and joins the pieces of a
// picture.
//
// It holds the one picture being assembled and nothing else, so a parser is
// cheap and one belongs to one socket: a reconnect starts with a new one, which
// is what keeps the pieces of a dead socket from being joined to the pieces of
// the next.
type Parser struct {
	// Max is the largest picture assembled. Zero takes [DefaultMaxPicture].
	Max int

	open   bool
	sample Sample
	data   []byte
}

// Text reads a text message.
func (p *Parser) Text(message []byte) (Event, error) {
	var head struct {
		Type        string `json:"type"`
		Codec       string `json:"codec"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Timescale   uint32 `json:"timescale"`
		Description string `json:"description"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(message, &head); err != nil {
		return Event{}, fmt.Errorf("live: a text message is not JSON: %w", err)
	}
	switch head.Type {
	case "video":
		description, err := base64.StdEncoding.DecodeString(head.Description)
		if err != nil {
			return Event{}, fmt.Errorf("%w: %v", ErrDescription, err)
		}
		track := Track{
			Codec: head.Codec, Width: head.Width, Height: head.Height,
			Timescale: head.Timescale, Description: description,
		}
		if head.Width <= 0 || head.Height <= 0 {
			return Event{}, fmt.Errorf("%w: a track of %dx%d", ErrDescription, head.Width, head.Height)
		}
		if _, err := track.NALLengthSize(); err != nil {
			return Event{}, err
		}
		// A new track means the stream started over: whatever picture was being
		// assembled belongs to the old one.
		p.reset()
		return Event{Kind: KindTrack, Track: track}, nil
	case "signal":
		return Event{Kind: KindSignal, Signal: Signal(head.State)}, nil
	case "ping":
		return Event{Kind: KindPing}, nil
	}
	return Event{Kind: KindApplication, Raw: json.RawMessage(append([]byte(nil), message...))}, nil
}

// Binary reads a binary message. It answers a complete picture, or false while
// pieces of one are still to come.
//
// The error is per message and the parser keeps working after it: a picture
// lost to one is a picture the decoder waits for the next keyframe past.
func (p *Parser) Binary(message []byte) (Event, bool, error) {
	if len(message) == 0 {
		return Event{}, false, ErrShort
	}
	max := p.Max
	if max <= 0 {
		max = DefaultMaxPicture
	}
	switch message[0] {
	case SampleMessage:
		if len(message) < SampleHeader {
			return Event{}, false, ErrShort
		}
		var interrupted error
		if p.open {
			interrupted = ErrInterrupted
			p.reset()
		}
		flags := message[1]
		p.sample = Sample{
			Key:  flags&KeyFlag != 0,
			Wall: time.UnixMilli(int64(binary.BigEndian.Uint64(message[2:]))),
			PTS:  time.Duration(int64(binary.BigEndian.Uint64(message[10:]))) * time.Microsecond,
		}
		body := message[SampleHeader:]
		if len(body) > max {
			return Event{}, false, ErrTooLarge
		}
		p.data = append(p.data[:0], body...)
		if flags&MoreFlag != 0 {
			p.open = true
			return Event{}, false, interrupted
		}
		return p.complete(), true, interrupted
	case ContinuationMessage:
		if len(message) < ContinuationHeader {
			return Event{}, false, ErrShort
		}
		if !p.open {
			return Event{}, false, ErrOrphan
		}
		body := message[ContinuationHeader:]
		if len(p.data)+len(body) > max {
			p.reset()
			return Event{}, false, ErrTooLarge
		}
		p.data = append(p.data, body...)
		if message[1]&MoreFlag != 0 {
			return Event{}, false, nil
		}
		p.open = false
		return p.complete(), true, nil
	}
	return Event{}, false, ErrUnknownKind
}

// complete hands the assembled picture over in a buffer of its own, so that the
// parser can start the next one while the decoder still holds this.
func (p *Parser) complete() Event {
	p.open = false
	sample := p.sample
	sample.Data = append([]byte(nil), p.data...)
	return Event{Kind: KindSample, Sample: sample}
}

func (p *Parser) reset() {
	p.open = false
	p.data = p.data[:0]
}
