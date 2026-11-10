package widget

import (
	"image"
	"strings"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/gesture"
	"github.com/arandu-io/ayra/engine/io/event"
	"github.com/arandu-io/ayra/engine/io/key"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/op/clip"
	"github.com/arandu-io/ayra/engine/op/paint"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	engine "github.com/arandu-io/ayra/engine/widget"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
)

// Keyboard is the state half of the on-screen keyboard: the page it shows, the
// state of the shift key and which keys are held.
//
// The caller holds it, one per window. It has no text of its own: what is typed
// goes into whichever field has the focus (see [KeyboardProps]).
type Keyboard struct {
	symbols bool
	shift   shiftState
	keys    [keyboardKeys]gesture.Click

	// held is the key of the erase repeat, and since and last are when it went
	// down and when it last erased.
	held        bool
	since, last time.Time

	// revealed is the field [Keyboard.Reveal] last brought into view.
	revealed event.Tag
}

// keyboardKeys is the most keys a page draws, with room to spare.
const keyboardKeys = 48

type shiftState uint8

const (
	shiftOff shiftState = iota
	// shiftOnce capitalises the next letter and goes back to off.
	shiftOnce
	// shiftLocked capitalises until it is pressed again.
	shiftLocked
)

// Symbols reports whether the page of numbers and symbols is the one showing.
func (k *Keyboard) Symbols() bool { return k.symbols }

// Capitals reports whether the next letter is typed in capitals.
func (k *Keyboard) Capitals() bool { return k.shift != shiftOff }

// ShowSymbols shows the page of numbers and symbols, or the letters. A screen
// asking for a number or a PIN opens on the first.
func (k *Keyboard) ShowSymbols(on bool) { k.symbols = on }

// Capitalise sets the next letter in capitals, and every letter after it when
// lock is true. It is what the shift key does, for a screen that wants the first
// letter of a name capitalised.
func (k *Keyboard) Capitalise(lock bool) {
	k.shift = shiftOnce
	if lock {
		k.shift = shiftLocked
	}
}

// Reset goes back to the page of letters, in lower case. A screen calls it when
// a field takes the focus, so that a password does not start on the symbols the
// last field ended on.
func (k *Keyboard) Reset() {
	k.symbols, k.shift = false, shiftOff
}

// Open reports whether a field that takes typing has the keyboard: the moment
// to draw the on-screen keyboard, and to leave room for it. A field that is
// read-only, or nothing at all, is not one.
//
// It reads the focus the window delivered, so it answers the same in the whole
// frame, and a screen can ask it before laying anything out.
func (k *Keyboard) Open(c ayra.Context) bool {
	_, ok := typingTarget(c)
	return ok
}

// Reveal brings the field that has the keyboard into view when the room above the
// keyboard is the part of the window a screen lays its content in, and holds the
// focus while a list in that room scrolls to it. The screen calls it each frame it
// draws the keyboard, with the rectangle of its content in window coordinates; it
// acts once for each field that takes the focus.
//
// A window does this itself when the system's keyboard shrinks it. This keyboard
// is the application's own, the window keeps its size, and the screen is the one
// that makes the room: without this a list that draws only the rows that fit
// drops the field from the frame, and a focus whose control is not in the frame
// is given up, with the keyboard.
func (k *Keyboard) Reveal(c ayra.Context, viewport image.Rectangle) {
	tag := c.Source.Focus()
	if tag == nil {
		k.revealed = nil
		return
	}
	if tag == k.revealed {
		return
	}
	k.revealed = tag
	c.Source.Reveal(viewport)
	c.Redraw()
}

// typingTarget is the editor that has the keyboard, when it takes typing.
func typingTarget(c ayra.Context) (*engine.Editor, bool) {
	editor, ok := c.Source.Focus().(*engine.Editor)
	if !ok || editor == nil || editor.ReadOnly {
		return nil, false
	}
	return editor, true
}

