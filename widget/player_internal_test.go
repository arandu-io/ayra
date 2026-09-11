package widget

import (
	"image"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// transport is a frame loop with a pointer attached, which is what a drag needs
// to be tested at all.
//
// A drag is three events a frame apart -- press, move, release -- and the state
// under test is what survives between them. Faking the drag by writing into the
// slider would test the arithmetic and skip the thing that actually goes wrong,
// which is the order the caller's position and the pointer's are written in.
type transport struct {
	router input.Router
	ops    op.Ops
	shaper *text.Shaper
	width  int
}

// newTransport builds the loop at a width in pixels, at one pixel per point so
// that a coordinate in a test is a coordinate on the track.
func newTransport(t *testing.T, width int) *transport {
	t.Helper()
	return &transport{
		shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
		width:  width,
	}
}

// frame draws one frame and hands the router the operations it produced, which
// is what routes the next pointer event to whatever registered an area.
func (h *transport) frame(draw func(ayra.Context)) {
	h.ops.Reset()

	c := ayra.Context{
		Context: layout.Context{
			Ops:         &h.ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(h.width, 1000)},
			Source:      h.router.Source(),
		},
		Theme:  theme.New(theme.Light),
		Shaper: h.shaper,
	}

	draw(c)
	h.router.Frame(&h.ops)
}

// at queues one pointer event on the track.
func (h *transport) at(kind pointer.Kind, x float32) {
	h.router.Queue(pointer.Event{
		Kind:     kind,
		Source:   pointer.Mouse,
		Buttons:  pointer.ButtonPrimary,
		Position: f32.Pt(x, 10),
	})
}

// scrubbing is the media under test: two hundred seconds across two hundred
// pixels, so that one pixel is one second and an assertion reads as a time.
const (
	scrubWidth  = 200
	scrubLength = 200 * time.Second
)

// TestTheThumbStaysUnderTheFingerWhileTheMediaRuns is the defect this control
// exists around.
//
// The caller supplies a new Position every frame while the media plays. If that
// position is written into the scrubber while a finger is on it, the thumb
// snaps back to where the media is between one frame and the next, for as long
// as somebody holds it -- and the scrubber becomes a control that can only be
// dragged to the one place nobody needs it.
func TestTheThumbStaysUnderTheFingerWhileTheMediaRuns(t *testing.T) {
	h := newTransport(t, scrubWidth)
	state := &Player{}

	draw := func(position time.Duration) func(ayra.Context) {
		return func(c ayra.Context) {
			PlayerProps{Duration: scrubLength, Position: position, Playing: true}.scrubber(c, state)
		}
	}

	// A frame at rest registers the track, so the press has something to land
	// on.
	h.frame(draw(0))

	h.at(pointer.Press, 150)
	h.frame(draw(1 * time.Second))

	to, scrubbing := state.Scrubbing()
	if !scrubbing {
		t.Fatal("a press on the track did not take hold of the scrubber")
	}
	if to != 150*time.Second {
		t.Fatalf("the press at 150 of 200 pixels took hold at %v, want 2m30s", to)
	}

	// The media keeps playing under the finger: every frame the caller says the
	// position has advanced, and every frame the thumb has to ignore it.
	for _, position := range []time.Duration{2 * time.Second, 3 * time.Second, 4 * time.Second} {
		h.frame(draw(position))

		to, scrubbing = state.Scrubbing()
		if !scrubbing {
			t.Fatalf("at %v the scrubber let go of a finger that is still down", position)
		}
		if to != 150*time.Second {
			t.Fatalf("at %v the thumb moved to %v: the caller's position was written over the drag", position, to)
		}
	}
}

// TestTheThumbFollowsTheFingerAcrossTheTrack keeps the hold from being a freeze.
//
// Ignoring the caller's position is only half of it. The other half is that the
// pointer's own movement still has to reach the value, and a hold written as
// "stop writing to the slider" would pin the thumb to wherever the press landed
// and make a drag impossible.
func TestTheThumbFollowsTheFingerAcrossTheTrack(t *testing.T) {
	h := newTransport(t, scrubWidth)
	state := &Player{}

	draw := func(position time.Duration) func(ayra.Context) {
		return func(c ayra.Context) {
			PlayerProps{Duration: scrubLength, Position: position, Playing: true}.scrubber(c, state)
		}
	}

	h.frame(draw(0))
	h.at(pointer.Press, 150)
	h.frame(draw(1 * time.Second))

	h.at(pointer.Move, 50)
	h.frame(draw(2 * time.Second))

	to, scrubbing := state.Scrubbing()
	if !scrubbing {
		t.Fatal("moving the pointer ended the drag")
	}
	if to != 50*time.Second {
		t.Fatalf("the finger moved to 50 of 200 pixels and the thumb reads %v, want 50s", to)
	}
}

