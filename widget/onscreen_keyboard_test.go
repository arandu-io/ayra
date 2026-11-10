package widget_test

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/mask/brazil"
	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/widget"
)

// The on-screen keyboard is proved the way a person uses it: a field is tapped,
// the keys are tapped, and the field is read. Every test drives a real router,
// because the part that can fail is the one a frame with no input attached never
// reaches -- a key that took the focus from the field it was typing into would
// type the first letter and nothing after it.

// board is a field over the keyboard, laid out the way a screen does: the
// keyboard is drawn only while a field waits for it.
type board struct {
	p         *panel
	field     widget.Input
	masked    widget.Masked
	keys      widget.Keyboard
	useMasked bool
	height    int
	submitted bool
	// open is what the keyboard answered in the last frame.
	open bool
}

func newBoard() *board { return &board{p: newPanel(), height: 40} }

func (b *board) screen(c ayra.Context) ayra.Dimensions {
	field := func(gtx layout.Context) layout.Dimensions {
		c := c.With(gtx)
		// Asked before the field is drawn, as the field's own documentation says:
		// drawing it reads the events and drops the ones nobody asked for.
		if b.useMasked {
			if b.masked.Submitted() {
				b.submitted = true
			}
			return widget.MaskedProps{Mask: brazil.CPF}.Layout(c, &b.masked)
		}
		if b.field.Submitted(c) {
			b.submitted = true
		}
		return widget.InputProps{}.Layout(c, &b.field)
	}
	children := []layout.FlexChild{layout.Rigid(field)}
	b.open = b.keys.Open(c)
	if b.open {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 20}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return widget.KeyboardProps{}.Layout(c.With(gtx), &b.keys)
			}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
}

// The geometry of the keyboard at this panel: 400 wide, a padding of 8, a gap of
// 6 and keys 44 tall, which is what the layout derives from them.
const (
	boardGap  = 6.0
	boardPad  = 8.0
	boardKeyH = 44.0
)

// top is where the keyboard starts: the field, and the 20 points under it. It is
// measured on a panel of its own, because drawing another screen on the board's
// would replace the targets its router holds.
func (b *board) top() int {
	field := newPanel().draw(func(c ayra.Context) ayra.Dimensions {
		return widget.InputProps{}.Layout(c, new(widget.Input))
	})
	return field.Size.Y + 20
}

// tapKey presses the key in a row at a slot, in the middle of it.
func (b *board) tapKey(row int, slot, span float64) {
	pitch := (400 - 2*boardPad + boardGap) / 11.5
	x := boardPad + (slot+span/2)*pitch - boardGap/2
	y := float64(b.top()) + boardPad + float64(row)*(boardKeyH+boardGap) + boardKeyH/2
	tap(b.p, b.screen, int(x), int(y))
}

// focus taps the field.
func (b *board) focus() {
	b.p.draw(b.screen)
	tap(b.p, b.screen, 40, 15)
	b.p.draw(b.screen)
}

func (b *board) text() string {
	if b.useMasked {
		return b.masked.Text()
	}
	return b.field.Text()
}

// The pages, as the layout draws them: each row's keys from the slot it starts at.
var (
	lettersPage = []struct {
		keys string
		at   float64
	}{{"1234567890", 0}, {"qwertyuiop", .75}, {"asdfghjkl", 1.25}, {"zxcvbnm", 2.5}, {"-_", 9.5}}
	symbolsPage = []struct {
		keys string
		at   float64
	}{{"1234567890", 0}, {"@#$%&*-+=_", .75}, {`!?.,:;/\|~`, .75}, {"()[]{}<>^`", .75}}
)

// tapChar presses the key that types r on the page showing.
func (b *board) tapChar(r rune, symbols bool) {
	page, row := lettersPage, 0
	if symbols {
		page = symbolsPage
	}
	for i, line := range page {
		if n := strings.IndexRune(line.keys, r); n >= 0 {
			row = i
			if !symbols && i == 4 {
				row = 3
			}
			b.tapKey(row, line.at+float64(n), 1)
			return
		}
	}
	switch r {
	case '@':
		b.tapKey(4, 3.5, 1)
	case '.':
		b.tapKey(4, 8.5, 1)
	case ' ':
		b.tapKey(4, map[bool]float64{false: 4.5, true: 5.5}[symbols], 4)
	case '\'':
		b.tapKey(4, 3.5, 1)
	case '"':
		b.tapKey(4, 4.5, 1)
	default:
		panic("no key for " + string(r))
	}
}

