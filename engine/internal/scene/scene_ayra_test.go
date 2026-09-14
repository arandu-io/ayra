package scene_test

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/arandu-io/ayra/engine/internal/byteslice"
	"github.com/arandu-io/ayra/engine/internal/f32"
	"github.com/arandu-io/ayra/engine/internal/scene"
)

// This package packs floats into an array of words and reads them back, and
// nothing about that is checked by the compiler: two fields written in the
// wrong order are the same type in the same array, so the mistake builds, runs
// and draws a shape nobody asked for. The round trip is what catches it, and it
// only catches it when every coordinate is a different number -- a test that
// encodes the same value twice cannot tell the two apart.
//
// The name of this file keeps it out of the way of a test file the original may
// gain later.

var (
	a = f32.Pt(1, 2)
	b = f32.Pt(3, 4)
	c = f32.Pt(5, 6)
	d = f32.Pt(7, 8)
)

// bbox is a clip rectangle with four distinct sides, for the same reason.
var bbox = f32.Rect(9, 10, 11, 12)

func TestALineDecodesToThePointsItWasGiven(t *testing.T) {
	from, to := scene.DecodeLine(scene.Line(a, b))
	if from != a || to != b {
		t.Errorf("Line(%v, %v) decoded to (%v, %v)", a, b, from, to)
	}
}

func TestAGapDecodesToThePointsItWasGiven(t *testing.T) {
	from, to := scene.DecodeGap(scene.Gap(a, b))
	if from != a || to != b {
		t.Errorf("Gap(%v, %v) decoded to (%v, %v)", a, b, from, to)
	}
}

func TestAQuadDecodesToThePointsItWasGiven(t *testing.T) {
	from, ctrl, to := scene.DecodeQuad(scene.Quad(a, b, c))
	if from != a || ctrl != b || to != c {
		t.Errorf("Quad(%v, %v, %v) decoded to (%v, %v, %v)", a, b, c, from, ctrl, to)
	}
}

func TestACubicDecodesToThePointsItWasGiven(t *testing.T) {
	from, ctrl0, ctrl1, to := scene.DecodeCubic(scene.Cubic(a, b, c, d))
	if from != a || ctrl0 != b || ctrl1 != c || to != d {
		t.Errorf("Cubic(%v, %v, %v, %v) decoded to (%v, %v, %v, %v)", a, b, c, d, from, ctrl0, ctrl1, to)
	}
}

// TestEveryCommandReportsItsOwnOp is what the readers of the stream switch on.
// A command that reports another op is decoded by the wrong reader, which reads
// the right words as the wrong thing.
func TestEveryCommandReportsItsOwnOp(t *testing.T) {
	for _, test := range []struct {
		command scene.Command
		want    scene.Op
	}{
		{scene.Command{}, scene.OpNop},
		{scene.Line(a, b), scene.OpLine},
		{scene.Quad(a, b, c), scene.OpQuad},
		{scene.Cubic(a, b, c, d), scene.OpCubic},
		{scene.FillColor(color.RGBA{R: 1, G: 2, B: 3, A: 4}), scene.OpFillColor},
		{scene.SetLineWidth(2.5), scene.OpLineWidth},
		{scene.Transform(f32.NewAffine2D(1, 2, 3, 4, 5, 6)), scene.OpTransform},
		{scene.BeginClip(bbox), scene.OpBeginClip},
		{scene.EndClip(bbox), scene.OpEndClip},
		{scene.FillImage(1, image.Pt(2, 3)), scene.OpFillImage},
		{scene.SetFillMode(scene.FillModeStroke), scene.OpSetFillMode},
		{scene.Gap(a, b), scene.OpGap},
	} {
		if got := test.command.Op(); got != test.want {
			t.Errorf("%v reports op %d, and %d was encoded", test.command, got, test.want)
		}
	}
}

// TestTheOpNumbersAreTheOnesTheStreamCarries pins the wire contract.
//
// The number is what goes into the stream, so the order of these constants is
// not an implementation detail: an op inserted in the middle renumbers every one
// after it, and anything reading a stream written before the change decodes
// every command as the wrong one.
func TestTheOpNumbersAreTheOnesTheStreamCarries(t *testing.T) {
	for want, op := range []scene.Op{
		scene.OpNop, scene.OpLine, scene.OpQuad, scene.OpCubic, scene.OpFillColor, scene.OpLineWidth,
		scene.OpTransform, scene.OpBeginClip, scene.OpEndClip, scene.OpFillImage, scene.OpSetFillMode, scene.OpGap,
	} {
		if int(op) != want {
			t.Errorf("op %v is %d and the stream carries it as %d", op, op, want)
		}
	}
}