// TestLettingGoAsksForOneSeek fixes the schedule: a drag reports nothing until
// the finger comes off, and then reports once.
func TestLettingGoAsksForOneSeek(t *testing.T) {
	h := newTransport(t, scrubWidth)
	state := &Player{}

	draw := func(position time.Duration) func(ayra.Context) {
		return func(c ayra.Context) {
			PlayerProps{Duration: scrubLength, Position: position, Playing: true}.scrubber(c, state)
		}
	}

	h.frame(draw(0))
	h.at(pointer.Press, 150)
	h.frame(draw(1 * time.Second))

	if _, asked := state.Sought(); asked {
		t.Fatal("a seek was reported while the finger was still down")
	}

	h.frame(draw(2 * time.Second))
	if _, asked := state.Sought(); asked {
		t.Fatal("a seek was reported on a frame in the middle of the drag")
	}

	h.at(pointer.Release, 150)
	h.frame(draw(3 * time.Second))

	sought, asked := state.Sought()
	if !asked {
		t.Fatal("letting go asked for no seek at all")
	}
	if sought != 150*time.Second {
		t.Errorf("the seek landed at %v, want the 2m30s the finger was at", sought)
	}
	if _, again := state.Sought(); again {
		t.Error("the same seek was reported twice: an intent is consumed when it is read")
	}
}

// TestTheThumbReturnsToTheCallerOnceTheFingerIsOff is the other side of the
// hold: the moment the drag ends, the media is the authority again.
//
// Without it the thumb would stay wherever it was left and the transport would
// stop following the media entirely -- which looks correct for a second and
// then never moves again.
func TestTheThumbReturnsToTheCallerOnceTheFingerIsOff(t *testing.T) {
	h := newTransport(t, scrubWidth)
	state := &Player{}

	draw := func(position time.Duration) func(ayra.Context) {
		return func(c ayra.Context) {
			PlayerProps{Duration: scrubLength, Position: position, Playing: true}.scrubber(c, state)
		}
	}

	h.frame(draw(0))
	h.at(pointer.Press, 150)
	h.frame(draw(1 * time.Second))
	h.at(pointer.Release, 150)
	h.frame(draw(2 * time.Second))

	h.frame(draw(20 * time.Second))

	if _, scrubbing := state.Scrubbing(); scrubbing {
		t.Fatal("the scrubber still reports a finger on it after the release")
	}
	if got, want := state.scrub.Value(), fraction(20*time.Second, scrubLength); got != want {
		t.Errorf("the thumb sits at %v, want the %v the caller reported", got, want)
	}
}

// TestATapOnTheTrackAsksForASeek keeps the commonest jump working.
//
// A press and a release inside one frame never registers as a drag, so a hold
// written only around the drag state drops it silently: tapping halfway along a
// recording does nothing at all, which reads as a track that is not a control.
func TestATapOnTheTrackAsksForASeek(t *testing.T) {
	state := &Player{}

	written := state.follow(10*time.Second, scrubLength, false)
	state.scrub.SetValue(0.25) // what the pointer did inside the slider
	state.settle(scrubLength, false, written)

	sought, asked := state.Sought()
	if !asked {
		t.Fatal("a tap on the track asked for nothing")
	}
	if sought != 50*time.Second {
		t.Errorf("the tap a quarter along asked for %v, want 50s", sought)
	}
}

// TestAFrameWithNothingTouchedAsksForNothing is the guard against a control
// that reports the same event every frame.
//
// The scrubber is written to on every frame it is not held, and a check that
// only asked "did the value change since last frame" would answer yes whenever
// the media moved -- a seek per frame, to where the media already is.
func TestAFrameWithNothingTouchedAsksForNothing(t *testing.T) {
	state := &Player{}

	for _, position := range []time.Duration{0, time.Second, 2 * time.Second, time.Hour} {
		written := state.follow(position, scrubLength, false)
		state.settle(scrubLength, false, written)

		if sought, asked := state.Sought(); asked {
			t.Fatalf("the media moving to %v produced a seek to %v", position, sought)
		}
	}
}

// TestAnIgnoredSeekIsNotAskedForAgain fixes what happens when the caller does
// not obey.
//
// A caller can refuse: the media has ended, the file is gone, the seek failed.
// The position it keeps reporting is then the old one, and a control that
// treated the difference between its thumb and the prop as a new ask would send
// the same seek forever.
func TestAnIgnoredSeekIsNotAskedForAgain(t *testing.T) {
	state := &Player{}

	written := state.follow(10*time.Second, scrubLength, false)
	state.scrub.SetValue(0.9)
	state.settle(scrubLength, false, written)

	if _, asked := state.Sought(); !asked {
		t.Fatal("the tap asked for nothing")
	}

	for frame := 0; frame < 5; frame++ {
		written = state.follow(10*time.Second, scrubLength, false)
		state.settle(scrubLength, false, written)

		if _, asked := state.Sought(); asked {
			t.Fatalf("frame %d asked for the seek again after the caller ignored it", frame)
		}
	}
}