func (b *board) shift()   { b.tapKey(3, 0, 2.5) }
func (b *board) page()    { b.tapKey(4, 1.5, 2) }
func (b *board) erase()   { b.tapKey(0, 10, 1.5) }
func (b *board) confirm() { b.tapKey(4, 9.5, 2) }
func (b *board) closeIt() { b.tapKey(4, 0, 1.5) }

func (b *board) typed(s string, symbols bool) {
	for _, r := range s {
		b.tapChar(r, symbols)
	}
}

func TestTheKeyboardIsDrawnOnlyWhileAFieldHasTheFocus(t *testing.T) {
	b := newBoard()
	ctx, _ := frame(t, theme.Light, 400)
	var keys widget.Keyboard
	if keys.Open(ctx) {
		t.Error("the keyboard is open in a window where nothing has the focus")
	}

	alone := b.p.draw(b.screen).Size
	b.focus()
	size := b.p.draw(b.screen).Size
	if size.Y-alone.Y < 5*44 {
		t.Errorf("a field with the focus left the keyboard %v tall, want five rows of keys", size)
	}
	if size.X > 400 {
		t.Errorf("the keyboard is %d wide in a room of 400", size.X)
	}
}

func TestTheKeysTypeIntoTheFieldAndLeaveItTheFocus(t *testing.T) {
	b := newBoard()
	b.focus()

	b.typed("paulo@a.br", false)
	if got := b.text(); got != "paulo@a.br" {
		t.Fatalf("the field holds %q after typing paulo@a.br on the keys", got)
	}
	if !b.open {
		t.Error("typing on the keys took the focus from the field")
	}
}

func TestShiftCapitalisesOneLetterAndTheSecondPressLocksIt(t *testing.T) {
	b := newBoard()
	b.focus()

	b.shift()
	b.typed("ab", false)
	if got := b.text(); got != "Ab" {
		t.Errorf("shift then a, b gave %q, want Ab", got)
	}

	b.shift()
	b.shift()
	b.typed("cd", false)
	if got := b.text(); got != "AbCD" {
		t.Errorf("shift twice then c, d gave %q, want the two capitals of a locked shift", got)
	}
	b.shift()
	b.typed("e", false)
	if got := b.text(); got != "AbCDe" {
		t.Errorf("a third press of shift left the capitals on: %q", got)
	}
}

func TestTheSymbolsPageTypesWhatAnAddressAndAPasswordAsk(t *testing.T) {
	b := newBoard()
	b.focus()

	b.page()
	if !b.keys.Symbols() {
		t.Fatal("the page key did not show the symbols")
	}
	b.typed("1#$%&*+=!?", true)
	b.page()
	if b.keys.Symbols() {
		t.Fatal("the page key did not go back to the letters")
	}
	b.typed("x", false)
	if got := b.text(); got != "1#$%&*+=!?x" {
		t.Errorf("the field holds %q", got)
	}
}

func TestEraseRemovesTheLastCharacterAndSpaceTypesOne(t *testing.T) {
	b := newBoard()
	b.focus()
	b.typed("ab c", false)
	if got := b.text(); got != "ab c" {
		t.Fatalf("space gave %q", got)
	}
	b.erase()
	b.erase()
	if got := b.text(); got != "ab" {
		t.Errorf("two erases left %q, want ab", got)
	}
}

func TestConfirmSubmitsTheFieldAndLetsItGo(t *testing.T) {
	b := newBoard()
	b.focus()
	b.typed("a", false)
	b.confirm()
	b.p.draw(b.screen)

	if !b.submitted {
		t.Error("the confirm key did not submit the field the way return does")
	}
	if b.open {
		t.Error("the keyboard stayed open after the confirm key")
	}
	if got := b.text(); got != "a" {
		t.Errorf("the field holds %q after confirming", got)
	}
}

func TestCloseLetsTheFieldGoWithoutSubmittingIt(t *testing.T) {
	b := newBoard()
	b.focus()
	b.typed("a", false)
	b.closeIt()
	b.p.draw(b.screen)

	if b.submitted {
		t.Error("the close key submitted the field")
	}
	if b.open {
		t.Error("the keyboard stayed open after the close key")
	}
}

