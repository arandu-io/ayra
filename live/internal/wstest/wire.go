package wstest

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"time"
)

// TrackMessage is the text message that configures a decoder, as the node
// writes it.
func TrackMessage(codec string, width, height int, description []byte) string {
	b, _ := json.Marshal(struct {
		Type        string `json:"type"`
		Codec       string `json:"codec"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Timescale   uint32 `json:"timescale"`
		Description string `json:"description"`
	}{"video", codec, width, height, 90000, base64.StdEncoding.EncodeToString(description)})
	return string(b)
}

// SignalMessage is the text message that reports the camera's state.
func SignalMessage(state string) string {
	return `{"type":"signal","state":"` + state + `"}`
}

// SampleMessages lays a picture out as the node does, in messages of at most
// max bytes: the first carries the header and the rest are continuations.
func SampleMessages(key bool, wall time.Time, pts time.Duration, data []byte, max int) [][]byte {
	const header, continuation = 18, 2
	flags := byte(0)
	if key {
		flags = 1
	}
	first := min(len(data), max-header)
	head := make([]byte, header+first)
	head[0] = 1
	head[1] = flags
	if first < len(data) {
		head[1] |= 2
	}
	binary.BigEndian.PutUint64(head[2:], uint64(wall.UnixMilli()))
	binary.BigEndian.PutUint64(head[10:], uint64(pts.Microseconds()))
	copy(head[header:], data[:first])
	pieces := [][]byte{head}
	for rest := data[first:]; len(rest) > 0; {
		n := min(len(rest), max-continuation)
		piece := make([]byte, continuation+n)
		piece[0] = 2
		if n < len(rest) {
			piece[1] = 2
		}
		copy(piece[continuation:], rest[:n])
		pieces = append(pieces, piece)
		rest = rest[n:]
	}
	return pieces
}