// TestTheElapsedColumnReadsTheDragRatherThanTheMedia fixes what the number
// beside the thumb is for.
//
// While a finger is on the scrubber the only thing that says where letting go
// will land is that figure. Reading the media's own position there leaves a
// person dragging with no idea where they are going.
func TestTheElapsedColumnReadsTheDragRatherThanTheMedia(t *testing.T) {
	state := &Player{}
	props := PlayerProps{Duration: scrubLength, Position: 5 * time.Second}

	if got := props.elapsed(state); got != 5*time.Second {
		t.Fatalf("with nothing held the column reads %v, want the media's 5s", got)
	}

	state.scrubbing, state.scrubbedTo = true, 120*time.Second

	if got := props.elapsed(state); got != 120*time.Second {
		t.Errorf("while dragging the column reads %v, want the 2m0s being dragged to", got)
	}
}

// TestAClockReadsAsAClock is the table the time formatting is fixed by.
func TestAClockReadsAsAClock(t *testing.T) {
	for _, test := range []struct {
		in   time.Duration
		want string
	}{
		{0, "0:00"},
		{time.Second, "0:01"},
		{7 * time.Second, "0:07"},
		{59 * time.Second, "0:59"},
		{67 * time.Second, "1:07"},
		{187 * time.Second, "3:07"},
		{599 * time.Second, "9:59"},
		{600 * time.Second, "10:00"},
		{3599 * time.Second, "59:59"},
		{3600 * time.Second, "1:00:00"},
		{3661 * time.Second, "1:01:01"},
		{90 * time.Minute, "1:30:00"},
		{10 * time.Hour, "10:00:00"},
		{100 * time.Hour, "100:00:00"},
	} {
		if got := formatDuration(test.in); got != test.want {
			t.Errorf("%v wrote %q, want %q", test.in, got, test.want)
		}
	}
}

// TestTheHourBoundaryIsWhereTheThirdFieldAppears fixes the one rounding
// boundary this has: the second before an hour is still m:ss, and the hour
// itself is h:mm:ss.
func TestTheHourBoundaryIsWhereTheThirdFieldAppears(t *testing.T) {
	before := formatDuration(59*time.Minute + 59*time.Second)
	after := formatDuration(time.Hour)

	if before != "59:59" {
		t.Errorf("one second before an hour wrote %q, want %q", before, "59:59")
	}
	if after != "1:00:00" {
		t.Errorf("exactly an hour wrote %q, want %q", after, "1:00:00")
	}
	if strings.Count(before, ":") != 1 || strings.Count(after, ":") != 2 {
		t.Errorf("the two sides of the boundary have the same shape: %q and %q", before, after)
	}
}

// TestASecondIsNotShownBeforeItHasPassed fixes truncation over rounding.
//
// A clock that rounded would read "0:01" before the first second was over, and
// would read one second past the end of the media as it finished -- which is
// the moment somebody is most likely to be looking at it.
func TestASecondIsNotShownBeforeItHasPassed(t *testing.T) {
	for _, test := range []struct {
		in   time.Duration
		want string
	}{
		{900 * time.Millisecond, "0:00"},
		{1900 * time.Millisecond, "0:01"},
		{3599*time.Second + 999*time.Millisecond, "59:59"},
		{-1900 * time.Millisecond, "-0:01"},
	} {
		if got := formatDuration(test.in); got != test.want {
			t.Errorf("%v wrote %q, want %q", test.in, got, test.want)
		}
	}
}

// TestTheMinutesCarryALeadingZeroOnlyWhenTheyAreAMiddleField fixes the
// difference between the two forms.
//
// Under an hour the minutes lead, and a leading zero there puts a column on
// screen that is nought for almost everything anybody plays. Past an hour they
// are a middle field, and a middle field of variable width is not a column.
func TestTheMinutesCarryALeadingZeroOnlyWhenTheyAreAMiddleField(t *testing.T) {
	if got := formatDuration(187 * time.Second); got != "3:07" {
		t.Errorf("under an hour wrote %q, want %q without the padded minute", got, "3:07")
	}
	if got := formatDuration(time.Hour + 7*time.Minute); got != "1:07:00" {
		t.Errorf("past an hour wrote %q, want %q with the padded minute", got, "1:07:00")
	}
}

