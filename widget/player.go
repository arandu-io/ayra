package widget

import (
	"image"
	"image/color"
	"strconv"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// Player is the state half of a transport: the presses, the two drags, and the
// position a finger is holding the scrubber at.
//
// It decodes nothing and it owns no clock. Everything it draws comes from the
// props it is handed each frame, and everything a person asks of it comes back
// out as an intent for the caller to carry out. Both halves of that are
// deliberate. A control that ran a timer of its own would keep counting while
// the media was stalled on a slow network, so the elapsed time would drift away
// from the sound and never come back -- and it would be wrong in the one
// situation where a person is actually watching the number. A control that
// decoded would put a codec in a package whose job is to put shapes on a
// screen, and would mean a build of this library carried one whether the
// application played anything or not.
type Player struct {
	play  Button
	mute  Button
	rate  Button
	scrub Slider
	sound Slider

	// scrubbing and scrubbedTo are the whole of the scrub-while-playing
	// problem: while a finger is down, this is where the thumb is, and the
	// Position the caller supplies is ignored until the finger comes off.
	scrubbing  bool
	scrubbedTo time.Duration

	toggled bool
	muted   bool

	sought    time.Duration
	hasSought bool

	volume    float32
	hasVolume bool

	speed    float32
	hasSpeed bool
}

// Scrubbing reports the position a finger is holding the scrubber at.
//
// It is a reading rather than an intent, so it is not consumed: it answers the
// same thing every time it is asked within a frame, and a caller drawing a
// preview beside the transport needs exactly that. What was asked for is
// [Player.Sought], and that one is consumed.
func (s *Player) Scrubbing() (time.Duration, bool) { return s.scrubbedTo, s.scrubbing }

// Toggled reports that play or pause was asked for, once, and consumes it.
//
// It does not flip anything. Playing is a prop, which is to say it is the
// caller's answer about what the media is doing, and this control has no way to
// make the media do anything. One that flipped a flag of its own would draw a
// pause bar over a file that failed to open.
//
// It is read after Layout rather than before, because the press it reports
// belongs to a button this control lays out: there is nothing to hear until the
// buttons have been drawn. That is the opposite of a [Button], which is asked
// first because the caller owns the press there.
func (s *Player) Toggled() bool {
	asked := s.toggled
	s.toggled = false
	return asked
}

// MuteToggled reports that the sound was asked to be silenced or restored,
// once, and consumes it.
//
// Muted is a prop for the reason Playing is: whether a device is actually
// silent is something only whatever owns the audio can answer.
func (s *Player) MuteToggled() bool {
	asked := s.muted
	s.muted = false
	return asked
}

// Sought reports the position the scrubber was let go at, once, and consumes
// it.
//
// It is reported on release and not while the finger is moving. A seek is not
// free -- it asks a decoder to find a keyframe and refill its buffers -- and a
// drag across a two-hour film produces one of those per frame, sixty times a
// second. Decoders answer that by dropping requests or by serialising them, so
// the media stutters for the whole drag and lands wherever the race left it.
// Reporting once asks for exactly one seek, at the position the person chose,
// and the thumb under their finger is the feedback that would otherwise have to
// come from the media.
//
// A press and release on the track without a drag is the same intent and is
// reported the same way: a tap halfway along means go halfway along.
func (s *Player) Sought() (time.Duration, bool) {
	asked := s.hasSought
	s.hasSought = false
	return s.sought, asked
}

// VolumeSet reports a new volume between 0 and 1, once, and consumes it.
//
// Unlike a seek this is reported continuously, every frame the drag moves it,
// and the difference is what the two cost to obey. Volume is a number a mixer
// multiplies by, so applying it a hundred times is a hundred multiplications
// and the sound follows the finger, which is what a person expects of a volume
// control. Applying a seek a hundred times is a hundred seeks.
func (s *Player) VolumeSet() (float32, bool) {
	asked := s.hasVolume
	s.hasVolume = false
	return s.volume, asked
}

// SpeedSet reports the rate that was asked for, once, and consumes it.
func (s *Player) SpeedSet() (float32, bool) {
	asked := s.hasSpeed
	s.hasSpeed = false
	return s.speed, asked
}

// PlayerProps is the transport for a piece of audio or video.
//
// Every field is an answer about media this control cannot see. Position and
// Duration are where the media is and how long it is, and they arrive fresh
// each frame from whatever is playing it; Playing, Muted, Volume and Speed are
// the settings that are actually in effect rather than the ones somebody asked
// for. Drawing from what is in effect is what makes a failed request visible:
// the button stays on pause until the media really starts.
type PlayerProps struct {
	// Title is the name of what is playing, drawn above the transport. Empty
	// draws no line at all rather than an empty one, so a transport under a
	// video that is already titled on screen takes no extra height.
	Title string
	// Position is how far into the media it is. A position past Duration is
	// drawn as the end: media that reports one frame too many is common, and a
	// clock reading past its own total reads as a bug in the clock.
	Position time.Duration
	// Duration is how long the media is. Zero means it is not known yet --
	// a stream, or a file whose header has not been read -- and it is the
	// denominator of every ratio here, so every one of them is guarded.
	Duration time.Duration
	// Playing is whether the media is running, which decides whether the
	// control draws a play mark or a pause mark.
	Playing bool
	// Volume is the level in effect, between 0 and 1. Anything outside is
	// clamped by the scrubber that draws it.
	Volume float32
	// Muted is whether the sound is silenced. It is separate from a volume of
	// zero because unmuting has to restore a level, and a control that
	// expressed mute by writing zero would have thrown that level away.
	Muted bool
	// Speed is the rate in effect. Zero is a caller that never set one and is
	// drawn as normal, because a rate of zero is a stopped clock rather than a
	// speed.
	Speed float32
	// ShowSpeed draws the control that cycles the rate. It is off by default
	// because most media is played at one speed and a button nobody presses is
	// width taken from the scrubber, which is the control everybody uses.
	ShowSpeed bool
	// Compact drops the title, the volume slider and the rate, leaving the
	// transport that fits in a row of a list. The two it drops are the two a
	// person sets once and leaves.
	Compact bool
	// Disabled draws the whole transport as unavailable and stops it
	// answering.
	Disabled bool
}

// normalSpeed is the rate media is recorded at.
const normalSpeed float32 = 1

// playbackSpeeds is every rate the control offers, in the order it cycles
// through them.
//
// A closed set rather than a list on the props, for two reasons that are really
// one. Normal has to be in it, or a person who pressed once has no way back to
// the speed the thing was recorded at -- and a caller-supplied list cannot be
// made to contain it. And the widest label the button will ever draw is
// knowable only because the set is closed, which is what lets the row reserve a
// width for it instead of shifting every time somebody presses.
var playbackSpeeds = [...]float32{0.5, 0.75, normalSpeed, 1.25, 1.5, 2}

// Layout draws the transport and returns the room it took.
func (p PlayerProps) Layout(c ayra.Context, state *Player) ayra.Dimensions {
	if p.Disabled {
		c.Context = c.Context.Disabled()
	}

	// The presses are read before anything is drawn so that what a press asks
	// for is recorded in the frame the press landed in. Read afterwards, an
	// intent waits for the next frame, and on a transport that is a button that
	// feels like it missed.
	if state.play.Clicked(c) {
		state.toggled = true
	}
	if state.mute.Clicked(c) {
		state.muted = true
	}
	if p.showsRate() && state.rate.Clicked(c) {
		state.speed, state.hasSpeed = nextSpeed(p.Speed), true
	}

	c.Constraints.Min = image.Point{}

	if p.Compact || p.Title == "" {
		return p.transport(c, state)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			ink := c.Theme.Colours.Foreground
			if p.Disabled {
				ink = fade(ink)
			}
			return drawText(c.With(gtx), p.Title, unit.Sp(c.Theme.Type.Body), ink, 1, text.Start, semibold())
		}),
		layout.Rigid(layout.Spacer{Height: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.transport(c.With(gtx), state)
		}),
	)
}

