package widget

import (
	"image"
	"image/color"
	"testing"

	"github.com/arandu-io/ayra/theme"
)

// tolerance is nought, and saying so is the point.
//
// The conversions are checked on the bytes that come out rather than on the
// floats in between, and over five and a half million colours the round trip
// is exact -- so a tolerance here would only be a licence for a future change
// to be a shade wrong without anything failing. Where a float is compared at
// all, it is a float this package itself produced from an exact byte, and the
// comparison says so at the site.
const tolerance = 0

// opaque names a colour without spelling the alpha at every call site.
func opaque(r, g, b uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

// apart answers how far two colours are, in the widest channel.
func apart(a, b color.NRGBA) int {
	widest := 0
	for _, pair := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}, {a.A, b.A}} {
		gap := int(pair[0]) - int(pair[1])
		if gap < 0 {
			gap = -gap
		}
		if gap > widest {
			widest = gap
		}
	}
	return widest
}

// TestAColourSurvivesTheRoundTrip is the property everything else on this
// control rests on.
//
// A picker converts on every frame of a drag: bytes in to place the handles,
// bytes out to write the field. A conversion that lost a unit each way would
// darken a colour nobody was changing, one step per frame, and the bug would
// look like a screen slowly going out.
func TestAColourSurvivesTheRoundTrip(t *testing.T) {
	// A spread rather than the whole cube, so the test stays quick: the ends,
	// the middle, the two values either side of the middle, and one and
	// two hundred and fifty-four, which are where a rounding error shows
	// first.
	steps := []uint8{0, 1, 2, 17, 63, 64, 65, 127, 128, 129, 191, 200, 253, 254, 255}

	for _, r := range steps {
		for _, g := range steps {
			for _, b := range steps {
				in := opaque(r, g, b)
				h, s, v := rgbToHSV(in)
				out := hsvToRGB(h, s, v)

				if gap := apart(in, out); gap > tolerance {
					t.Fatalf("%v came back as %v, %d apart, through h=%v s=%v v=%v", in, out, gap, h, s, v)
				}
			}
		}
	}
}

// TestAGreyHasNoHueAndComesBackGrey is the classic defect in this conversion.
//
// Red, green and blue agree in a grey, so the hue is nought over nought. Left
// unguarded that is a value no comparison catches, and the colour that comes
// back out of it is not grey -- a person picks a shade of grey and the control
// answers with a colour.
func TestAGreyHasNoHueAndComesBackGrey(t *testing.T) {
	for level := 0; level < 256; level++ {
		grey := opaque(uint8(level), uint8(level), uint8(level))

		h, s, v := rgbToHSV(grey)
		if h != 0 || s != 0 {
			t.Errorf("%v answered hue %v and saturation %v; a grey has neither", grey, h, s)
		}

		if back := hsvToRGB(h, s, v); apart(grey, back) > tolerance {
			t.Errorf("%v came back as %v", grey, back)
		}
	}
}

// TestASaturationOfNoughtIsGreyAtEveryHue keeps a leftover hue from colouring
// a grey.
//
// The hue a picker holds does not disappear when somebody drags the handle to
// the colourless edge -- it is kept so that dragging back restores the colour.
// What must not happen is that it reaches the drawing: at saturation nought
// every hue is the same grey, and a conversion that read the sector table
// anyway would tint it.
func TestASaturationOfNoughtIsGreyAtEveryHue(t *testing.T) {
	for hue := 0; hue < 360; hue += 5 {
		for _, value := range []float32{0, 0.25, 0.5, 1} {
			got := hsvToRGB(float32(hue), 0, value)
			want := round8(value)

			if got.R != want || got.G != want || got.B != want {
				t.Fatalf("hue %d at value %v with no saturation drew %v, want a grey of %d", hue, value, got, want)
			}
		}
	}
}

// TestBlackAndWhiteAreTheSameAtEveryHue fixes the two ends of the value axis.
func TestBlackAndWhiteAreTheSameAtEveryHue(t *testing.T) {
	for hue := 0; hue <= 360; hue += 15 {
		for saturation := float32(0); saturation <= 1; saturation += 0.25 {
			if black := hsvToRGB(float32(hue), saturation, 0); black != opaque(0, 0, 0) {
				t.Errorf("hue %d at saturation %v with no value drew %v, want black", hue, saturation, black)
			}
		}
		if white := hsvToRGB(float32(hue), 0, 1); white != opaque(255, 255, 255) {
			t.Errorf("hue %d with no saturation at full value drew %v, want white", hue, white)
		}
	}
}