// TestANegativeTimeCarriesOneSignAtTheFront fixes what a caller subtracting a
// position from a length it has not learned yet gets, and what the remaining
// column is made of.
func TestANegativeTimeCarriesOneSignAtTheFront(t *testing.T) {
	for _, test := range []struct {
		in   time.Duration
		want string
	}{
		{-83 * time.Second, "-1:23"},
		{-3661 * time.Second, "-1:01:01"},
		{-time.Hour, "-1:00:00"},
		// Under a second there is nothing left to count down, and "-0:00" is a
		// sign on a nought.
		{-500 * time.Millisecond, "0:00"},
		{0, "0:00"},
	} {
		got := formatDuration(test.in)
		if got != test.want {
			t.Errorf("%v wrote %q, want %q", test.in, got, test.want)
		}
		if index := strings.LastIndex(got, "-"); index > 0 {
			t.Errorf("%q carries a sign in the middle, at %d", got, index)
		}
	}
}

// TestTheMostNegativeDurationDoesNotTurnPositive is the overflow this kind of
// code is written wrong by.
//
// Negating the most negative duration answers itself, still negative. A formatter
// that negated the duration to take its magnitude would then divide a negative
// by sixty and write a minus in front of every field it produced.
func TestTheMostNegativeDurationDoesNotTurnPositive(t *testing.T) {
	for _, extreme := range []time.Duration{math.MinInt64, math.MaxInt64} {
		got := formatDuration(extreme)

		if index := strings.LastIndex(got, "-"); index > 0 {
			t.Errorf("%d wrote %q, which carries a sign at %d", extreme, got, index)
		}
		if extreme < 0 && !strings.HasPrefix(got, "-") {
			t.Errorf("%d wrote %q, which reads as a time in the future", extreme, got)
		}
		if extreme > 0 && strings.Contains(got, "-") {
			t.Errorf("%d wrote %q, which carries a sign it should not have", extreme, got)
		}
		if strings.Count(got, ":") != 2 {
			t.Errorf("%d wrote %q, which is not a clock", extreme, got)
		}
	}
}

// TestALengthNobodyKnowsIsNotADenominator fixes every ratio in this control.
//
// Media whose length has not been read reports a duration of zero, and it is
// the denominator of the position, of the thumb and of the seek.
func TestALengthNobodyKnowsIsNotADenominator(t *testing.T) {
	if got := fraction(42*time.Second, 0); got != 0 {
		t.Errorf("a position in media of unknown length answered %v, want 0", got)
	}
	if got := at(0.5, 0); got != 0 {
		t.Errorf("half of an unknown length answered %v, want 0", got)
	}
	if got := at(0.5, -time.Second); got != 0 {
		t.Errorf("half of a negative length answered %v, want 0", got)
	}
}

// TestAPositionOutsideTheMediaIsClamped keeps a thumb on its own track.
//
// Media reporting one frame past its own end is ordinary, and an unclamped
// ratio draws the thumb past the end of the track, over whatever is beside it.
func TestAPositionOutsideTheMediaIsClamped(t *testing.T) {
	if got := fraction(2*time.Minute, time.Minute); got != 1 {
		t.Errorf("a position past the end answered %v, want 1", got)
	}
	if got := fraction(-time.Minute, time.Minute); got != 0 {
		t.Errorf("a position before the start answered %v, want 0", got)
	}
	if got := at(2, time.Minute); got != time.Minute {
		t.Errorf("a fraction above one answered %v, want the whole minute", got)
	}
	if got := at(-1, time.Minute); got != 0 {
		t.Errorf("a fraction below nought answered %v, want 0", got)
	}
}

// TestTheRemainingColumnCountsDownAndSaysSo fixes the sign on the right-hand
// clock, which is the only thing distinguishing it from the left-hand one.
func TestTheRemainingColumnCountsDownAndSaysSo(t *testing.T) {
	state := &Player{}
	props := PlayerProps{Duration: 4 * time.Minute, Position: 83 * time.Second}

	if got := props.remaining(state); got != "-2:37" {
		t.Errorf("the remaining column reads %q, want %q", got, "-2:37")
	}

	unknown := PlayerProps{Position: 83 * time.Second}
	if got := unknown.remaining(state); got != "" {
		t.Errorf("media of unknown length has a remaining time of %q, and there is none to have", got)
	}
}

// TestTheRateCyclesThroughTheSetAndComesBackToNormal keeps a person able to
// undo a press.
func TestTheRateCyclesThroughTheSetAndComesBackToNormal(t *testing.T) {
	if speedIndex(normalSpeed) < 0 {
		t.Fatal("the offered rates do not include normal, so nothing can go back to it")
	}

	rate := normalSpeed
	seen := map[float32]bool{}
	for press := 0; press < len(playbackSpeeds); press++ {
		if seen[rate] {
			t.Fatalf("the cycle repeated %v before it had visited every rate", rate)
		}
		seen[rate] = true
		rate = nextSpeed(rate)
	}

	if rate != normalSpeed {
		t.Errorf("a full cycle of %d presses ended at %v, want normal", len(playbackSpeeds), rate)
	}
}