// KeyboardLabels are the words on the keyboard and the names a screen reader
// says. A label left empty is the English one.
type KeyboardLabels struct {
	// Space is the space bar's name.
	Space string
	// Confirm is the key that finishes: the label on it.
	Confirm string
	// Symbols is the key that shows the numbers and symbols.
	Symbols string
	// Letters is the key that goes back to the letters.
	Letters string
	// Erase, Shift and Close are names for a screen reader, since the keys carry
	// an icon and no text.
	Erase, Shift, Close string
}

func (l KeyboardLabels) or(fallback, given string) string {
	if given != "" {
		return given
	}
	return fallback
}

// KeyboardProps is a keyboard for typing an e-mail address or a password, drawn
// by the application instead of the system's. It is native to this half: Kyse
// has no counterpart, because a browser always has the system's keyboard.
//
// It has letters in lower and upper case, the row of digits across the top, a
// page of the symbols an address and a password ask for, a space bar, erase, and
// a key that confirms. It is sized for a touch screen and is the full width it
// is given.
//
// It types into the field that has the focus, whatever that field is: an
// [Input], a [Masked], a password. It does not take the focus itself -- a press
// on a key leaves the field as it was, with the caret where it was -- so the
// screen does not hand it a field. The screen draws it when [Keyboard.Open]
// says a field is waiting, below the content and out of its way:
//
//	if keys.Open(c) {
//		widget.KeyboardProps{}.Layout(c, &keys)
//	}
//
// The confirm key is what the return key is: it raises the field's submission
// where the field has one, and lets the field go, so the keyboard closes. The
// key beside the space bar lets the field go without submitting it.
type KeyboardProps struct {
	// ComponentProps is what the caller changes on this one instance. The
	// keyboard publishes "root", the panel behind the keys; "key", a letter, a
	// digit or a symbol; "action", the keys that do something else -- shift,
	// erase, the page and close; "confirm", the key that finishes; and
	// "pressed", laid over the key that is held.
	ComponentProps
	// Labels are the words on it. The zero value is English.
	Labels KeyboardLabels
}

// PartNames are the parts this widget publishes.
func (p KeyboardProps) PartNames() []string {
	return []string{"root", "key", "action", "confirm", "pressed"}
}

type keyKind uint8

const (
	keyText keyKind = iota
	keyErase
	keyShift
	keyPage
	keyClose
	keyConfirm
	keySpace
)

// cell is one key: what it does, and where it is on the grid of its row. A slot
// is the width of an ordinary key and the gap after it.
type cell struct {
	kind keyKind
	text string
	at   float32
	span float32
}

// slots is the width of every row, in slots.
const slots = 11.5

func chars(chars string, at float32) []cell {
	var row []cell
	for i, r := range []rune(chars) {
		row = append(row, cell{kind: keyText, text: string(r), at: at + float32(i), span: 1})
	}
	return row
}

// rows are the keys of a page, top to bottom.
func (k *Keyboard) rows() [][]cell {
	digits := append(chars("1234567890", 0), cell{kind: keyErase, at: 10, span: 1.5})
	if k.symbols {
		return [][]cell{
			digits,
			chars("@#$%&*-+=_", .75),
			chars(`!?.,:;/\|~`, .75),
			chars("()[]{}<>^`", .75),
			{
				{kind: keyClose, at: 0, span: 1.5},
				{kind: keyPage, at: 1.5, span: 2},
				{kind: keyText, text: "'", at: 3.5, span: 1},
				{kind: keyText, text: `"`, at: 4.5, span: 1},
				{kind: keySpace, at: 5.5, span: 4},
				{kind: keyConfirm, at: 9.5, span: 2},
			},
		}
	}
	third := []cell{{kind: keyShift, at: 0, span: 2.5}}
	third = append(third, chars("zxcvbnm", 2.5)...)
	third = append(third, chars("-_", 9.5)...)
	return [][]cell{
		digits,
		chars("qwertyuiop", .75),
		chars("asdfghjkl", 1.25),
		third,
		{
			{kind: keyClose, at: 0, span: 1.5},
			{kind: keyPage, at: 1.5, span: 2},
			{kind: keyText, text: "@", at: 3.5, span: 1},
			{kind: keySpace, at: 4.5, span: 4},
			{kind: keyText, text: ".", at: 8.5, span: 1},
			{kind: keyConfirm, at: 9.5, span: 2},
		},
	}
}