func TestTheKeysTypeIntoAMaskedField(t *testing.T) {
	b := newBoard()
	b.useMasked = true
	b.focus()
	b.typed("12345678909", false)
	b.p.draw(b.screen)
	if got := b.text(); got != "123.456.789-09" {
		t.Errorf("a CPF typed on the keys is %q, want it formatted", got)
	}
}

// drawAt is a frame of the board at an instant, which the panel's own does not
// have: the repeat of a held erase key is a matter of time.
func (b *board) drawAt(now time.Time) {
	p := b.p
	p.ops.Reset()
	gtx := layout.Context{
		Ops:         p.ops,
		Source:      p.router.Source(),
		Now:         now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: p.size},
	}
	b.screen(ayra.Context{Context: gtx, Theme: theme.New(theme.Light), Shaper: p.shaper})
	p.router.Frame(p.ops)
}

func TestAHeldEraseKeyKeepsErasing(t *testing.T) {
	b := newBoard()
	b.focus()
	b.typed("abcdefghij", false)

	pitch := (400 - 2*boardPad + boardGap) / 11.5
	at := f32.Pt(float32(boardPad+10.75*pitch-boardGap/2), float32(float64(b.top())+boardPad+boardKeyH/2))

	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	b.drawAt(now)
	b.p.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: at})
	b.drawAt(now)
	if got := b.text(); got != "abcdefghi" {
		t.Fatalf("the press erased %q, want one character gone", got)
	}

	b.drawAt(now.Add(200 * time.Millisecond))
	if got := b.text(); got != "abcdefghi" {
		t.Errorf("the key repeated after 200ms (%q), before the delay", got)
	}
	b.drawAt(now.Add(450 * time.Millisecond))
	b.drawAt(now.Add(520 * time.Millisecond))
	b.drawAt(now.Add(580 * time.Millisecond))
	if got := b.text(); len(got) > 7 || len(got) < 6 {
		t.Errorf("after 580ms held the field holds %q, want the key to have repeated", got)
	}

	b.p.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: at})
	b.drawAt(now.Add(600 * time.Millisecond))
	held := b.text()
	b.drawAt(now.Add(900 * time.Millisecond))
	if got := b.text(); got != held {
		t.Errorf("the key kept erasing after it was let go: %q from %q", got, held)
	}
}

// The keyboard is the application's own, so the window does not shrink: the
// screen gives its content the room that is left, and a list in it draws only the
// rows that fit. The field that was tapped can be one it leaves out for a few
// frames, until the list has scrolled to it. A focus whose control is not in the
// frame is given up, with the keyboard; Keyboard.Reveal is what holds it.
func TestTheKeyboardKeepsAFieldThatTheRoomItLeavesDoesNotShow(t *testing.T) {
	for _, reveal := range []bool{false, true} {
		p := newPanel()
		var (
			field  widget.Input
			keys   widget.Keyboard
			opened bool
			since  int
		)
		screen := func(c ayra.Context) ayra.Dimensions {
			open := keys.Open(c)
			opened = open
			if open {
				since++
			} else {
				since = 0
			}
			children := []layout.FlexChild{layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				// The field is left out of the first frames of the smaller room.
				if open && since <= 3 {
					return layout.Dimensions{Size: gtx.Constraints.Max}
				}
				return widget.InputProps{}.Layout(c.With(gtx), &field)
			})}
			if open {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return widget.KeyboardProps{}.Layout(c.With(gtx), &keys)
				}))
			}
			dims := layout.Flex{Axis: layout.Vertical}.Layout(c.Context, children...)
			if open && reveal {
				keys.Reveal(c, image.Rect(0, 0, 400, 100))
			}
			return dims
		}

		for range 3 {
			p.draw(screen)
		}
		tap(p, screen, 100, 15)
		for range 12 {
			p.draw(screen)
		}
		if reveal && !opened {
			t.Error("the keyboard closed although the screen asked Keyboard.Reveal to hold the field")
		}
		if !reveal && opened {
			t.Error("the focus survived a field that was not drawn with nothing holding it: this test no longer shows what Reveal is for")
		}
	}
}