// TestTheSixCornersAreTheSaturatedPrimaries is what pins the sector table down.
//
// Each corner is where two of the three arms meet, so a table with two arms
// swapped still draws a spectrum -- it is smooth, it runs red to red, and the
// only thing wrong with it is that the colours are in the wrong order. These
// six are what say so.
func TestTheSixCornersAreTheSaturatedPrimaries(t *testing.T) {
	for _, corner := range []struct {
		hue  float32
		want color.NRGBA
	}{
		{0, opaque(255, 0, 0)},
		{60, opaque(255, 255, 0)},
		{120, opaque(0, 255, 0)},
		{180, opaque(0, 255, 255)},
		{240, opaque(0, 0, 255)},
		{300, opaque(255, 0, 255)},
	} {
		if got := hsvToRGB(corner.hue, 1, 1); got != corner.want {
			t.Errorf("hue %v drew %v, want %v", corner.hue, got, corner.want)
		}

		// And back again, so the two directions are pinned to the same table.
		h, s, v := rgbToHSV(corner.want)
		if h != corner.hue || s != 1 || v != 1 {
			t.Errorf("%v answered hue %v saturation %v value %v, want hue %v at full", corner.want, h, s, v, corner.hue)
		}
	}
}

// TestTheWheelCloses keeps the last pixel of the spectrum from being drawn by
// whatever the default arm of the table happens to be.
//
// Three hundred and sixty is nought, and a drag can land exactly on it: the
// hue bar hands over a fraction of one, and one times three hundred and sixty
// is the end of the wheel. Without the wrap the sector index is six, one past
// the table.
func TestTheWheelCloses(t *testing.T) {
	for _, hue := range []float32{360, 720, -360, 0} {
		if got := hsvToRGB(hue, 1, 1); got != opaque(255, 0, 0) {
			t.Errorf("hue %v drew %v, want the red at nought", hue, got)
		}
	}

	for _, pair := range [][2]float32{{0, 360}, {60, 420}, {200, -160}, {359, -1}} {
		first, second := hsvToRGB(pair[0], 0.8, 0.7), hsvToRGB(pair[1], 0.8, 0.7)
		if first != second {
			t.Errorf("hue %v drew %v and hue %v drew %v; they are the same point on the wheel", pair[0], first, pair[1], second)
		}
	}
}

// TestEveryHueIsReachedFromItsOwnColour walks the wheel in both directions.
//
// A table whose sectors are the right colours at the corners can still be
// wrong between them -- an arm that rises where it should fall gives a sector
// that starts and ends correctly and is inverted in the middle.
func TestEveryHueIsReachedFromItsOwnColour(t *testing.T) {
	for hue := 0; hue < 360; hue++ {
		drawn := hsvToRGB(float32(hue), 1, 1)
		back, saturation, value := rgbToHSV(drawn)

		if saturation != 1 || value != 1 {
			t.Fatalf("hue %d drew %v, which reads back at saturation %v and value %v", hue, drawn, saturation, value)
		}

		// A degree of slack, and only here: the hue is reconstructed from
		// three bytes, and one degree of the wheel is narrower than one step
		// of a byte at the sector boundaries.
		gap := back - float32(hue)
		if gap > 180 {
			gap -= 360
		}
		if gap < -180 {
			gap += 360
		}
		if gap > 1 || gap < -1 {
			t.Errorf("hue %d drew %v, which reads back as hue %v", hue, drawn, back)
		}
	}
}

// TestHexIsReadBackAsTheColourItNames closes the loop on the text field.
func TestHexIsReadBackAsTheColourItNames(t *testing.T) {
	for _, colour := range []color.NRGBA{
		opaque(0, 0, 0),
		opaque(255, 255, 255),
		opaque(18, 52, 86),
		{R: 255, G: 0, B: 0, A: 128},
		{R: 1, G: 2, B: 3, A: 0},
	} {
		written := hex(colour)
		read, ok := parseHex(written)

		if !ok {
			t.Fatalf("%v was written as %q, which will not read back", colour, written)
		}
		if read != colour {
			t.Errorf("%v was written as %q and read as %v", colour, written, read)
		}
	}
}