// repeat timing for an erase key that is held.
const (
	eraseDelay    = 400 * time.Millisecond
	eraseInterval = 55 * time.Millisecond
)

// Layout draws the keyboard and returns the room it took: the width it was
// given, and as tall as the keys need.
func (p KeyboardProps) Layout(c ayra.Context, state *Keyboard) ayra.Dimensions {
	root := p.root(c)
	c.Constraints.Min.X = c.Constraints.Max.X
	return Surface(c, root, func(c ayra.Context) ayra.Dimensions {
		return p.keys(c, state)
	})
}

// root is the panel behind the keys.
func (p KeyboardProps) root(c ayra.Context) theme.Style {
	t := c.Theme
	return t.Resolve("keyboard", "root", p.VariantName("default"), theme.Style{
		Background:  theme.Colour(t.Colours.Muted),
		Border:      theme.Colour(t.Colours.Border),
		BorderWidth: theme.Pt(1),
		Radius:      theme.Pt(0),
		Padding:     theme.Uniform(8),
	}, p.ComponentProps)
}

// keyRows is how many rows of keys a page has: the digits, three of letters or
// symbols, and the bottom one.
const keyRows = 5

// geometry is the gap between keys, the pitch of a slot and the height of a key
// for a row of this width.
func geometry(c ayra.Context, width int) (gap int, pitch float32, height int) {
	gap = max(c.Dp(unit.Dp(6)), 2)
	pitch = (float32(width) + float32(gap)) / slots
	// Tall enough for a thumb on a wide screen and no taller than it needs to
	// be on a narrow one.
	height = min(max(int(pitch*0.62), c.Dp(unit.Dp(44))), c.Dp(unit.Dp(64)))
	return gap, pitch, height
}

// Height is the height in pixels that [KeyboardProps.Layout] takes in the room
// the context gives it. A screen that lays out around the keyboard asks it
// before drawing the keyboard.
func (p KeyboardProps) Height(c ayra.Context) int {
	pad := p.root(c).Padding
	top, bottom := c.Dp(unit.Dp(pad.Top)), c.Dp(unit.Dp(pad.Bottom))
	inner := c.Constraints.Max.X - c.Dp(unit.Dp(pad.Left)) - c.Dp(unit.Dp(pad.Right))
	gap, _, height := geometry(c, inner)
	return top + keyRows*height + (keyRows-1)*gap + bottom
}

// keys lays out the rows inside the panel.
func (p KeyboardProps) keys(c ayra.Context, state *Keyboard) ayra.Dimensions {
	width := c.Constraints.Max.X
	gap, pitch, height := geometry(c, width)

	editor, typing := typingTarget(c)
	rows := state.rows()

	// Presses are read before anything is drawn, so that the key that was
	// pressed is already typed, and the page it switched to already drawn.
	index := 0
	var actions []keyKind
	var typed []string
	for _, row := range rows {
		for _, cl := range row {
			for {
				ev, ok := state.keys[index].Update(c.Source)
				if !ok {
					break
				}
				if ev.Kind == gesture.KindPress {
					actions = append(actions, cl.kind)
					typed = append(typed, state.output(cl))
				}
			}
			index++
		}
	}
	for i, kind := range actions {
		state.press(c, editor, typing, kind, typed[i])
	}
	if len(actions) > 0 {
		// What a key changed is drawn in the next frame, and nothing else may ask
		// for one: the keyboard can be drawn after the field it types into.
		c.Execute(op.InvalidateCmd{})
	}
	state.repeatErase(c, editor, typing, rows)

	// The page may have changed under the presses.
	rows = state.rows()

	var y int
	index = 0
	for r, row := range rows {
		for _, cl := range row {
			x := int(cl.at * pitch)
			w := int(cl.span*pitch) - gap
			origin := op.Offset(image.Pt(x, y)).Push(c.Ops)
			p.drawKey(c, state, &state.keys[index], cl, image.Pt(w, height))
			origin.Pop()
			index++
		}
		y += height
		if r < len(rows)-1 {
			y += gap
		}
	}
	return ayra.Dimensions{Size: image.Pt(width, y)}
}