// TestAnUnknownRateAdvancesFromNormal keeps the first press on a fresh player
// from doing nothing.
//
// A caller that never set a rate leaves zero, and the button already reads
// "1x". Answering normal would be a press that changes neither the label nor
// the media, which is indistinguishable from a control that does not work.
func TestAnUnknownRateAdvancesFromNormal(t *testing.T) {
	for _, unknown := range []float32{0, -1, 3, 0.1} {
		got := nextSpeed(unknown)

		if got == normalSpeed {
			t.Errorf("a rate of %v advanced to normal, which is what the button already reads", unknown)
		}
		if speedIndex(got) < 0 {
			t.Errorf("a rate of %v advanced to %v, which is not one this control offers", unknown, got)
		}
	}
}

// TestARateIsLabelledAsARate fixes what the button says.
func TestARateIsLabelledAsARate(t *testing.T) {
	for _, test := range []struct {
		in   float32
		want string
	}{
		{0.5, "0.5x"},
		{0.75, "0.75x"},
		{1, "1x"},
		{1.25, "1.25x"},
		{2, "2x"},
		// A caller that never set one, and a stopped clock, both read as normal.
		{0, "1x"},
		{-2, "1x"},
	} {
		if got := speedLabel(test.in); got != test.want {
			t.Errorf("%v wrote %q, want %q", test.in, got, test.want)
		}
	}
}

// TestEveryIntentIsReportedOnceAndConsumed fixes the contract every one of them
// shares.
//
// A control that answered the same press on the following frame would toggle
// play twice, seek twice, and mute and unmute in the time it takes to let go.
func TestEveryIntentIsReportedOnceAndConsumed(t *testing.T) {
	state := &Player{}

	state.toggled = true
	state.muted = true
	state.sought, state.hasSought = time.Minute, true
	state.volume, state.hasVolume = 0.4, true
	state.speed, state.hasSpeed = 1.5, true

	if !state.Toggled() || !state.MuteToggled() {
		t.Fatal("an intent that was recorded was not reported")
	}
	if _, asked := state.Sought(); !asked {
		t.Fatal("a seek that was recorded was not reported")
	}
	if _, asked := state.VolumeSet(); !asked {
		t.Fatal("a volume that was recorded was not reported")
	}
	if _, asked := state.SpeedSet(); !asked {
		t.Fatal("a rate that was recorded was not reported")
	}

	if state.Toggled() {
		t.Error("play was asked for twice from one press")
	}
	if state.MuteToggled() {
		t.Error("mute was asked for twice from one press")
	}
	if _, asked := state.Sought(); asked {
		t.Error("a seek was asked for twice from one release")
	}
	if _, asked := state.VolumeSet(); asked {
		t.Error("a volume was asked for twice from one move")
	}
	if _, asked := state.SpeedSet(); asked {
		t.Error("a rate was asked for twice from one press")
	}
}

// TestScrubbingIsAReadingRatherThanAnIntent fixes the one accessor that is not
// consumed.
//
// It answers where the finger is, and a caller drawing a preview beside the
// transport asks it as often as it likes within a frame.
func TestScrubbingIsAReadingRatherThanAnIntent(t *testing.T) {
	state := &Player{}
	state.scrubbing, state.scrubbedTo = true, 90*time.Second

	for ask := 0; ask < 3; ask++ {
		to, scrubbing := state.Scrubbing()
		if !scrubbing || to != 90*time.Second {
			t.Fatalf("ask %d answered %v, %v: a reading does not run out", ask, to, scrubbing)
		}
	}
}

// TestTheClockColumnFitsEveryTimeItWillEverDraw is what keeps the scrubber from
// sliding sideways once a second.
//
// The column is measured once, from the longest string the media can produce.
// If any reading along the way is wider, the row is laid out again on the tick
// that produces it and the thumb somebody is aiming at moves.
func TestTheClockColumnFitsEveryTimeItWillEverDraw(t *testing.T) {
	h := newTransport(t, 600)

	for _, duration := range []time.Duration{30 * time.Second, 9 * time.Minute, 59 * time.Minute, 95 * time.Minute, 3 * time.Hour} {
		props := PlayerProps{Duration: duration}

		h.frame(func(c ayra.Context) {
			column := props.timeColumn(c)
			if column <= 0 {
				t.Fatalf("a %v recording reserved no room for its clock", duration)
			}

			step := max(duration/97, time.Second)
			for position := time.Duration(0); position <= duration; position += step {
				for _, reading := range []string{formatDuration(position), formatDuration(position - duration)} {
					measured := measureClock(c, reading)
					if measured > column {
						t.Errorf("a %v recording reserved %d for its clock and %q needs %d", duration, column, reading, measured)
					}
				}
			}
		})
	}
}

// measureClock is the width one reading takes, in the face the clock is drawn
// in.
func measureClock(c ayra.Context, reading string) int {
	c.Constraints.Min = image.Point{}

	measure := op.Record(c.Ops)
	dims := drawText(c, reading, unit.Sp(c.Theme.Type.Mono), c.Theme.Colours.MutedForeground, 1, text.Middle, mono())
	measure.Stop()

	return dims.Size.X
}