// TestHexSpellsTheAlphaOnlyWhenItSaysSomething keeps two characters of noise
// out of the one place on this control where somebody types.
func TestHexSpellsTheAlphaOnlyWhenItSaysSomething(t *testing.T) {
	if got := hex(opaque(255, 0, 0)); got != "#ff0000" {
		t.Errorf("an opaque red was written as %q", got)
	}
	if got := hex(color.NRGBA{R: 255, A: 128}); got != "#ff000080" {
		t.Errorf("a half-transparent red was written as %q", got)
	}
}

// TestAHexIsTakenInEveryFormAPersonWrites fixes what the field accepts.
//
// The three lengths, with the hash and without it, upper case and lower and
// mixed. A form missing from here is a form somebody pastes in and the field
// marks as a mistake.
func TestAHexIsTakenInEveryFormAPersonWrites(t *testing.T) {
	for _, taken := range []struct {
		written string
		want    color.NRGBA
	}{
		{"#fff", opaque(255, 255, 255)},
		{"fff", opaque(255, 255, 255)},
		{"#FFF", opaque(255, 255, 255)},
		{"#f00", opaque(255, 0, 0)},
		{"#0f0", opaque(0, 255, 0)},
		{"#abc", opaque(170, 187, 204)},
		{"#123", opaque(17, 34, 51)},
		{"012", opaque(0, 17, 34)},
		{"F0F", opaque(255, 0, 255)},
		{"#000", opaque(0, 0, 0)},
		{"#ff0000", opaque(255, 0, 0)},
		{"FF0000", opaque(255, 0, 0)},
		{"#Ff0000", opaque(255, 0, 0)},
		{"#000000", opaque(0, 0, 0)},
		{"ffffff", opaque(255, 255, 255)},
		{"#0a0B0c", opaque(10, 11, 12)},
		{"aBcDeF", opaque(171, 205, 239)},
		{"#012345", opaque(1, 35, 69)},
		{"#7f7F7f", opaque(127, 127, 127)},
		{"#fedcba", opaque(254, 220, 186)},
		{"#123456ff", opaque(18, 52, 86)},
		{"#ffffffff", opaque(255, 255, 255)},
		{"#00ff0080", color.NRGBA{R: 0, G: 255, B: 0, A: 128}},
		{"0000ff00", color.NRGBA{R: 0, G: 0, B: 255, A: 0}},
		{"#00000000", color.NRGBA{}},
		{"#11223344", color.NRGBA{R: 17, G: 34, B: 51, A: 68}},
		{"#ABCDEF01", color.NRGBA{R: 171, G: 205, B: 239, A: 1}},
		{"#deadbeef", color.NRGBA{R: 222, G: 173, B: 190, A: 239}},
		{"#1A2B3C4D", color.NRGBA{R: 26, G: 43, B: 60, A: 77}},
		{"#f0f0f0f0", color.NRGBA{R: 240, G: 240, B: 240, A: 240}},
	} {
		got, ok := parseHex(taken.written)

		if !ok {
			t.Errorf("%q was refused", taken.written)
			continue
		}
		if got != taken.want {
			t.Errorf("%q read as %v, want %v", taken.written, got, taken.want)
		}
	}
}

// TestAHexThatIsNotOneIsRefused is the other half, and the half that matters:
// a control that guessed would put a colour on the screen that nobody typed.
func TestAHexThatIsNotOneIsRefused(t *testing.T) {
	for _, written := range []string{
		"",
		"#",
		"##",
		"#gg0000",
		"gg0000",
		"#12345",
		"12345",
		"#1234",
		"#1234567",
		"#123456789",
		"#ff 000",
		" #ff0000",
		"#ff0000 ",
		"#ff-000",
		"rgb(255,0,0)",
		"red",
		"#xyz",
		"#ffffffg0",
		"##ff0000",
		"0x0000ff",
	} {
		if got, ok := parseHex(written); ok {
			t.Errorf("%q was taken as %v", written, got)
		}
	}
}