// output is the text a key types, in the case the shift key says.
func (k *Keyboard) output(cl cell) string {
	switch cl.kind {
	case keyText:
		if k.shift != shiftOff {
			return strings.ToUpper(cl.text)
		}
		return cl.text
	case keySpace:
		return " "
	}
	return ""
}

// press does what one key does.
func (k *Keyboard) press(c ayra.Context, editor *engine.Editor, typing bool, kind keyKind, typed string) {
	switch kind {
	case keyText, keySpace:
		if typing {
			editor.Insert(typed)
		}
		if k.shift == shiftOnce && kind == keyText {
			k.shift = shiftOff
		}
	case keyErase:
		if typing {
			editor.Delete(-1)
		}
		k.held, k.since, k.last = true, c.Now, c.Now
	case keyShift:
		k.shift = (k.shift + 1) % 3
	case keyPage:
		k.symbols = !k.symbols
	case keyClose:
		c.Execute(key.FocusCmd{Tag: nil})
		c.Execute(key.SoftKeyboardCmd{Show: false})
		k.Reset()
	case keyConfirm:
		if typing {
			editor.RequestSubmit()
		}
		c.Execute(key.FocusCmd{Tag: nil})
		c.Execute(key.SoftKeyboardCmd{Show: false})
		k.Reset()
	}
}

// repeatErase erases again and again while the erase key stays down.
func (k *Keyboard) repeatErase(c ayra.Context, editor *engine.Editor, typing bool, rows [][]cell) {
	if !k.held {
		return
	}
	index := 0
	down := false
	for _, row := range rows {
		for _, cl := range row {
			if cl.kind == keyErase && k.keys[index].Pressed() {
				down = true
			}
			index++
		}
	}
	if !down {
		k.held = false
		return
	}
	next := k.last.Add(eraseInterval)
	if k.last.Equal(k.since) {
		next = k.since.Add(eraseDelay)
	}
	if !c.Now.Before(next) {
		if typing {
			editor.Delete(-1)
		}
		k.last = c.Now
		next = c.Now.Add(eraseInterval)
	}
	c.Execute(op.InvalidateCmd{At: next})
}