// TestTheRateColumnFitsEveryRateOffered keeps the button from resizing under a
// press and moving the clock beside it.
func TestTheRateColumnFitsEveryRateOffered(t *testing.T) {
	h := newTransport(t, 600)

	h.frame(func(c ayra.Context) {
		column := PlayerProps{ShowSpeed: true}.speedColumn(c)
		if column <= 0 {
			t.Fatal("the rate control reserved no room")
		}

		for _, rate := range playbackSpeeds {
			c.Constraints.Min = image.Point{}
			measure := op.Record(c.Ops)
			dims := drawText(c, speedLabel(rate), unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Middle, mono())
			measure.Stop()

			if dims.Size.X > column {
				t.Errorf("the rate column is %d and %q needs %d", column, speedLabel(rate), dims.Size.X)
			}
		}
	})
}

// TestThePlayerIsNeverBuiltFromACopyOfItsOwnState extends the guard this
// package keeps for every other control.
//
// A player constructed from a copy of its buttons or its drags is a fresh
// control every frame: it never sees the release of its own press, so the
// transport can be held down and do nothing, and the scrubber can be dragged
// and never report.
func TestThePlayerIsNeverBuiltFromACopyOfItsOwnState(t *testing.T) {
	copies := []string{"Player{play:", "Player{scrub:", "Player{mute:", "Player{sound:", "Player{rate:"}

	for _, path := range sources(t) {
		body := read(t, path)
		for _, spelling := range copies {
			if strings.Contains(body, spelling) {
				t.Errorf("%s constructs a player from a copy of its state (%s); one made per frame never sees the release of its own press", path, spelling)
			}
		}
	}
}

// probe presses the whole transport at one fixed point on its track and answers
// the position that press took hold at.
//
// It is how a test sees where the track begins and how wide it is, which is not
// in anything the control returns: the row fills the column it is given
// whatever the scrubber does with the room left over, so its dimensions say
// nothing about it. The same press landing on a different time is the track
// having moved.
func probe(t *testing.T, x float32, props PlayerProps) (time.Duration, bool) {
	t.Helper()

	h := newTransport(t, 600)
	state := &Player{}
	draw := func(c ayra.Context) { props.Layout(c, state) }

	h.frame(draw)
	h.at(pointer.Press, x)
	h.frame(draw)

	return state.Scrubbing()
}

// TestTheTrackDoesNotMoveAsTheClockRuns is the reserved clock column seen from
// the one place it matters: where a press lands.
//
// The clocks either side of the scrubber change every second. Sized to their
// own text, "9:59" becoming "10:00" takes a digit's width out of the track and
// moves its left edge, so the point somebody is aiming at becomes a different
// time -- once a second, for the length of the recording.
func TestTheTrackDoesNotMoveAsTheClockRuns(t *testing.T) {
	landings := map[time.Duration][]time.Duration{}

	for _, position := range []time.Duration{
		0,
		9 * time.Second,
		10 * time.Second,
		9*time.Minute + 59*time.Second,
		10 * time.Minute,
		59*time.Minute + 59*time.Second,
		time.Hour,
		2 * time.Hour,
	} {
		to, scrubbing := probe(t, 300, PlayerProps{Duration: 2*time.Hour + 14*time.Minute, Position: position, Playing: true})
		if !scrubbing {
			t.Fatalf("a press at 300 with the clock reading %v did not land on the track", position)
		}
		landings[to] = append(landings[to], position)
	}

	if len(landings) != 1 {
		t.Errorf("the same press landed on different times as the clock ran: %v", landings)
	}
}

// TestTheTrackDoesNotMoveWhenAMarkChanges keeps a press from moving everything
// beside it.
//
// The play mark is a triangle and the pause mark is two bars; the speaker gains
// a cross when it is silenced. Sized to their drawings, each of those buttons
// would change width the moment its state changed and drag the track along with
// it.
func TestTheTrackDoesNotMoveWhenAMarkChanges(t *testing.T) {
	base := PlayerProps{Duration: scrubLength, Position: 30 * time.Second, Volume: 0.6}

	stopped, ok := probe(t, 300, base)
	if !ok {
		t.Fatal("the press did not land on the track")
	}

	for name, props := range map[string]PlayerProps{
		"playing": {Duration: base.Duration, Position: base.Position, Volume: base.Volume, Playing: true},
		"muted":   {Duration: base.Duration, Position: base.Position, Volume: base.Volume, Muted: true},
		"silent":  {Duration: base.Duration, Position: base.Position},
	} {
		to, ok := probe(t, 300, props)
		if !ok {
			t.Fatalf("%s: the press did not land on the track", name)
		}
		if to != stopped {
			t.Errorf("%s: the press landed at %v rather than %v, so the mark changed the row's widths", name, to, stopped)
		}
	}
}