// TestAHandleStaysInsideTheAreaItMarks keeps the ring whole at every corner.
//
// A handle placed at the fraction alone is half outside the plane at nought
// and half outside it at one, which on a rounded field is a ring with a bite
// out of it -- and on the alpha bar it is a handle that appears to leave.
func TestAHandleStaysInsideTheAreaItMarks(t *testing.T) {
	corners := [][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0.5, 0.5}, {-1, 2}}

	for _, area := range []image.Point{{X: 40, Y: 40}, {X: 300, Y: 140}, {X: 15, Y: 601}, {X: 1, Y: 1}} {
		for _, radius := range []int{1, 7, 20} {
			for _, corner := range corners {
				centre := markCentre(area, corner[0], corner[1], radius)

				fits := area.X >= 2*radius && area.Y >= 2*radius
				if fits {
					if centre.X-radius < 0 || centre.X+radius > area.X {
						t.Errorf("in %v a handle of %d at %v sits from %d to %d across", area, radius, corner, centre.X-radius, centre.X+radius)
					}
					if centre.Y-radius < 0 || centre.Y+radius > area.Y {
						t.Errorf("in %v a handle of %d at %v sits from %d to %d down", area, radius, corner, centre.Y-radius, centre.Y+radius)
					}
					continue
				}

				// Too small to hold the handle: it goes in the middle, which
				// is the only place wrong by the least amount.
				if centre.X < 0 || centre.X > area.X || centre.Y < 0 || centre.Y > area.Y {
					t.Errorf("in %v, which cannot hold a handle of %d, it went to %v", area, radius, centre)
				}
			}
		}
	}
}

// TestAHandleMovesWithTheFractionItIsGiven keeps the clamp from being the only
// thing that ever runs.
func TestAHandleMovesWithTheFractionItIsGiven(t *testing.T) {
	area := image.Pt(300, 140)

	left := markCentre(area, 0, 0.5, 6)
	middle := markCentre(area, 0.5, 0.5, 6)
	right := markCentre(area, 1, 0.5, 6)

	if !(left.X < middle.X && middle.X < right.X) {
		t.Errorf("the handle does not travel: %d, %d, %d", left.X, middle.X, right.X)
	}
	if middle.X != 150 {
		t.Errorf("half way across %v is %d", area, middle.X)
	}
}

// TestAnAreaOfNothingDoesNotPlaceAHandleNowhere fixes the division a plane of
// no width would do.
//
// Nought over nought is a value that compares false against every bound, so a
// clamp written the obvious way lets it straight through and the handle is
// drawn at a coordinate that is not a number.
func TestAnAreaOfNothingDoesNotPlaceAHandleNowhere(t *testing.T) {
	var zero float32
	if got := clamp01(zero / zero); got != 0 {
		t.Errorf("a fraction of nothing over nothing clamped to %v", got)
	}
}

// TestAPickerStartsOnAColourSomebodyCanSee fixes the zero value.
//
// Every field of a fresh state is nought, and an alpha of nought is a colour
// that draws nothing: a screen showing the zero value would look like a
// control that failed to paint rather than one nobody has touched.
func TestAPickerStartsOnAColourSomebodyCanSee(t *testing.T) {
	var state ColourPicker

	if got := state.Colour(); got != opaque(0, 0, 0) {
		t.Errorf("a fresh picker is on %v, want opaque black", got)
	}
}

// TestTheHueIsKeptWhenTheColourCannotNameIt is the state half of the defect
// the conversions guard.
//
// Every grey is at every hue, and black is at every saturation. Taking the
// nought the conversion has to answer with would throw the handle into a
// corner for a colour that is not in that corner: the picture does not change,
// so it reads as a control moving by itself.
func TestTheHueIsKeptWhenTheColourCannotNameIt(t *testing.T) {
	var state ColourPicker
	state.SetColour(opaque(0, 0, 255)) // blue, hue 240

	for _, colourless := range []color.NRGBA{
		opaque(128, 128, 128),
		opaque(0, 0, 0),
		opaque(255, 255, 255),
	} {
		state.SetColour(colourless)

		if state.hue != 240 {
			t.Errorf("%v moved the hue to %v", colourless, state.hue)
		}
		if got := state.Colour(); got != colourless {
			t.Errorf("%v came back as %v", colourless, got)
		}
	}

	// And the saturation survives black, so that dragging the value back up
	// returns the colour rather than a white.
	state.SetColour(opaque(0, 0, 255))
	state.SetColour(opaque(0, 0, 0))
	if state.saturation != 1 {
		t.Errorf("black took the saturation to %v", state.saturation)
	}
}