// TestADecoderRefusesACommandOfAnotherOp fixes the guard at the top of each
// decoder.
//
// Without it a quad handed to the line decoder answers two of its three points
// and says nothing, because the words are there and every one of them is a
// float. The complaint is the only thing that tells a caller it read the wrong
// shape.
func TestADecoderRefusesACommandOfAnotherOp(t *testing.T) {
	for _, test := range []struct {
		name   string
		decode func()
	}{
		{"a line read as a gap", func() { scene.DecodeGap(scene.Line(a, b)) }},
		{"a gap read as a line", func() { scene.DecodeLine(scene.Gap(a, b)) }},
		{"a quad read as a line", func() { scene.DecodeLine(scene.Quad(a, b, c)) }},
		{"a cubic read as a quad", func() { scene.DecodeQuad(scene.Cubic(a, b, c, d)) }},
		{"a quad read as a cubic", func() { scene.DecodeCubic(scene.Quad(a, b, c)) }},
		{"a nop read as a line", func() { scene.DecodeLine(scene.Command{}) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("the command decoded without complaint, and the points came from the wrong words")
				}
			}()
			test.decode()
		})
	}
}

// TestASequenceComesBackInTheOrderItWentIn is the second half of the round
// trip: a command survives the copy into the byte stream and out of it, and a
// run of them keeps its order.
//
// Order is the property worth stating, because a path is a sequence of segments
// joined end to end. Two of them read back the other way round still decode to
// valid points, and the shape they draw is a different shape.
func TestASequenceComesBackInTheOrderItWentIn(t *testing.T) {
	want := []scene.Command{
		scene.Line(a, b),
		scene.Quad(b, c, d),
		scene.Gap(d, a),
		scene.Cubic(a, b, c, d),
		scene.Line(c, d),
	}

	stream := make([]byte, len(want)*scene.CommandSize)
	for at, command := range want {
		encode(stream[at*scene.CommandSize:], command)
	}

	got := readAll(stream)
	if len(got) != len(want) {
		t.Fatalf("%d commands were written and %d read back", len(want), len(got))
	}
	for at := range want {
		if got[at] != want[at] {
			t.Errorf("command %d read back as %v, and %v was written", at, got[at], want[at])
		}
	}
}

// TestAnEmptyStreamYieldsNoCommands is the case a path with nothing in it
// produces, and it has to end rather than read past what is there.
//
// The short stream is the same question asked of a partial write: fewer bytes
// than one command is not a command, and treating it as one reads whatever
// follows the buffer.
func TestAnEmptyStreamYieldsNoCommands(t *testing.T) {
	for _, stream := range [][]byte{nil, {}, make([]byte, scene.CommandSize-1)} {
		if got := readAll(stream); len(got) != 0 {
			t.Errorf("%d bytes yielded %d commands, and none was written", len(stream), len(got))
		}
	}
}

// TestTheZeroCommandIsANop is what an unwritten command has to be.
//
// A buffer that was allocated and not filled decodes as this one, and the only
// safe reading of it is "do nothing". Any other op there would be a draw nobody
// asked for, taken from words nobody wrote.
func TestTheZeroCommandIsANop(t *testing.T) {
	var command scene.Command
	if got := command.Op(); got != scene.OpNop {
		t.Errorf("the zero command reports op %d, and a command nobody wrote has to be a nop", got)
	}
	if got := command.String(); got != "nop" {
		t.Errorf("the zero command prints as %q", got)
	}
}

// TestEveryOpPrints keeps the description from being the one thing that stops
// the program.
//
// It is read when something is already wrong -- a trace, a failing test, a
// person looking at a stream that draws the wrong thing -- and a printer that
// gives up on one of the ops takes the diagnosis with it.
func TestEveryOpPrints(t *testing.T) {
	for _, command := range []scene.Command{
		{},
		scene.Line(a, b),
		scene.Gap(a, b),
		scene.Quad(a, b, c),
		scene.Cubic(a, b, c, d),
		scene.FillColor(color.RGBA{R: 1, G: 2, B: 3, A: 4}),
		scene.SetLineWidth(2.5),
		scene.Transform(f32.NewAffine2D(1, 2, 3, 4, 5, 6)),
		scene.BeginClip(bbox),
		scene.EndClip(bbox),
		scene.FillImage(1, image.Pt(2, 3)),
		scene.SetFillMode(scene.FillModeStroke),
	} {
		t.Run(fmt.Sprint(command.Op()), func(t *testing.T) {
			defer func() {
				if state := recover(); state != nil {
					t.Errorf("printing op %d stopped the program: %v", command.Op(), state)
				}
			}()
			if got := command.String(); got == "" {
				t.Errorf("op %d prints as nothing", command.Op())
			}
		})
	}
}

