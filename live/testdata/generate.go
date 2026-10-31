//go:build ignore

// generate writes the fixtures the live package decodes in its tests: a few
// pictures of a test pattern, coded as the node sends them -- a decoder
// configuration record and length-prefixed NAL units -- for H.264 and for H.265.
//
// It needs ffmpeg with libx264 and libx265, and it is run by hand when a
// fixture has to change:
//
//	go run live/testdata/generate.go
//
// The output is committed. A test that needed ffmpeg to run would be a test
// that is skipped on the machines that matter.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	width  = 128
	height = 96
	frames = 12
	fps    = 10
)

type sample struct {
	Key  bool   `json:"key"`
	PTS  int64  `json:"pts_us"`
	Data string `json:"data"`
}

type fixture struct {
	Codec       string   `json:"codec"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Description string   `json:"description"`
	Samples     []sample `json:"samples"`
}

func main() {
	dir := "live/testdata"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	for _, c := range []struct {
		name, encoder string
		args          []string
		hevc          bool
	}{
		{"h264", "libx264", []string{"-profile:v", "baseline", "-x264-params", "bframes=0:keyint=6:scenecut=0:threads=1:slices=1"}, false},
		{"hevc", "libx265", []string{"-x265-params", "bframes=0:keyint=6:scenecut=0:frame-threads=1:pools=1:log-level=none"}, true},
	} {
		raw, err := encode(c.encoder, c.args, c.hevc)
		if err != nil {
			fatal(err)
		}
		f, err := build(raw, c.hevc)
		if err != nil {
			fatal(err)
		}
		out, _ := json.MarshalIndent(f, "", " ")
		if err := os.WriteFile(filepath.Join(dir, c.name+".json"), append(out, '\n'), 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("%s: %d samples\n", c.name, len(f.Samples))
	}
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func encode(encoder string, extra []string, hevc bool) ([]byte, error) {
	format := "h264"
	if hevc {
		format = "hevc"
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi",
		"-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%d", width, height, fps),
		"-frames:v", fmt.Sprint(frames), "-pix_fmt", "yuv420p", "-c:v", encoder}
	args = append(args, extra...)
	args = append(args, "-f", format, "-")
	var out bytes.Buffer
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// nalus splits an Annex B stream at its start codes.
func nalus(stream []byte) [][]byte {
	var units [][]byte
	start := -1
	for i := 0; i+3 <= len(stream); i++ {
		if stream[i] == 0 && stream[i+1] == 0 && stream[i+2] == 1 {
			if start >= 0 {
				end := i
				for end > start && stream[end-1] == 0 {
					end--
				}
				units = append(units, stream[start:end])
			}
			start = i + 3
			i += 2
		}
	}
	if start >= 0 {
		units = append(units, stream[start:])
	}
	return units
}

func build(stream []byte, hevc bool) (fixture, error) {
	var vps, sps, pps []byte
	var pictures [][][]byte
	var keys []bool
	for _, n := range nalus(stream) {
		if len(n) == 0 {
			continue
		}
		var kind int
		if hevc {
			kind = int(n[0]>>1) & 0x3F
		} else {
			kind = int(n[0] & 0x1F)
		}
		switch {
		case hevc && kind == 32:
			vps = n
		case hevc && kind == 33, !hevc && kind == 7:
			sps = n
		case hevc && kind == 34, !hevc && kind == 8:
			pps = n
		case hevc && kind < 32, !hevc && (kind == 1 || kind == 5):
			key := !hevc && kind == 5 || hevc && kind >= 16 && kind <= 21
			pictures = append(pictures, [][]byte{n})
			keys = append(keys, key)
		}
	}
	if sps == nil || pps == nil || len(pictures) == 0 || hevc && vps == nil {
		return fixture{}, fmt.Errorf("the stream has no parameter sets or no pictures")
	}

	f := fixture{Width: width, Height: height}
	if hevc {
		f.Codec = "hvc1.1.6.L60.90"
		f.Description = base64.StdEncoding.EncodeToString(hvcC(vps, sps, pps))
	} else {
		f.Codec = fmt.Sprintf("avc1.%02X%02X%02X", sps[1], sps[2], sps[3])
		f.Description = base64.StdEncoding.EncodeToString(avcC(sps, pps))
	}
	for i, units := range pictures {
		var data []byte
		for _, n := range units {
			var prefix [4]byte
			binary.BigEndian.PutUint32(prefix[:], uint32(len(n)))
			data = append(data, prefix[:]...)
			data = append(data, n...)
		}
		f.Samples = append(f.Samples, sample{
			Key:  keys[i],
			PTS:  int64(i) * 1_000_000 / fps,
			Data: base64.StdEncoding.EncodeToString(data),
		})
	}
	return f, nil
}

func avcC(sps, pps []byte) []byte {
	rec := []byte{1, sps[1], sps[2], sps[3], 0xFF, 0xE1}
	rec = binary.BigEndian.AppendUint16(rec, uint16(len(sps)))
	rec = append(rec, sps...)
	rec = append(rec, 1)
	rec = binary.BigEndian.AppendUint16(rec, uint16(len(pps)))
	return append(rec, pps...)
}

func hvcC(vps, sps, pps []byte) []byte {
	// The 22-byte header: version, profile space/tier/idc, compatibility flags,
	// constraint flags, level, spatial segmentation, parallelism, chroma format,
	// bit depths, frame rate, and the length size (4, as 0xF | 3).
	rec := make([]byte, 23)
	rec[0] = 1
	rec[1] = 1
	rec[5] = 0x60
	rec[12] = 0x80
	rec[12+5] = 60
	rec[13+5] = 0xF0
	rec[15+5-1] = 0xFC
	rec[16+4] = 0xFD
	rec[17+4] = 0xF8
	rec[18+4] = 0xF8
	rec[21] = 0x03 | 0x0
	rec[22] = 3
	for _, set := range []struct {
		kind byte
		data []byte
	}{{32, vps}, {33, sps}, {34, pps}} {
		rec = append(rec, 0x80|set.kind)
		rec = binary.BigEndian.AppendUint16(rec, 1)
		rec = binary.BigEndian.AppendUint16(rec, uint16(len(set.data)))
		rec = append(rec, set.data...)
	}
	return rec
}