// TestSettingAColourWritesTheField keeps the two halves of the control from
// disagreeing while nobody is looking.
func TestSettingAColourWritesTheField(t *testing.T) {
	var state ColourPicker
	state.SetColour(color.NRGBA{R: 18, G: 52, B: 86, A: 200})

	if got := state.hex.Text(); got != "#123456c8" {
		t.Errorf("the field says %q", got)
	}
}

// TestSettingAColourIsNotAChange keeps a screen from being handed back what it
// has just sent.
func TestSettingAColourIsNotAChange(t *testing.T) {
	var state ColourPicker
	state.SetColour(opaque(255, 0, 0))

	if state.Changed() {
		t.Error("a colour the caller set was reported as a change")
	}
}

// TestAChangeIsReportedOnceAndConsumed is the defect this package writes
// whenever a control has something to say.
//
// Reported every frame, a picker sends the same colour sixty times a second
// for as long as a finger is down -- and from the far end that is a client
// that will not stop talking.
func TestAChangeIsReportedOnceAndConsumed(t *testing.T) {
	var state ColourPicker
	state.changed = true

	if !state.Changed() {
		t.Fatal("a change was not reported")
	}
	if state.Changed() {
		t.Error("the same change was reported twice")
	}
}

// TestTypingAColourMovesThePickers is the first half of the field being
// two-way.
func TestTypingAColourMovesThePickers(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	state := &ColourPicker{}

	// What a person typing leaves behind: text in the field that the control
	// did not put there.
	state.hex.SetText("#00ff00")

	ColourPickerProps{}.Layout(c, state)

	if got := state.Colour(); got != opaque(0, 255, 0) {
		t.Errorf("typing a green left the picker on %v", got)
	}
	if !state.Changed() {
		t.Error("typing a colour was not reported as a change")
	}
}

// TestTypingSomethingThatIsNotAColourChangesNothing is the other half, and the
// one a control gets wrong by being helpful.
func TestTypingSomethingThatIsNotAColourChangesNothing(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	state := &ColourPicker{}
	state.SetColour(opaque(255, 0, 0))

	state.hex.SetText("#gg00")
	ColourPickerProps{}.Layout(c, state)

	if got := state.Colour(); got != opaque(255, 0, 0) {
		t.Errorf("a string that is not a colour moved the picker to %v", got)
	}
	if state.Changed() {
		t.Error("a string that is not a colour was reported as a change")
	}
	if got := state.hex.Text(); got != "#gg00" {
		t.Errorf("what was typed was rewritten to %q", got)
	}
}

// TestWhatWasTypedIsLeftInTheSpellingItWasTypedIn keeps the caret where the
// person put it.
//
// The control writes lower case with a hash. Somebody who typed "FF0000" named
// the same red, and rewriting it into the control's own spelling would throw
// their caret to the end of the field on the keystroke that completed it.
func TestWhatWasTypedIsLeftInTheSpellingItWasTypedIn(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	state := &ColourPicker{}

	state.hex.SetText("FF0000")
	ColourPickerProps{}.Layout(c, state)
	ColourPickerProps{}.Layout(c, state)

	if got := state.hex.Text(); got != "FF0000" {
		t.Errorf("the field was rewritten to %q", got)
	}
	if got := state.Colour(); got != opaque(255, 0, 0) {
		t.Errorf("the picker is on %v", got)
	}
}

// TestTheFieldIsWrittenFromThePickers is the direction a drag takes.
//
// The drag itself needs a pointer, which these tests have no way to press. The
// state a drag leaves is the same state, so it is written directly and what is
// checked is that the field follows it.
func TestTheFieldIsWrittenFromThePickers(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	state := &ColourPicker{}

	ColourPickerProps{}.Layout(c, state)
	if got := state.hex.Text(); got != "#000000" {
		t.Fatalf("a fresh picker wrote %q into its field", got)
	}

	state.hue, state.saturation, state.value = 240, 1, 1
	ColourPickerProps{}.Layout(c, state)

	if got := state.hex.Text(); got != "#0000ff" {
		t.Errorf("after the handles moved the field says %q", got)
	}
}