// TestASegmentPrintsThePointsItWasGiven is the round trip taken through the
// description instead of through the decoder, which is what makes a printed
// stream worth reading.
func TestASegmentPrintsThePointsItWasGiven(t *testing.T) {
	for _, test := range []struct {
		command scene.Command
		want    string
	}{
		{scene.Line(a, b), fmt.Sprintf("line(%v, %v)", a, b)},
		{scene.Gap(a, b), fmt.Sprintf("gap(%v, %v)", a, b)},
		{scene.Quad(a, b, c), fmt.Sprintf("quad(%v, %v, %v)", a, b, c)},
		{scene.Cubic(a, b, c, d), fmt.Sprintf("cubic(%v, %v, %v, %v)", a, b, c, d)},
	} {
		if got := test.command.String(); got != test.want {
			t.Errorf("printed as %q, want %q", got, test.want)
		}
	}
}

// TestACommandPrintsItsArgumentsAndNotOnlyItsName covers the commands whose
// description named the op and stopped.
//
// Which command went wrong is half of what a reader of a stream needs. The
// other half is what it said, and for these it was not there to read.
func TestACommandPrintsItsArgumentsAndNotOnlyItsName(t *testing.T) {
	for _, test := range []struct {
		command scene.Command
		want    string
	}{
		{scene.SetLineWidth(2.5), "linewidth 2.5"},
		{scene.FillImage(11, image.Pt(3, -5)), "fillimage 11 at (3,-5)"},
		{scene.SetFillMode(scene.FillModeNonzero), "setfillmode nonzero"},
		{scene.SetFillMode(scene.FillModeStroke), "setfillmode stroke"},
	} {
		if got := test.command.String(); got != test.want {
			t.Errorf("printed as %q, want %q", got, test.want)
		}
	}
}

// TestAnOpThisPackageDoesNotDefineStillPrints keeps the description total.
//
// A command with an op from nowhere is exactly the case somebody is printing a
// stream to look into: a stride that slipped, a buffer that was not filled, a
// writer and a reader disagreeing about a number. Stopping the program on it
// takes away the one thing that would have said so.
func TestAnOpThisPackageDoesNotDefineStillPrints(t *testing.T) {
	defer func() {
		if state := recover(); state != nil {
			t.Fatalf("printing an unknown op stopped the program: %v", state)
		}
	}()
	if got, want := (scene.Command{0: 99}).String(), "op 99"; got != want {
		t.Errorf("an unknown op printed as %q, want %q", got, want)
	}
}

// TestATransformPrintsTheMatrixItWasGiven is the only round trip a transform
// has, because nothing in this module decodes one. The description is the only
// thing that says whether the six elements went into the words they were meant
// to.
//
// The matrix is stored a column at a time and handed over a row at a time, and
// the two middle elements are the ones that cross. Swapped, an upright drawing
// is unchanged and every rotation is mirrored.
func TestATransformPrintsTheMatrixItWasGiven(t *testing.T) {
	matrix := f32.NewAffine2D(1, 2, 3, 4, 5, 6)
	want := fmt.Sprintf("transform (%v)", matrix)
	if got := scene.Transform(matrix).String(); got != want {
		t.Errorf("printed as %q, want %q", got, want)
	}
}

// TestAClipPrintsTheBoundsItWasGiven is the same round trip for the two clip
// commands, whose four words are the near corner of a rectangle and then the
// far one.
func TestAClipPrintsTheBoundsItWasGiven(t *testing.T) {
	if got, want := scene.BeginClip(bbox).String(), fmt.Sprintf("beginclip (%v)", bbox); got != want {
		t.Errorf("printed as %q, want %q", got, want)
	}
	if got, want := scene.EndClip(bbox).String(), fmt.Sprintf("endclip (%v)", bbox); got != want {
		t.Errorf("printed as %q, want %q", got, want)
	}
}

