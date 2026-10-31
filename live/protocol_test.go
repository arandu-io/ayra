package live_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/arandu-io/ayra/live"
	"github.com/arandu-io/ayra/live/internal/wstest"
)

// clip is a fixture: a few pictures coded as the node sends them.
type clip struct {
	Codec       string `json:"codec"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Description string `json:"description"`
	Samples     []struct {
		Key  bool   `json:"key"`
		PTS  int64  `json:"pts_us"`
		Data string `json:"data"`
	} `json:"samples"`
}

// load reads a fixture and answers its track and its pictures.
func load(t testing.TB, name string) (live.Track, []live.Sample) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var c clip
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	description, err := base64.StdEncoding.DecodeString(c.Description)
	if err != nil {
		t.Fatal(err)
	}
	track := live.Track{Codec: c.Codec, Width: c.Width, Height: c.Height, Timescale: 90000, Description: description}
	var samples []live.Sample
	for _, s := range c.Samples {
		data, err := base64.StdEncoding.DecodeString(s.Data)
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, live.Sample{
			Key: s.Key, PTS: time.Duration(s.PTS) * time.Microsecond,
			Wall: time.UnixMilli(1_700_000_000_000 + s.PTS/1000), Data: data,
		})
	}
	return track, samples
}

func TestATrackIsReadFromTheTextMessage(t *testing.T) {
	for _, name := range []string{"h264", "hevc"} {
		track, _ := load(t, name)
		var p live.Parser
		event, err := p.Text([]byte(wstest.TrackMessage(track.Codec, track.Width, track.Height, track.Description)))
		if err != nil || event.Kind != live.KindTrack {
			t.Fatalf("%s: %v %v", name, event.Kind, err)
		}
		got := event.Track
		if got.Codec != track.Codec || got.Width != track.Width || got.Height != track.Height || !bytes.Equal(got.Description, track.Description) {
			t.Errorf("%s: the track read back as %+v", name, got)
		}
	}
}

func TestTheCodecFamilyAndTheParameterSetsComeFromTheDescription(t *testing.T) {
	h264, _ := load(t, "h264")
	if h264.Family() != live.CodecH264 {
		t.Errorf("h264 read as %v", h264.Family())
	}
	size, err := h264.NALLengthSize()
	if err != nil || size != 4 {
		t.Errorf("the h264 length size is %d (%v), want 4", size, err)
	}
	sets, err := h264.ParameterSets()
	if err != nil || len(sets) != 2 || sets[0][0]&0x1F != 7 || sets[1][0]&0x1F != 8 {
		t.Errorf("h264 parameter sets: %d, %v", len(sets), err)
	}

	hevc, _ := load(t, "hevc")
	if hevc.Family() != live.CodecHEVC {
		t.Errorf("hevc read as %v", hevc.Family())
	}
	sets, err = hevc.ParameterSets()
	if err != nil || len(sets) != 3 {
		t.Fatalf("hevc parameter sets: %d, %v", len(sets), err)
	}
	for i, want := range []byte{32, 33, 34} {
		if got := sets[i][0] >> 1 & 0x3F; got != want {
			t.Errorf("hevc set %d is NAL type %d, want %d", i, got, want)
		}
	}
}

func TestADescriptionThatCannotBeReadIsRefused(t *testing.T) {
	for _, track := range []live.Track{
		{Codec: "avc1.640028", Description: []byte{1, 2, 3}},
		{Codec: "avc1.640028", Description: []byte{9, 0x64, 0, 0x28, 0xFF, 0xE1, 0, 5}},
		{Codec: "avc1.640028", Description: []byte{1, 0x64, 0, 0x28, 0xFF, 0xE1, 0, 200, 1}},
		{Codec: "hvc1.1.6.L120", Description: make([]byte, 10)},
		{Codec: "vp09.00.10.08", Description: []byte{1, 2, 3, 4, 5, 6, 7, 8}},
	} {
		if _, err := track.ParameterSets(); !errors.Is(err, live.ErrDescription) {
			t.Errorf("%q with %d bytes answered %v, want ErrDescription", track.Codec, len(track.Description), err)
		}
	}
}

func TestTheSignalsAndTheKeepaliveAreTold(t *testing.T) {
	var p live.Parser
	for _, state := range []live.Signal{live.SignalWaiting, live.SignalStarting, live.SignalLost, live.SignalBusy, live.SignalFull} {
		event, err := p.Text([]byte(wstest.SignalMessage(string(state))))
		if err != nil || event.Kind != live.KindSignal || event.Signal != state {
			t.Errorf("%s read as %+v (%v)", state, event, err)
		}
	}
	if event, err := p.Text([]byte(`{"type":"ping"}`)); err != nil || event.Kind != live.KindPing {
		t.Errorf("the ping read as %+v (%v)", event, err)
	}
}

func TestAnApplicationMessageIsRelayedUntouched(t *testing.T) {
	var p live.Parser
	message := `{"type":"boxes","at":1700000000000,"plates":[{"text":"ABC1D23","x":1,"y":2}]}`
	event, err := p.Text([]byte(message))
	if err != nil || event.Kind != live.KindApplication || string(event.Raw) != message {
		t.Errorf("the application message read as %+v (%v)", event, err)
	}
}

func TestATextMessageThatIsNotJSONIsAnError(t *testing.T) {
	var p live.Parser
	if _, err := p.Text([]byte("not json")); err == nil {
		t.Error("text that is not JSON was accepted")
	}
	if _, err := p.Text([]byte(`{"type":"video","codec":"avc1.640028","width":0,"height":0,"description":""}`)); err == nil {
		t.Error("a track of no size was accepted")
	}
	if _, err := p.Text([]byte(`{"type":"video","codec":"avc1.640028","width":2,"height":2,"description":"!!"}`)); !errors.Is(err, live.ErrDescription) {
		t.Errorf("a description that is not base64 answered %v", err)
	}
}

func TestAPictureInOneMessageIsComplete(t *testing.T) {
	_, samples := load(t, "h264")
	want := samples[0]
	var p live.Parser
	messages := wstest.SampleMessages(want.Key, want.Wall, want.PTS, want.Data, 128<<10)
	if len(messages) != 1 {
		t.Fatalf("a small picture took %d messages", len(messages))
	}
	event, done, err := p.Binary(messages[0])
	if err != nil || !done {
		t.Fatalf("done=%v err=%v", done, err)
	}
	got := event.Sample
	if got.Key != want.Key || !got.Wall.Equal(want.Wall) || got.PTS != want.PTS || !bytes.Equal(got.Data, want.Data) {
		t.Errorf("the picture read back differently: key=%v wall=%v pts=%v len=%d", got.Key, got.Wall, got.PTS, len(got.Data))
	}
}

// TestAPictureInPiecesIsJoined is the keyframe over the node's 128 KiB cap, and
// it is read at the sizes that sit on the edges of a piece.
func TestAPictureInPiecesIsJoined(t *testing.T) {
	picture := bytes.Repeat([]byte{0x65, 1, 2, 3, 4, 5, 6, 7}, 40_000)
	for _, max := range []int{20, 64, 1000, 128 << 10, len(picture) + 18, len(picture) + 17} {
		var p live.Parser
		pieces := wstest.SampleMessages(true, time.UnixMilli(5), 40*time.Millisecond, picture, max)
		var got live.Event
		for i, piece := range pieces {
			event, done, err := p.Binary(piece)
			if err != nil {
				t.Fatalf("max %d piece %d: %v", max, i, err)
			}
			if done != (i == len(pieces)-1) {
				t.Fatalf("max %d: piece %d of %d reported done=%v", max, i, len(pieces), done)
			}
			got = event
		}
		if !bytes.Equal(got.Sample.Data, picture) || !got.Sample.Key || got.Sample.PTS != 40*time.Millisecond {
			t.Errorf("max %d: the joined picture is %d bytes, want %d", max, len(got.Sample.Data), len(picture))
		}
	}
}

func TestTwoPicturesInARowDoNotShareABuffer(t *testing.T) {
	var p live.Parser
	a := wstest.SampleMessages(true, time.UnixMilli(1), 0, []byte{1, 1, 1, 1}, 100)
	b := wstest.SampleMessages(false, time.UnixMilli(2), time.Second, []byte{2, 2, 2, 2}, 100)
	first, _, _ := p.Binary(a[0])
	second, _, _ := p.Binary(b[0])
	if !bytes.Equal(first.Sample.Data, []byte{1, 1, 1, 1}) || !bytes.Equal(second.Sample.Data, []byte{2, 2, 2, 2}) {
		t.Errorf("the pictures are %v and %v", first.Sample.Data, second.Sample.Data)
	}
}

func TestMalformedBinaryMessagesAreErrorsAndTheParserGoesOn(t *testing.T) {
	var p live.Parser
	cases := []struct {
		name    string
		message []byte
		want    error
	}{
		{"empty", nil, live.ErrShort},
		{"a sample without its header", []byte{1, 0, 0}, live.ErrShort},
		{"a continuation without its flags", []byte{2}, live.ErrShort},
		{"a continuation with no picture open", []byte{2, 0, 9, 9}, live.ErrOrphan},
		{"a kind nobody sends", []byte{7, 0, 1, 2}, live.ErrUnknownKind},
	}
	for _, c := range cases {
		if _, _, err := p.Binary(c.message); !errors.Is(err, c.want) {
			t.Errorf("%s answered %v, want %v", c.name, err, c.want)
		}
	}

	// And it still reads a picture afterwards.
	pieces := wstest.SampleMessages(true, time.UnixMilli(1), 0, []byte{9, 9}, 100)
	if _, done, err := p.Binary(pieces[0]); err != nil || !done {
		t.Errorf("after the errors a picture answered done=%v err=%v", done, err)
	}
}

func TestAPictureThatBeginsBeforeTheLastEndedDropsTheLast(t *testing.T) {
	var p live.Parser
	long := wstest.SampleMessages(true, time.UnixMilli(1), 0, bytes.Repeat([]byte{1}, 100), 40)
	if _, done, err := p.Binary(long[0]); done || err != nil {
		t.Fatalf("the first piece: done=%v err=%v", done, err)
	}
	next := wstest.SampleMessages(false, time.UnixMilli(2), time.Second, []byte{2, 2}, 40)
	event, done, err := p.Binary(next[0])
	if !errors.Is(err, live.ErrInterrupted) || !done || !bytes.Equal(event.Sample.Data, []byte{2, 2}) {
		t.Errorf("the interrupting picture answered done=%v err=%v data=%v", done, err, event.Sample.Data)
	}
}

func TestAPictureOverTheLimitIsDropped(t *testing.T) {
	p := live.Parser{Max: 100}
	pieces := wstest.SampleMessages(true, time.UnixMilli(1), 0, bytes.Repeat([]byte{1}, 300), 60)
	var failed error
	for _, piece := range pieces {
		if _, _, err := p.Binary(piece); err != nil {
			failed = err
			break
		}
	}
	if !errors.Is(failed, live.ErrTooLarge) {
		t.Errorf("a picture of 300 bytes under a limit of 100 answered %v", failed)
	}

	// The next picture starts clean.
	small := wstest.SampleMessages(true, time.UnixMilli(1), 0, []byte{1, 2}, 60)
	if _, done, err := p.Binary(small[0]); err != nil || !done {
		t.Errorf("after the drop a picture answered done=%v err=%v", done, err)
	}
}

func TestANewTrackDropsAPictureLeftOpen(t *testing.T) {
	track, _ := load(t, "h264")
	var p live.Parser
	long := wstest.SampleMessages(true, time.UnixMilli(1), 0, bytes.Repeat([]byte{1}, 100), 40)
	p.Binary(long[0])
	if _, err := p.Text([]byte(wstest.TrackMessage(track.Codec, track.Width, track.Height, track.Description))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Binary(long[1]); !errors.Is(err, live.ErrOrphan) {
		t.Errorf("a piece of the old stream answered %v after a new track, want ErrOrphan", err)
	}
}

func TestTheUnitsOfAPictureAreSplitByTheirLengths(t *testing.T) {
	_, samples := load(t, "h264")
	units, err := samples[0].NALUnits(4)
	if err != nil || len(units) != 1 {
		t.Fatalf("units: %d, %v", len(units), err)
	}
	if units[0][0]&0x1F != 5 {
		t.Errorf("the first picture's unit is NAL type %d, want an IDR slice", units[0][0]&0x1F)
	}

	for _, bad := range [][]byte{{0, 0, 0}, {0, 0, 0, 9, 1, 2}} {
		if _, err := (live.Sample{Data: bad}).NALUnits(4); err == nil {
			t.Errorf("%v was split", bad)
		}
	}
	if _, err := (live.Sample{Data: []byte{0, 1, 2}}).NALUnits(5); err == nil {
		t.Error("a length size of 5 was accepted")
	}
}