// Lengths the transport is built from, in points.
const (
	// playerGap is the space between one control and the next.
	playerGap unit.Dp = 8
	// playerTarget is the side of a control a finger has to hit. It is square
	// so that the play mark and the pause mark, which are different shapes,
	// take the same room -- otherwise everything to the right of the button
	// moves when the media starts.
	playerTarget unit.Dp = 32
	// playerMark is the side the drawn mark is fitted into, centred in the
	// target.
	playerMark unit.Dp = 18
	// playerVolume is how wide the volume slider is. Fixed rather than shared
	// with the scrubber, because a volume control that grew with the window
	// would take room from the one that is actually aimed at.
	playerVolume unit.Dp = 72
	// playerTrack is the narrowest a scrubber may be and still be worth
	// dragging.
	playerTrack unit.Dp = 48
)

// transport draws the row: the play control, the clock either side of the
// scrubber, and the sound.
//
// The scrubber is laid out first, before the row it sits in, and then replayed
// into place. That is not an optimisation. A flex lays its rigid children out
// before its flexed one, so a scrubber written as the flexed child would be
// processed after the clock that has to read it, and the elapsed column would
// draw the position a finger was at one frame ago -- a number lagging under the
// hand that is moving it. Laying it out first and adding the recorded drawing
// as a rigid child puts the two in the order the reading needs.
func (p PlayerProps) transport(c ayra.Context, state *Player) ayra.Dimensions {
	gap := c.Dp(playerGap)
	target := c.Dp(playerTarget)
	column := p.timeColumn(c)

	compact := p.Compact
	rate := 0
	if !compact && p.showsRate() {
		rate = p.speedColumn(c)
	}

	// Everything that is not the scrubber, added up. The width cannot come
	// from the constraints: inside a flex child Max.X is the whole of the rest
	// of the row, which would draw a track over the controls beside it, and
	// Min.X is zero, which would draw no track at all. Both have been written
	// in this package. What the siblings took is only knowable by adding them
	// up, and the two whose width is text -- the clock and the rate -- are
	// measured rather than guessed.
	fixed := func(compact bool, rate int) int {
		width := target + gap + column + gap + gap + column + gap + target
		if !compact {
			width += gap + playerVolumeWidth(c)
			if rate > 0 {
				width += gap + rate
			}
		}
		return width
	}

	track := c.Constraints.Max.X - fixed(compact, rate)
	if !compact && track < c.Dp(playerTrack) {
		// Too narrow for the full transport. The compact one is drawn rather
		// than squeezing the track to nothing, because a scrubber a few pixels
		// wide is not a control anybody can aim at, and the volume slider
		// drawn on top of the remaining time is what happens if the row is
		// simply allowed to overflow.
		compact, rate = true, 0
		track = c.Constraints.Max.X - fixed(compact, rate)
	}
	track = max(track, c.Dp(playerTrack))

	scrubbing := c
	scrubbing.Constraints.Min = image.Point{}
	scrubbing.Constraints.Max.X = track

	measure := op.Record(c.Ops)
	scrubber := p.scrubber(scrubbing, state)
	drawn := measure.Stop()

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.control(c.With(gtx), &state.play, p.transportMark())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.clock(c.With(gtx), column, formatDuration(p.elapsed(state)))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			drawn.Add(gtx.Ops)
			return scrubber
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.clock(c.With(gtx), column, p.remaining(state))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.control(c.With(gtx), &state.mute, p.soundMark())
		}),
	}

	if !compact {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.volume(c.With(gtx), state)
		}))
		if rate > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.speed(c.With(gtx), state, rate)
			}))
		}
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gap}.Layout(c.Context, children...)
}