// TestAFillColourKeepsItsChannelsInOrder fixes which byte of the word is which
// channel.
//
// Every channel is a different value here, because the usual mistake is red and
// blue exchanged and the usual test colour is a grey, which is the one colour
// where that mistake is invisible.
func TestAFillColourKeepsItsChannelsInOrder(t *testing.T) {
	got := scene.FillColor(color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x78}).String()
	if want := "fillcolor 0x12345678"; got != want {
		t.Errorf("printed as %q, want %q: red is the top byte and alpha the bottom", got, want)
	}
}

// TestAnImageOffsetSurvivesBeingPackedIntoOneWord is the one command that
// narrows what it is given.
//
// The offset is two signed halves in a single word, x in the low one. A
// negative offset is the ordinary case rather than the edge -- it is what a
// scrolled list hands over -- and a sign dropped in the packing moves the image
// tens of thousands of pixels instead of a few.
func TestAnImageOffsetSurvivesBeingPackedIntoOneWord(t *testing.T) {
	for _, offset := range []image.Point{
		{X: 0, Y: 0},
		{X: 3, Y: -5},
		{X: -7, Y: 9},
		{X: -32768, Y: 32767},
	} {
		command := scene.FillImage(11, offset)
		if got := command[1]; got != 11 {
			t.Errorf("the image index came back as %d, and 11 was encoded", got)
		}
		x := int(int16(uint16(command[2])))
		y := int(int16(uint16(command[2] >> 16)))
		if x != offset.X || y != offset.Y {
			t.Errorf("offset %v packed and unpacked to (%d, %d)", offset, x, y)
		}
	}
}

// TestSetLineWidthCarriesTheWidth checks the one float the command holds.
func TestSetLineWidthCarriesTheWidth(t *testing.T) {
	command := scene.SetLineWidth(2.5)
	if got := math.Float32frombits(command[1]); got != 2.5 {
		t.Errorf("the width came back as %v, and 2.5 was encoded", got)
	}
}

// TestSetFillModeCarriesTheMode checks the one number the command holds, for
// both of the modes there are.
func TestSetFillModeCarriesTheMode(t *testing.T) {
	for _, mode := range []scene.FillMode{scene.FillModeNonzero, scene.FillModeStroke} {
		command := scene.SetFillMode(mode)
		if got := scene.FillMode(command[1]); got != mode {
			t.Errorf("mode %d came back as %d", mode, got)
		}
	}
}

// TestCommandSizeIsTheWholeCommandAndNothingMore is what the readers of the
// stream advance by.
//
// A size that disagrees with the array is an offset that slips by the
// difference on every segment: the first shape is right, and the tenth is
// noise.
func TestCommandSizeIsTheWholeCommandAndNothingMore(t *testing.T) {
	var command scene.Command
	if got, want := scene.CommandSize, len(command)*4; got != want {
		t.Errorf("CommandSize is %d and a command is %d words of four bytes", got, want)
	}
	if got := len(byteslice.Slice(command[:])); got != scene.CommandSize {
		t.Errorf("a command views as %d bytes and CommandSize says %d", got, scene.CommandSize)
	}
}

// TestACommandHoldsTheLongestOp keeps the array from being sized to the op that
// happened to be written first.
//
// A cubic is the long one, at four points beside its op. One word short and its
// last coordinate is written past the end of the command, into the next one.
func TestACommandHoldsTheLongestOp(t *testing.T) {
	if got, want := len(scene.Command{}), 1+4*2; got < want {
		t.Errorf("a command is %d words and a cubic needs %d", got, want)
	}
}

// encode and decode are what the operation list does with a command: it copies
// the words into the byte stream and copies them back out. The test does it
// here rather than through the writer, because what is under test is that a
// command survives that copy -- not that the writer calls it.
func encode(out []byte, command scene.Command) {
	copy(out, byteslice.Slice(command[:]))
}

func decode(in []byte) scene.Command {
	var command scene.Command
	copy(byteslice.Slice(command[:]), in)
	return command
}

// readAll drains a stream the way its readers do: one whole command at a time,
// stopping when what is left is not one.
func readAll(stream []byte) []scene.Command {
	var commands []scene.Command
	for len(stream) >= scene.CommandSize {
		commands = append(commands, decode(stream[:scene.CommandSize]))
		stream = stream[scene.CommandSize:]
	}
	return commands
}