// TestARowCarryingLessGivesTheRoomToTheTrack fixes what Compact and ShowSpeed
// are for.
//
// A field that changed nothing about the drawing would be a word in the API
// that means nothing, and the room those controls take comes out of the one
// control everybody aims at.
func TestARowCarryingLessGivesTheRoomToTheTrack(t *testing.T) {
	media := PlayerProps{Duration: scrubLength, Position: 30 * time.Second, Volume: 0.6}

	withRate, ok := probe(t, 300, PlayerProps{Duration: media.Duration, Position: media.Position, Volume: media.Volume, ShowSpeed: true})
	if !ok {
		t.Fatal("with the rate control the press did not land on the track")
	}
	full, ok := probe(t, 300, media)
	if !ok {
		t.Fatal("the press did not land on the track")
	}
	compact, ok := probe(t, 300, PlayerProps{Duration: media.Duration, Position: media.Position, Volume: media.Volume, Compact: true})
	if !ok {
		t.Fatal("compact: the press did not land on the track")
	}

	// A wider track puts the same point earlier in the recording, because the
	// point is a smaller fraction of a longer line.
	if !(compact < full && full < withRate) {
		t.Errorf("the same press landed at %v compact, %v full and %v with the rate control; a row carrying less has to widen the track", compact, full, withRate)
	}
}

// TestANarrowRowDrawsTheCompactTransport fixes what happens when the column is
// too small for everything.
//
// The alternative is a scrubber a few pixels wide that nobody can aim at, with
// the volume slider drawn over the remaining time beside it.
func TestANarrowRowDrawsTheCompactTransport(t *testing.T) {
	wide := newTransport(t, 600)
	narrow := newTransport(t, 190)

	props := PlayerProps{Duration: scrubLength, Position: 30 * time.Second, Volume: 0.6, ShowSpeed: true}

	var full, squeezed int
	wide.frame(func(c ayra.Context) { full = props.Layout(c, &Player{}).Size.X })
	narrow.frame(func(c ayra.Context) { squeezed = props.Layout(c, &Player{}).Size.X })

	if squeezed > 190 {
		t.Errorf("at 190 the transport took %d, so the row overflowed rather than dropping what does not fit", squeezed)
	}
	if full > 600 {
		t.Errorf("at 600 the transport took %d", full)
	}
}

// TestTheLastControlInTheRowIsStillReachable is how a row wider than its column
// is actually seen.
//
// A flex reports its own size clamped to the constraints it was given, so a row
// that does not fit answers the width of the column and looks correct to
// anything that measures it. What has really happened is that everything after
// the scrubber is placed past the right edge, where no press can reach it: the
// sound control is drawn, is the right size, and does nothing.
//
// The track's width is the difference between the room there is and what the
// other controls take, so it is exactly the sum this catches being wrong.
func TestTheLastControlInTheRowIsStillReachable(t *testing.T) {
	for _, width := range []int{240, 320, 480, 600, 900} {
		h := newTransport(t, width)
		state := &Player{}

		props := PlayerProps{Duration: scrubLength, Position: 30 * time.Second, Volume: 0.6, Compact: true}
		draw := func(c ayra.Context) { props.Layout(c, state) }

		// The sound control ends a compact row, so its middle is half a target
		// in from the right edge.
		middle := float32(width) - float32(playerTarget)/2

		h.frame(draw)
		h.at(pointer.Press, middle)
		h.frame(draw)
		h.at(pointer.Release, middle)
		h.frame(draw)

		if !state.MuteToggled() {
			t.Errorf("at %d a press at the right edge of the row reached nothing, so the row is wider than its column", width)
		}
	}
}

// TestTheVolumeFollowsTheCaller fixes the level the slider draws.
//
// Volume is a prop, so the drawn level is the caller's answer about what is in
// effect rather than a number the control keeps. A slider that never adopted it
// would sit at nought over media playing at full volume.
func TestTheVolumeFollowsTheCaller(t *testing.T) {
	h := newTransport(t, 600)
	state := &Player{}

	for _, level := range []float32{0, 0.25, 0.7, 1} {
		h.frame(func(c ayra.Context) {
			PlayerProps{Duration: scrubLength, Volume: level}.Layout(c, state)
		})

		if got := state.sound.Value(); got != level {
			t.Errorf("the caller reported a volume of %v and the slider drew %v", level, got)
		}
	}

	// Muted draws as silent, and the caller's level is still the caller's: it
	// is what unmuting goes back to, so the control must not write over it.
	h.frame(func(c ayra.Context) {
		PlayerProps{Duration: scrubLength, Volume: 0.7, Muted: true}.Layout(c, state)
	})
	if got := state.sound.Value(); got != 0 {
		t.Errorf("muted the slider drew %v, want silence", got)
	}
	if _, asked := state.VolumeSet(); asked {
		t.Error("drawing a muted transport asked to change the volume")
	}
}