// showsRate reports whether the rate control is drawn.
func (p PlayerProps) showsRate() bool { return p.ShowSpeed && !p.Compact }

// playerVolumeWidth is the volume slider's width in pixels.
func playerVolumeWidth(c ayra.Context) int { return c.Dp(playerVolume) }

// mark names the shape a transport control carries.
//
// It is a value rather than the drawing function itself, so that which shape a
// state resolves to is something that can be compared. Two functions cannot be:
// a control that drew the play triangle over running media would be a defect
// nothing could state, because the only way to ask what it drew would be to
// look at it.
type mark uint8

const (
	// markPlay starts the media. Zero, because a transport that has not been
	// told anything is a transport over media that is not running.
	markPlay mark = iota
	// markPause stops it.
	markPause
	// markSound is the speaker with its waves.
	markSound
	// markSilent is the speaker with a cross through them.
	markSilent
)

// String names the mark, for a diagnostic screen and for a test failure.
func (m mark) String() string {
	switch m {
	case markPause:
		return "pause"
	case markSound:
		return "sound"
	case markSilent:
		return "silent"
	}
	return "play"
}

// markShapes is the drawing each mark resolves to.
//
// A table rather than a switch, so that which shape a mark answers with is
// something that can be read rather than only looked at. A switch arm sending
// two marks to one drawing compiles, draws a picture, and can be found only by
// rasterising the frame and comparing it with a picture somebody approved.
var markShapes = [...]func(ayra.Context, int, color.NRGBA){
	markPlay:   drawPlay,
	markPause:  drawPause,
	markSound:  drawSpeaker,
	markSilent: drawSpeakerSilent,
}