// drawKey draws one key and registers it as a press target.
func (p KeyboardProps) drawKey(c ayra.Context, state *Keyboard, click *gesture.Click, cl cell, size image.Point) {
	t, variant := c.Theme, p.VariantName("default")

	key := theme.Style{
		Background:  theme.Colour(t.Colours.Card),
		Foreground:  theme.Colour(t.Colours.CardForeground),
		Border:      theme.Colour(t.Colours.Border),
		BorderWidth: theme.Pt(1),
		Radius:      theme.Pt(t.Metrics.Radius),
		Font:        theme.Font{Size: theme.Pt(t.Type.Heading)},
	}
	small := theme.Font{Size: theme.Pt(t.Type.Heading)}

	var style theme.Style
	switch cl.kind {
	case keyErase, keyShift, keyPage, keyClose:
		action := key
		action.Background = theme.Colour(t.Colours.Secondary)
		action.Foreground = theme.Colour(t.Colours.SecondaryForeground)
		action.Font = small
		if cl.kind == keyShift && state.shift != shiftOff {
			action.Background = theme.Colour(t.Colours.Primary)
			action.Foreground = theme.Colour(t.Colours.PrimaryForeground)
		}
		style = t.Resolve("keyboard", "action", variant, action, p.ComponentProps)
	case keyConfirm:
		confirm := key
		confirm.Background = theme.Colour(t.Colours.Primary)
		confirm.Foreground = theme.Colour(t.Colours.PrimaryForeground)
		confirm.Border = theme.Paint{}
		confirm.Font = small
		style = t.Resolve("keyboard", "confirm", variant, confirm, p.ComponentProps)
	case keySpace:
		key.Font = small
		style = t.Resolve("keyboard", "key", variant, key, p.ComponentProps)
	default:
		style = t.Resolve("keyboard", "key", variant, key, p.ComponentProps)
	}

	fill := style.Background.NRGBA()
	if click.Pressed() {
		fill = press(fill, true)
		style = t.Resolve("keyboard", "pressed", variant, theme.Style{Background: theme.Colour(fill)}, theme.Component{Parts: p.Parts}).Or(style)
		fill = style.Background.NRGBA()
	}
	radius := c.Dp(unit.Dp(style.Radius.Points()))
	shape := clip.UniformRRect(image.Rectangle{Max: size}, radius)
	if fill.A > 0 {
		paint.FillShape(c.Ops, fill, shape.Op(c.Ops))
	}
	if border := style.Border.NRGBA(); border.A > 0 {
		stroke := clip.Stroke{Path: shape.Path(c.Ops), Width: float32(c.Dp(unit.Dp(style.BorderWidth.Points())))}
		paint.FillShape(c.Ops, border, stroke.Op())
	}

	label, name := p.face(state, cl)
	closeNode := Announce(c, size, Announcement{Label: name, Class: Pressable})
	area := clip.Rect(image.Rectangle{Max: size}).Push(c.Ops)
	click.Add(c.Ops)
	area.Pop()

	inner := c
	inner.Constraints = layout.Exact(size)
	layout.Center.Layout(inner.Context, func(gtx layout.Context) layout.Dimensions {
		kc := c.With(gtx)
		kc.Constraints.Min = image.Point{}
		ink := style.Foreground.NRGBA()
		if icon := p.icon(state, cl); !icon.IsZero() {
			side := c.Dp(unit.Dp(style.Font.Size.Or(theme.Pt(t.Type.Heading)).Points() + 8))
			DrawIcon(kc, icon, side, ink)
			return ayra.Dimensions{Size: image.Pt(side, side)}
		}
		return drawPart(kc, label, style, unit.Sp(t.Type.Body), 1, text.Middle, font.Font{Weight: font.Medium})
	})
	closeNode()
}

// face is what is written on a key, and the name a screen reader says for it.
func (p KeyboardProps) face(state *Keyboard, cl cell) (label, name string) {
	l := p.Labels
	switch cl.kind {
	case keyErase:
		return "", l.or("Erase", l.Erase)
	case keyShift:
		return "", l.or("Shift", l.Shift)
	case keyClose:
		return "", l.or("Close the keyboard", l.Close)
	case keyPage:
		if state.symbols {
			label = l.or("ABC", l.Letters)
		} else {
			label = l.or("?123", l.Symbols)
		}
		return label, label
	case keyConfirm:
		label = l.or("OK", l.Confirm)
		return label, label
	case keySpace:
		return "", l.or("Space", l.Space)
	}
	label = state.output(cl)
	return label, label
}

// icon is the mark on the keys that carry one.
func (p KeyboardProps) icon(state *Keyboard, cl cell) icons.Icon {
	switch cl.kind {
	case keyErase:
		return innerIcon("keyboard", "Backspace")
	case keyShift:
		if state.shift == shiftLocked {
			return innerIcon("keyboard", "ArrowFatLineUp")
		}
		return innerIcon("keyboard", "ArrowFatUp")
	case keyClose:
		return innerIcon("keyboard", "CaretDown")
	}
	return icons.Icon{}
}