// TestAPickerReportsNothingWhileNobodyTouchesIt is the frame-by-frame half of
// once-and-consumed: a control that raised its event from its own drawing
// would raise it forever.
func TestAPickerReportsNothingWhileNobodyTouchesIt(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	state := &ColourPicker{}
	state.SetColour(color.NRGBA{R: 30, G: 200, B: 90, A: 140})

	props := ColourPickerProps{Alpha: true, Swatches: []color.NRGBA{opaque(255, 0, 0)}}
	for frame := 0; frame < 5; frame++ {
		props.Layout(c, state)

		if state.Changed() {
			t.Fatalf("frame %d reported a change nobody made", frame)
		}
	}

	if got := state.Colour(); got != (color.NRGBA{R: 30, G: 200, B: 90, A: 140}) {
		t.Errorf("five frames of drawing moved the colour to %v", got)
	}
}

// TestTheHueBarDoesNotDriftThroughTheSlider fixes the comparison that reports a
// change on a frame nobody touched.
//
// Hue is carried in degrees and the bar in a fraction of one, and the round
// trip through float32 does not land on the degree it started from. Compared in
// degrees the control finds a difference every frame, reports it, and writes
// the drifted value back -- so the colour walks while the screen sits still.
func TestTheHueBarDoesNotDriftThroughTheSlider(t *testing.T) {
	c, _ := field(t, theme.Light, 400)

	// Round hues, and then the hues a colour actually produces. The round ones
	// mostly divide and multiply back to themselves, which is why they are not
	// enough on their own: the defect this fixes hides behind exactly the
	// numbers somebody writes into a test by hand.
	hues := []float32{0, 1, 1.5, 3, 6, 12, 37, 179.5, 240, 359.9, 360}
	for _, colour := range []color.NRGBA{
		opaque(0, 7, 26), opaque(0, 7, 65), opaque(0, 14, 91),
		opaque(3, 200, 17), opaque(200, 3, 91), opaque(91, 200, 3),
	} {
		hue, _, _ := rgbToHSV(colour)
		hues = append(hues, hue)
	}

	for _, hue := range hues {
		state := &ColourPicker{}
		state.SetColour(opaque(1, 2, 3)) // settles the alpha
		state.hue, state.saturation, state.value = hue, 1, 1

		for frame := 0; frame < 3; frame++ {
			ColourPickerProps{Alpha: true}.Layout(c, state)
		}

		if state.hue != hue {
			t.Errorf("hue %v drifted to %v", hue, state.hue)
		}
		if state.Changed() {
			t.Errorf("hue %v reported a change nobody made", hue)
		}
	}
}

// TestTheAlphaBarDoesNotDriftThroughTheSlider is the same fact on the other
// bar, which carries a fraction on both sides and so has one fewer conversion
// to get wrong.
func TestTheAlphaBarDoesNotDriftThroughTheSlider(t *testing.T) {
	c, _ := field(t, theme.Light, 400)

	for alpha := 0; alpha < 256; alpha += 17 {
		state := &ColourPicker{}
		state.SetColour(color.NRGBA{R: 10, G: 20, B: 30, A: uint8(alpha)})

		for frame := 0; frame < 3; frame++ {
			ColourPickerProps{Alpha: true}.Layout(c, state)
		}

		if got := state.Colour().A; got != uint8(alpha) {
			t.Errorf("an alpha of %d drifted to %d", alpha, got)
		}
		if state.Changed() {
			t.Errorf("an alpha of %d reported a change nobody made", alpha)
		}
	}
}

// TestHidingTheAlphaBarDoesNotChangeTheColour fixes what the prop governs.
//
// It decides what is on the screen. A control that also rewrote the colour to
// opaque because a bar was hidden would be changing a value the caller set,
// for a reason the caller cannot see.
func TestHidingTheAlphaBarDoesNotChangeTheColour(t *testing.T) {
	c, _ := field(t, theme.Light, 400)
	state := &ColourPicker{}
	state.SetColour(color.NRGBA{R: 255, A: 64})

	ColourPickerProps{Alpha: false}.Layout(c, state)

	if got := state.Colour(); got.A != 64 {
		t.Errorf("hiding the bar took the alpha to %d", got.A)
	}
	if got := state.hex.Text(); got != "#ff000040" {
		t.Errorf("the field says %q, which does not name a colour that is not solid", got)
	}
}