// draw puts the shape on screen, fitted to a square of the given side.
func (m mark) draw(c ayra.Context, side int, ink color.NRGBA) {
	if int(m) >= len(markShapes) {
		m = markPlay
	}
	markShapes[m](c, side, ink)
}

// transportMark answers the shape on the play control.
//
// It reads Playing, which is what the media is doing, rather than anything this
// control remembers about what was asked for. A transport that drew the mark
// for the press it had just reported would show a pause bar over a file that
// failed to open.
func (p PlayerProps) transportMark() mark {
	if p.Playing {
		return markPause
	}
	return markPlay
}

// soundMark answers the shape on the sound control.
//
// A volume of nought is drawn as silenced as well as an explicit mute, because
// they sound the same and a speaker with waves over silence is a control saying
// something the device is not doing.
func (p PlayerProps) soundMark() mark {
	if p.Muted || p.Volume <= 0 {
		return markSilent
	}
	return markSound
}

// scrubber draws the track and keeps the thumb under whichever of the two has
// the better claim on it.
//
// The order is the point. Before the slider reads its pointer, the caller's
// position is written in -- but only if no finger is on it. Afterwards, a drag
// that has just ended hands back the position it ended at.
//
// A scrubber for media of unknown length is drawn as unavailable. There is
// nowhere to drag to when nobody knows where the end is, and a track that
// accepted a drag and answered zero would report a seek to the beginning every
// time somebody touched it.
func (p PlayerProps) scrubber(c ayra.Context, state *Player) ayra.Dimensions {
	written := state.follow(p.Position, p.Duration, state.scrub.value.Dragging())
	dims := SliderProps{Disabled: p.Disabled || p.Duration <= 0}.Layout(c, &state.scrub)
	state.settle(p.Duration, state.scrub.value.Dragging(), written)
	return dims
}

// follow writes the caller's position into the scrubber unless a finger is on
// it, and answers the value it left there.
//
// This is the failure the whole control is shaped around. The caller supplies a
// new Position every frame while the media runs. Written in unconditionally, it
// lands on top of the value the drag has just set, and the thumb snaps back
// under the hand between one frame and the next -- thirty times a second, for
// as long as somebody holds it. It does not read as a glitch: it reads as a
// scrubber that refuses to go anywhere except where the media already is, which
// is the one place nobody needs to drag it to.
//
// The value it answers is what [Player.settle] compares against, so that a
// press and release inside a single frame -- a tap on the track, which never
// registers as a drag at all -- is still seen as an ask.
func (s *Player) follow(position, duration time.Duration, dragging bool) float32 {
	if !dragging {
		s.scrub.SetValue(fraction(position, duration))
	}
	return s.scrub.Value()
}