// TestTheVolumeReportsWhileTheFingerIsStillDown fixes the schedule that is
// deliberately not the scrubber's.
//
// A volume is a number a mixer multiplies by, so obeying it a hundred times is
// a hundred multiplications and the sound follows the finger. Holding it back
// until the release would make a person let go to find out how loud it is.
func TestTheVolumeReportsWhileTheFingerIsStillDown(t *testing.T) {
	h := newTransport(t, 600)
	state := &Player{}

	draw := func(c ayra.Context) {
		PlayerProps{Duration: scrubLength, Position: 30 * time.Second, Volume: 0.2}.Layout(c, state)
	}

	h.frame(draw)

	// The sound slider ends the full row, so its middle is half its width in
	// from the right edge.
	h.at(pointer.Press, 600-float32(playerVolume)/2)
	h.frame(draw)

	level, asked := state.VolumeSet()
	if !asked {
		t.Fatal("a press on the volume reported nothing while the finger was still down")
	}
	if level < 0.45 || level > 0.55 {
		t.Errorf("a press halfway along the volume reported %v, want about half", level)
	}
}

// TestEachControlCarriesTheMarkForTheStateTheMediaIsIn fixes the two buttons
// whose shape is their whole label.
//
// Neither has any text on it, so the mark is the only thing that says what the
// control will do. A transport drawing the play triangle over running media is
// not a cosmetic fault: it tells a person the media is stopped.
func TestEachControlCarriesTheMarkForTheStateTheMediaIsIn(t *testing.T) {
	for _, test := range []struct {
		props PlayerProps
		play  mark
		sound mark
	}{
		{PlayerProps{Volume: 0.5}, markPlay, markSound},
		{PlayerProps{Playing: true, Volume: 0.5}, markPause, markSound},
		{PlayerProps{Playing: true, Volume: 0.5, Muted: true}, markPause, markSilent},
		// A level of nought sounds like a mute and has to read like one.
		{PlayerProps{Playing: true, Volume: 0}, markPause, markSilent},
	} {
		if got := test.props.transportMark(); got != test.play {
			t.Errorf("playing=%v draws %s on the transport, want %s", test.props.Playing, got, test.play)
		}
		if got := test.props.soundMark(); got != test.sound {
			t.Errorf("volume=%v muted=%v draws %s on the sound, want %s", test.props.Volume, test.props.Muted, got, test.sound)
		}
	}
}

// TestTheFourMarksAreFourDifferentShapes keeps the set from collapsing.
//
// Two marks resolving to one drawing is one mark with two names, and the second
// name is a promise the picture does not keep. It is checked by identity rather
// than by looking, because nothing in this package rasterises: a table sending
// pause to the play triangle compiles and draws, and the only other way to find
// it is a picture somebody approved.
func TestTheFourMarksAreFourDifferentShapes(t *testing.T) {
	drawings := map[uintptr]mark{}
	names := map[string]mark{}

	for _, shape := range []mark{markPlay, markPause, markSound, markSilent} {
		if int(shape) >= len(markShapes) || markShapes[shape] == nil {
			t.Fatalf("%v resolves to no drawing at all", shape)
		}

		drawn := reflect.ValueOf(markShapes[shape]).Pointer()
		if other, repeated := drawings[drawn]; repeated {
			t.Errorf("%v and %v are drawn by the same function", shape, other)
		}
		drawings[drawn] = shape

		if other, repeated := names[shape.String()]; repeated {
			t.Errorf("%v and %v answer to the same name", shape, other)
		}
		names[shape.String()] = shape
	}

	if len(markShapes) != 4 {
		t.Errorf("the table holds %d drawings for four marks", len(markShapes))
	}
}

// TestNoMarkOnTheTransportComesFromAGlyph fixes how the shapes are put on
// screen.
//
// A glyph comes from whichever font the platform happens to carry, and on one
// that is missing it a mark renders as a hollow box. A person can still work
// out a rating from four empty boxes. Nobody can tell play from pause, or sound
// from silence, and those two buttons have no text on them to fall back on.
func TestNoMarkOnTheTransportComesFromAGlyph(t *testing.T) {
	body := read(t, "player.go")

	marks := body[strings.Index(body, "func drawPlay("):]
	if strings.Contains(marks, "drawText(") || strings.Contains(marks, "drawLine(") {
		t.Error("a mark on the transport is set as text, so a platform missing the glyph draws a hollow box")
	}
	for _, drawn := range []string{"clip.Path", "paint.FillShape"} {
		if !strings.Contains(marks, drawn) {
			t.Errorf("the marks do not use %s, so they are not being drawn as shapes", drawn)
		}
	}
}