// settle records what the scrubber was asked for, after its pointer has been
// read.
//
// While the finger is down it keeps the position being dragged to, which is
// what the elapsed column reads and what a caller drawing a preview asks for.
// The frame the finger comes off, that position becomes the seek -- once.
func (s *Player) settle(duration time.Duration, dragging bool, written float32) {
	if dragging {
		s.scrubbing = true
		s.scrubbedTo = at(s.scrub.Value(), duration)
		return
	}

	if s.scrubbing {
		s.scrubbing = false
		s.sought, s.hasSought = s.scrubbedTo, true
		return
	}

	if s.scrub.Value() != written {
		// The value moved without a drag ever being in progress, which is a
		// press and a release inside one frame: a tap on the track. It is the
		// same ask as a drag that went nowhere, and dropping it would make the
		// commonest way of jumping through a recording do nothing.
		s.sought, s.hasSought = at(s.scrub.Value(), duration), true
	}
}

// volume draws the sound level and reports every move of it.
//
// It is the same shape as the scrubber and reports on a different schedule, for
// the reason [Player.VolumeSet] gives. The comparison against the value written
// in is what keeps it from reporting every frame: with no finger on it the
// prop is written back each frame and nothing has moved, so there is nothing to
// say -- including when the caller ignores what was said, which is the case
// that turns a naive implementation into an endless stream of the same event.
func (p PlayerProps) volume(c ayra.Context, state *Player) ayra.Dimensions {
	c.Constraints.Min = image.Point{}
	c.Constraints.Max.X = min(playerVolumeWidth(c), c.Constraints.Max.X)

	if !state.sound.value.Dragging() {
		level := p.Volume
		if p.Muted {
			// Muted draws as silent without the caller's level being lost: the
			// prop still says what to go back to, and this only says what to
			// draw.
			level = 0
		}
		state.sound.SetValue(level)
	}
	written := state.sound.Value()

	dims := SliderProps{Disabled: p.Disabled}.Layout(c, &state.sound)

	if state.sound.Value() != written {
		state.volume, state.hasVolume = state.sound.Value(), true
	}
	return dims
}

// speed draws the rate control at the width the widest rate needs.
func (p PlayerProps) speed(c ayra.Context, state *Player, width int) ayra.Dimensions {
	ink := c.Theme.Colours.MutedForeground
	if p.Disabled {
		ink = fade(ink)
	}

	label := speedLabel(p.Speed)
	return state.rate.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		inner.Constraints.Min = image.Pt(width, 0)
		inner.Constraints.Max.X = max(width, gtx.Constraints.Max.X)
		return drawText(inner, label, unit.Sp(inner.Theme.Type.Small), ink, 1, text.Middle, mono())
	})
}

// control draws one transport button: a round target with a mark centred in it.
func (p PlayerProps) control(c ayra.Context, state *Button, shape mark) ayra.Dimensions {
	side := c.Dp(playerTarget)
	glyph := c.Dp(playerMark)

	ink := c.Theme.Colours.Foreground
	wash := color.NRGBA{}
	if p.Disabled {
		ink = fade(ink)
	} else if state.Pressed() || state.Hovered() {
		wash = press(wash, state.Pressed())
	}

	return state.click.Layout(c.Context, func(gtx layout.Context) layout.Dimensions {
		inner := c.With(gtx)
		target := image.Rectangle{Max: image.Pt(side, side)}

		if wash.A > 0 {
			paint.FillShape(inner.Ops, wash, clip.UniformRRect(target, side/2).Op(inner.Ops))
		}

		// Offset rather than a clip: a clip says where drawing may land and
		// moves nothing, so the mark would be drawn at the corner and trimmed
		// to whatever of it fell inside.
		offset := op.Offset(image.Pt((side-glyph)/2, (side-glyph)/2)).Push(inner.Ops)
		shape.draw(inner, glyph, ink)
		offset.Pop()

		return layout.Dimensions{Size: target.Max}
	})
}

// clock draws one time in a column of a fixed width.
func (p PlayerProps) clock(c ayra.Context, width int, label string) ayra.Dimensions {
	ink := c.Theme.Colours.MutedForeground
	if p.Disabled {
		ink = fade(ink)
	}

	c.Constraints.Min = image.Pt(width, 0)
	c.Constraints.Max.X = max(width, c.Constraints.Max.X)
	return drawText(c, label, unit.Sp(c.Theme.Type.Mono), ink, 1, text.Middle, mono())
}

// elapsed is the time the left-hand column reads.
//
// While a finger is on the scrubber it reads the position being dragged to
// rather than the media's own, because that is the question the person is
// asking: the number is the only thing that says where letting go will land.
func (p PlayerProps) elapsed(state *Player) time.Duration {
	if to, scrubbing := state.Scrubbing(); scrubbing {
		return to
	}
	if p.Position < 0 {
		return 0
	}
	if p.Duration > 0 && p.Position > p.Duration {
		return p.Duration
	}
	return p.Position
}

// remaining is the time the right-hand column reads.
//
// It is signed -- "-1:23" rather than "1:23" -- because the two columns are the
// same size, in the same face, a scrubber apart, and nothing else distinguishes
// a number counting up from one counting down. A reader who cannot tell them
// apart reads the remaining column as the length of the media, which is the
// same number only at the very start.
//
// Media of unknown length has no remaining time, and the column is left blank.
// A figure there would be a claim about a length nobody has yet.
func (p PlayerProps) remaining(state *Player) string {
	if p.Duration <= 0 {
		return ""
	}
	return formatDuration(p.elapsed(state) - p.Duration)
}

// timeColumn is the width both clocks reserve, measured rather than assumed.
//
// The two sit either side of the scrubber and their text changes every second.
// Left to size themselves, the row is laid out again on every tick: "9:59"
// becomes "10:00", the scrubber loses a digit's width, and the thumb somebody
// is aiming at slides sideways once a second for the length of the recording.
//
// It has to be measured because how wide "1:23:45" is depends on the face, the
// size and the density, and the only thing that knows all three is the shaper.
// So the widest string the media can produce is laid out into a recording that
// is then thrown away, and its width is the column.
//
// The widest string comes from the duration, because the clock cannot exceed
// it. When the duration is not known the position is used instead, which is the
// only figure there is -- and the column then does grow the first time the
// media passes ten minutes. There is nothing better to go on for a stream.
func (p PlayerProps) timeColumn(c ayra.Context) int {
	longest := p.Duration
	if longest <= 0 {
		longest = p.Position
	}
	if longest < 0 {
		longest = 0
	}

	widest := formatDuration(longest)
	if counted := formatDuration(-longest); len(counted) > len(widest) {
		widest = counted
	}

	c.Constraints.Min = image.Point{}

	measure := op.Record(c.Ops)
	dims := drawText(c, widest, unit.Sp(c.Theme.Type.Mono), c.Theme.Colours.MutedForeground, 1, text.Middle, mono())
	measure.Stop()

	return dims.Size.X
}

// speedColumn is the width the rate control reserves.
//
// Measured across the whole closed set, so the button is as wide as "1.25x"
// even while it reads "2x". A button that resized on press would take room from
// the scrubber and move the clock beside it, and it would do so as a result of
// somebody pressing an unrelated control.
func (p PlayerProps) speedColumn(c ayra.Context) int {
	c.Constraints.Min = image.Point{}

	widest := 0
	measure := op.Record(c.Ops)
	for _, rate := range playbackSpeeds {
		dims := drawText(c, speedLabel(rate), unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Middle, mono())
		widest = max(widest, dims.Size.X)
	}
	measure.Stop()

	return widest
}

// fraction is where a position sits in a duration, between 0 and 1.
//
// The duration is the denominator of every ratio in this control, and it is
// zero for as long as nobody has read the length -- a stream, or a file whose
// header has not arrived. Answering zero is what keeps the thumb at the start
// instead of somewhere off the end of a track divided by nothing.
//
// It clamps, because a position past the duration is ordinary: media reports
// one frame too many all the time, and an unclamped ratio draws a thumb past
// the end of its own track, over whatever is beside it.
func fraction(position, duration time.Duration) float32 {
	if duration <= 0 {
		return 0
	}
	return min(max(float32(position)/float32(duration), 0), 1)
}

// at is the position a fraction of a duration names.
//
// It multiplies in float64 rather than float32 because a float32 carries
// twenty-four bits of mantissa and an hour in nanoseconds needs forty-two: the
// narrower type would round a seek in a long recording to the nearest
// half-millisecond for no reason at all.
func at(f float32, duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}
	return time.Duration(float64(min(max(f, 0), 1)) * float64(duration))
}

// formatDuration writes a duration the way a clock on a transport reads it.
//
// Under an hour it is m:ss, and the minutes carry no leading zero: "3:07", not
// "03:07". The padded form puts a column on screen that is zero for almost
// everything anybody plays, and a three-minute recording written "03:07" reads
// at a glance as though it might be thirty. The seconds are padded, because two
// digits is what makes them read as seconds at all -- "3:7" is not a time.
//
// An hour or more it is h:mm:ss, and the minutes are padded there, because they
// have become a middle field and a middle field with a variable width is not a
// column.
//
// It truncates rather than rounds. A clock that rounded would show "0:01"
// before the first second had passed and would show a time one second past the
// end of the media as it finished, which is the moment somebody is most likely
// to be looking at it.
//
// A negative duration -- which is what a caller subtracting a position from a
// length it has not learned yet produces, and what the remaining column is --
// carries one sign, at the front, and the magnitude is written normally. There
// is never a sign in the middle. A negative smaller than a second writes as
// "0:00" rather than "-0:00", because there is nothing left to count down.
func formatDuration(d time.Duration) string {
	// The seconds are taken before anything is negated. Negating the duration
	// itself would overflow at the most negative one it can hold -- the
	// negation of that value is itself, still negative -- and every sign test
	// after it would then be wrong. Dividing first puts the figure nine orders
	// of magnitude inside the range where negating is safe.
	seconds := int64(d / time.Second)

	sign := ""
	if seconds < 0 {
		sign, seconds = "-", -seconds
	}

	hours := seconds / 3600
	minutes := seconds % 3600 / 60
	rest := seconds % 60

	if hours > 0 {
		return sign + strconv.FormatInt(hours, 10) + ":" + pad(minutes) + ":" + pad(rest)
	}
	return sign + strconv.FormatInt(minutes, 10) + ":" + pad(rest)
}

// pad writes a figure in two digits, which is what makes it read as a field of
// a time rather than as a number.
func pad(v int64) string {
	if v < 10 {
		return "0" + strconv.FormatInt(v, 10)
	}
	return strconv.FormatInt(v, 10)
}

// speedLabel writes a rate the way the button reads it.
//
// A rate that is not positive is drawn as normal, because zero is the value a
// caller that never set one leaves behind and a stopped clock is not a playback
// speed. A positive rate outside the offered set is drawn as it is, because
// lying about what is in effect is worse than showing a figure the button
// cannot cycle back to.
func speedLabel(speed float32) string {
	if speed <= 0 {
		speed = normalSpeed
	}
	return strconv.FormatFloat(float64(speed), 'f', -1, 32) + "x"
}

// nextSpeed is the rate one press past the current one.
//
// A rate the set does not contain -- including the zero a caller that never set
// one leaves -- advances from normal rather than answering normal. Answering
// normal would be a first press that changed nothing, on a control already
// reading "1x", which is indistinguishable from a button that does not work.
func nextSpeed(current float32) float32 {
	index := speedIndex(current)
	if index < 0 {
		index = speedIndex(normalSpeed)
	}
	return playbackSpeeds[(index+1)%len(playbackSpeeds)]
}

// speedIndex is where a rate sits in the offered set, or -1.
func speedIndex(speed float32) int {
	for index, rate := range playbackSpeeds {
		if rate == speed {
			return index
		}
	}
	return -1
}

// drawPlay draws the mark on the control that starts the media: a triangle
// pointing the way the media runs.
//
// A path and not a glyph, for the reason the rating's star gives: a glyph comes
// from whichever font the platform happens to carry, and on one that is missing
// it the transport draws a row of hollow boxes. A person can still tell a
// rating of three from a rating of four that way. Nobody can tell play from
// pause.
func drawPlay(c ayra.Context, side int, ink color.NRGBA) {
	w := float32(side)

	var shape clip.Path
	shape.Begin(c.Ops)
	shape.MoveTo(f32.Pt(w*0.32, w*0.20))
	shape.LineTo(f32.Pt(w*0.32, w*0.80))
	shape.LineTo(f32.Pt(w*0.82, w*0.50))
	shape.Close()

	paint.FillShape(c.Ops, ink, clip.Outline{Path: shape.End()}.Op())
}

// drawPause draws the mark on the control that stops the media: two bars.
func drawPause(c ayra.Context, side int, ink color.NRGBA) {
	w := float32(side)
	bar := max(int(w*0.18), 1)
	top, bottom := int(w*0.20), int(w*0.80)
	radius := max(bar/3, 1)

	left := image.Rect(int(w*0.28), top, int(w*0.28)+bar, bottom)
	right := image.Rect(int(w*0.72)-bar, top, int(w*0.72), bottom)

	paint.FillShape(c.Ops, ink, clip.UniformRRect(left, radius).Op(c.Ops))
	paint.FillShape(c.Ops, ink, clip.UniformRRect(right, radius).Op(c.Ops))
}

// drawSpeaker draws the mark on a sound control that is not silenced: a cone
// with two waves coming off it.
func drawSpeaker(c ayra.Context, side int, ink color.NRGBA) {
	w := float32(side)
	speakerCone(c, w, ink)
	thickness := max(w/11, 1)

	var near clip.Path
	near.Begin(c.Ops)
	near.MoveTo(f32.Pt(w*0.60, w*0.34))
	near.QuadTo(f32.Pt(w*0.74, w*0.50), f32.Pt(w*0.60, w*0.66))
	paint.FillShape(c.Ops, ink, clip.Stroke{Path: near.End(), Width: thickness}.Op())

	var far clip.Path
	far.Begin(c.Ops)
	far.MoveTo(f32.Pt(w*0.74, w*0.22))
	far.QuadTo(f32.Pt(w*0.96, w*0.50), f32.Pt(w*0.74, w*0.78))
	paint.FillShape(c.Ops, ink, clip.Stroke{Path: far.End(), Width: thickness}.Op())
}

// drawSpeakerSilent draws the mark on a sound control that is silenced: the
// same cone with a cross where the waves were.
//
// A cross rather than the cone on its own. A mark that means silence by the
// absence of a mark is indistinguishable from a mark that failed to draw, and
// the two states of this button have to be told apart by eye at the size of a
// fingertip.
func drawSpeakerSilent(c ayra.Context, side int, ink color.NRGBA) {
	w := float32(side)
	speakerCone(c, w, ink)
	thickness := max(w/11, 1)

	var first clip.Path
	first.Begin(c.Ops)
	first.MoveTo(f32.Pt(w*0.62, w*0.34))
	first.LineTo(f32.Pt(w*0.90, w*0.66))
	paint.FillShape(c.Ops, ink, clip.Stroke{Path: first.End(), Width: thickness}.Op())

	var second clip.Path
	second.Begin(c.Ops)
	second.MoveTo(f32.Pt(w*0.90, w*0.34))
	second.LineTo(f32.Pt(w*0.62, w*0.66))
	paint.FillShape(c.Ops, ink, clip.Stroke{Path: second.End(), Width: thickness}.Op())
}

// speakerCone draws the body both sound marks share.
func speakerCone(c ayra.Context, w float32, ink color.NRGBA) {
	var cone clip.Path
	cone.Begin(c.Ops)
	cone.MoveTo(f32.Pt(w*0.10, w*0.36))
	cone.LineTo(f32.Pt(w*0.28, w*0.36))
	cone.LineTo(f32.Pt(w*0.48, w*0.16))
	cone.LineTo(f32.Pt(w*0.48, w*0.84))
	cone.LineTo(f32.Pt(w*0.28, w*0.64))
	cone.LineTo(f32.Pt(w*0.10, w*0.64))
	cone.Close()
	paint.FillShape(c.Ops, ink, clip.Outline{Path: cone.End()}.Op())
}
