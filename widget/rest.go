package widget

import (
	"context"
	"image"
	"sync"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/font"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/mask"
	"github.com/arandu-io/ayra/theme"
)

// OutputProps is a value the screen calculated, with the name of what it is --
// the counterpart of Kyse's Output. On a native screen that is text, and this is
// the text with its label drawn the way every label in the product is, so that a
// total and the line above it are the same pair everywhere.
type OutputProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root", whose foreground and font are the value's.
	ComponentProps
	// Label names the value. Empty draws the value alone.
	Label string
	// Value is what was calculated.
	Value string
	// Role and Tone are the value's, as for a [TextProps]. Zero is running text.
	Role Role
	Tone Tone
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p OutputProps) PartNames() []string { return []string{"root"} }

// Layout draws the label and the value and returns the room they took.
func (p OutputProps) Layout(c ayra.Context) ayra.Dimensions {
	t := c.Theme
	style := t.Resolve("output", "root", p.VariantName(p.Tone.String()), theme.Style{
		Foreground: theme.Colour(p.Tone.ink(t)),
		Font:       theme.Font{Size: theme.Pt(float32(p.Role.size(t)))},
	}, p.ComponentProps)

	return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
		inner := style
		inner.Padding = theme.Insets{}
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.Label == "" {
					return layout.Dimensions{}
				}
				return LabelProps{Text: p.Label}.Layout(c.With(gtx))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawPart(c.With(gtx), p.Value, inner, p.Role.size(t), 0, text.Start, p.Role.fontFace())
			}),
		)
	})
}

// fontFace is the font a role is set in, before the theme's scale has its say.
func (r Role) fontFace() (f font.Font) {
	if r == Mono {
		f.Typeface = theme.MonoTypeface
	}
	return f
}

// ImageVariant is one rendition of a picture: the source, and how wide it is.
type ImageVariant struct {
	// Width is the picture's width in pixels.
	Width int
	// Source produces it.
	Source ImageSource
}

// ResponsiveImage is the state of a picture that comes in several sizes: the
// rendition it chose, and the picture loaded from it.
type ResponsiveImage struct {
	image  *Image
	chosen int
}

// Chosen is the index of the rendition in use, and -1 before the first frame.
func (r *ResponsiveImage) Chosen() int {
	if r.image == nil {
		return -1
	}
	return r.chosen
}

// ResponsiveImageProps is a picture that comes in several sizes -- the
// counterpart of Kyse's ResponsiveImage, where the browser picks a source by the
// screen it is on. A window is the one that knows its pixels, so it picks: the
// narrowest rendition that is still as wide as the box, in device pixels, and the
// widest there is when none is. A phone at three pixels to a point asks for a
// larger file than a laptop at one, and neither downloads the other's.
type ResponsiveImageProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes the part of an [ImageProps].
	ComponentProps
	// Variants are the renditions, in any order.
	Variants []ImageVariant
	// Width, Height, Fit and Alt are the box's and the picture's, as in an
	// [ImageProps].
	Width, Height unit.Dp
	Fit           Fit
	Alt           string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ResponsiveImageProps) PartNames() []string { return []string{"root"} }

// Layout picks the rendition for the box and draws it.
func (p ResponsiveImageProps) Layout(c ayra.Context, state *ResponsiveImage) ayra.Dimensions {
	width := c.Constraints.Max.X
	if p.Width != 0 {
		width = c.Dp(p.Width)
	}
	pick := p.pick(width)

	if state.image == nil || state.chosen != pick {
		// A rendition that changed is a picture that has not been loaded: the
		// state of the old one is let go, and the new one loads on its own.
		if state.image != nil {
			state.image.Release()
		}
		state.image, state.chosen = new(Image), pick
	}

	var source ImageSource
	if pick >= 0 {
		source = p.Variants[pick].Source
	}
	root := c.Theme.Resolve("responsiveimage", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	return ImageProps{
		ComponentProps: ComponentProps{Style: root}, Source: source, Width: p.Width, Height: p.Height, Fit: p.Fit, Alt: p.Alt,
	}.Layout(c, state.image)
}

// pick is the index of the rendition for a box this many pixels wide.
func (p ResponsiveImageProps) pick(pixels int) int {
	best, widest := -1, -1
	for i, v := range p.Variants {
		if widest < 0 || v.Width > p.Variants[widest].Width {
			widest = i
		}
		if v.Width >= pixels && (best < 0 || v.Width < p.Variants[best].Width) {
			best = i
		}
	}
	if best < 0 {
		return widest
	}
	return best
}

// DialogTriggerProps is the control that opens a dialog -- the counterpart of
// Kyse's DialogTrigger. On the web it is a link that falls back to a page holding
// the same form; a window has no page to fall back to, so it is a button that
// opens the dialog the screen already holds.
type DialogTriggerProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root", which is the button.
	ComponentProps
	// Label is what it says. Empty draws the icon alone.
	Label string
	// Icon is drawn before the label.
	Icon icons.Icon
	// Dialog is the dialog it opens: the one the screen holds, whose state is the
	// caller's like every state.
	Dialog *Dialog
	// Variant and Size are the button's.
	Variant Variant
	Size    Size
	// Disabled draws it as unavailable.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DialogTriggerProps) PartNames() []string { return []string{"root"} }

// Layout draws the button, and opens the dialog when it is pressed.
func (p DialogTriggerProps) Layout(c ayra.Context, state *Button) ayra.Dimensions {
	if state.Clicked(c) && !p.Disabled && p.Dialog != nil {
		p.Dialog.Open()
	}
	root := c.Theme.Resolve("dialogtrigger", "root", p.VariantName(p.Variant.String()), theme.Style{}, p.ComponentProps)
	return ButtonProps{
		ComponentProps: ComponentProps{Style: root}, Label: p.Label, Icon: p.Icon,
		Variant: p.Variant, Size: p.Size, Disabled: p.Disabled,
	}.Layout(c, state)
}

// DeleteRow is the state of the control that removes a row after asking once.
type DeleteRow struct {
	armed     bool
	trigger   Button
	confirm   Button
	cancel    Button
	confirmed bool
}

// Arm puts the control in the state of having been pressed once, for a screen
// that restores it and for the catalogue.
func (d *DeleteRow) Arm() { d.armed = true }

// Confirmed reports that the removal was confirmed, once, and consumes it. What
// is removed, and from where, is the screen's.
func (d *DeleteRow) Confirmed() bool {
	confirmed := d.confirmed
	d.confirmed = false
	return confirmed
}

// DeleteRowProps is the control at the end of a row that removes it -- the
// counterpart of Kyse's DeleteRow. On the web the row swaps itself out of the page
// when the server answers; here the screen removes it from the data the list is
// drawn from, and what this adds is the one question that stands between a press
// and the loss: the first press asks, and the second one answers.
type DeleteRowProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root".
	ComponentProps
	// Label names the action for a screen reader, and ConfirmLabel and
	// CancelLabel are the two buttons of the question.
	Label, ConfirmLabel, CancelLabel string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p DeleteRowProps) PartNames() []string { return []string{"root"} }

// Layout draws the trash, or the question once it was pressed.
func (p DeleteRowProps) Layout(c ayra.Context, state *DeleteRow) ayra.Dimensions {
	switch {
	case state.trigger.Clicked(c):
		state.armed = true
	case state.cancel.Clicked(c):
		state.armed = false
	case state.confirm.Clicked(c):
		state.armed, state.confirmed = false, true
	}

	return withRoot(c, "deleterow", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		if !state.armed {
			return ButtonProps{
				Icon: icons.Trash(), Size: Icon, Variant: Ghost, Label: "",
			}.Layout(c, &state.trigger)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				return ButtonProps{Label: orDefault(p.ConfirmLabel, "Delete"), Variant: Destructive, Size: Small}.Layout(inner, &state.confirm)
			}),
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				inner := c.With(gtx)
				inner.Constraints.Min = image.Point{}
				return ButtonProps{Label: orDefault(p.CancelLabel, "Cancel"), Variant: Ghost, Size: Small}.Layout(inner, &state.cancel)
			}),
		)
	})
}

// LazyLoad is the state of a region that is built when it is first drawn.
type LazyLoad struct {
	seen  bool
	ready bool
	told  bool
}

// Visible reports that the region was drawn for the first time, once. It is the
// moment to fetch what it will hold.
func (l *LazyLoad) Visible() bool {
	if l.seen && !l.told {
		l.told = true
		return true
	}
	return false
}

// Ready says the content can be drawn: what it needed has arrived.
func (l *LazyLoad) Ready() { l.ready = true }

// LazyLoadProps is a region that holds a placeholder until it is needed -- the
// counterpart of Kyse's LazyLoad. The browser asks when a fragment scrolls into
// view; a native list draws only the rows in view, so a region inside one is
// drawn for the first time exactly when it scrolls in, and [LazyLoad.Visible]
// says so. The placeholder stands until the application calls [LazyLoad.Ready],
// or at once with Immediate.
type LazyLoadProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes "root" and "placeholder".
	ComponentProps
	// Placeholder is what stands in. Nil draws a skeleton.
	Placeholder ayra.Widget
	// Immediate draws the content from the first frame the region is drawn,
	// when there is nothing to wait for.
	Immediate bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p LazyLoadProps) PartNames() []string { return []string{"root", "placeholder"} }

// Layout draws the placeholder or the content.
func (p LazyLoadProps) Layout(c ayra.Context, state *LazyLoad, content ayra.Widget) ayra.Dimensions {
	state.seen = true
	if p.Immediate {
		state.ready = true
	}
	return withRoot(c, "lazyload", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		if state.ready {
			return content(c)
		}
		style := c.Theme.Resolve("lazyload", "placeholder", p.VariantName("default"), theme.Style{}, p.ComponentProps)
		return Surface(c, style, func(c ayra.Context) ayra.Dimensions {
			if p.Placeholder != nil {
				return p.Placeholder(c)
			}
			return SkeletonProps{}.Layout(c)
		})
	})
}

// OptimisticToggle is the state of a switch that draws the new position before
// the server has confirmed it, and puts it back if the server refuses.
type OptimisticToggle struct {
	toggle Toggle
	mu     sync.Mutex
	err    error
	busy   int
}

// On reports the position drawn.
func (o *OptimisticToggle) On() bool { return o.toggle.On() }

// Set puts the switch in a position without asking anybody, for the state a
// screen arrives with.
func (o *OptimisticToggle) Set(on bool) { o.toggle.Set(on) }

// Err is why the last change was refused, and nil when it was not.
func (o *OptimisticToggle) Err() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.err
}

// Pending reports whether a change is still waiting for the server.
func (o *OptimisticToggle) Pending() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.busy > 0
}

// ask sends the new position to the server in the background, and puts the
// switch back if the answer is no.
func (o *OptimisticToggle) ask(confirm func(ctx context.Context, on bool) error, on bool, redraw func()) {
	o.mu.Lock()
	o.busy++
	o.err = nil
	o.mu.Unlock()

	go func() {
		var err error
		if confirm != nil {
			err = confirm(context.Background(), on)
		}
		o.mu.Lock()
		o.busy--
		if err != nil {
			o.err = err
			// Put back, unless the person has moved it again since: the last
			// press is the one that counts.
			if o.toggle.On() == on {
				o.toggle.Set(!on)
			}
		}
		o.mu.Unlock()
		redraw()
	}()
}

// OptimisticToggleProps is a switch that believes the server -- the counterpart of
// Kyse's OptimisticToggle. A press moves it at once and asks Confirm in the
// background; if Confirm fails it moves back and keeps the error. The web half
// needs a component for this because a page is rebuilt by the server; a window
// holds its own state, and what is left to say is what to do when the answer is no.
type OptimisticToggleProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root".
	ComponentProps
	// Label is the text beside the switch.
	Label string
	// Confirm asks the server for the new position. It runs off the frame loop.
	Confirm func(ctx context.Context, on bool) error
	// Disabled draws it as unavailable.
	Disabled bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p OptimisticToggleProps) PartNames() []string { return []string{"root"} }

// Layout draws the switch and, under it, why the last change was refused.
func (p OptimisticToggleProps) Layout(c ayra.Context, state *OptimisticToggle) ayra.Dimensions {
	if !p.Disabled && state.toggle.Changed(c) {
		state.ask(p.Confirm, state.toggle.On(), c.Redraw)
	}

	return withRoot(c, "optimistictoggle", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return SwitchProps{Label: p.Label, Disabled: p.Disabled}.Layout(c.With(gtx), &state.toggle)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if err := state.Err(); err != nil {
					return drawText(c.With(gtx), err.Error(), unit.Sp(c.Theme.Type.Small), c.Theme.Colours.Destructive, 0, text.Start, plain())
				}
				return layout.Dimensions{}
			}),
		)
	})
}

// ActiveSearch is the state of a search whose results are listed under the field.
type ActiveSearch struct {
	input   Input
	mu      sync.Mutex
	options []Option
	err     error
	loading bool
	cancel  context.CancelFunc
	seq     int
	rows    []Button
	typed   string
	typedAt time.Time
	asked   string
	chosen  Option
	picked  bool
}

// Text is what has been typed.
func (a *ActiveSearch) Text() string { return a.input.Text() }

// Chosen reports the result that was pressed, once.
func (a *ActiveSearch) Chosen() (Option, bool) {
	if !a.picked {
		return Option{}, false
	}
	a.picked = false
	return a.chosen, true
}

// ActiveSearchProps is a search that answers as it is typed, with the results
// listed under the field -- the counterpart of Kyse's ActiveSearch. It differs
// from a [ComboboxProps] in what it is for: a combobox chooses a value for a
// form and closes, and this is a search page, where the results stay on the
// screen and each one is a way into somewhere.
type ActiveSearchProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes the parts of an [InputProps], restyled under "activesearch".
	ComponentProps
	// Source answers the searches.
	Source Lookup
	// Placeholder is shown while the field is empty.
	Placeholder string
	// MinChars is how many characters are typed before the server is asked. Zero
	// takes one.
	MinChars int
	// Debounce is how long the search waits for the next keystroke. Zero takes
	// 250 ms.
	Debounce time.Duration
	// Empty is said when nothing matched, and Searching while it looks.
	Empty, Searching string
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ActiveSearchProps) PartNames() []string {
	return []string{"root", "focus", "invalid", "placeholder"}
}

// Layout draws the field and the results.
func (p ActiveSearchProps) Layout(c ayra.Context, state *ActiveSearch) ayra.Dimensions {
	min := p.MinChars
	if min <= 0 {
		min = 1
	}
	debounce := p.Debounce
	if debounce == 0 {
		debounce = 250 * time.Millisecond
	}

	typed := state.input.Text()
	switch {
	case typed != state.typed:
		state.typed, state.typedAt = typed, c.Now
	case typed != state.asked && c.Now.Sub(state.typedAt) >= debounce:
		state.asked = typed
		state.search(c, p, typed, min)
	case typed != state.asked:
		c.Execute(op.InvalidateCmd{At: state.typedAt.Add(debounce)})
	}

	state.mu.Lock()
	options, loading := state.options, state.loading
	state.mu.Unlock()
	for len(state.rows) < len(options) {
		state.rows = append(state.rows, Button{})
	}
	for i := range options {
		if state.rows[i].Clicked(c) {
			state.chosen, state.picked = options[i], true
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return InputProps{ComponentProps: p.ComponentProps, widget: "activesearch", Placeholder: p.Placeholder}.Layout(c.With(gtx), &state.input)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inner := c.With(gtx)
			switch {
			case len([]rune(typed)) < min:
				return layout.Dimensions{}
			case loading:
				return drawText(inner, orDefault(p.Searching, "Searching..."), unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
			case len(options) == 0:
				return drawText(inner, orDefault(p.Empty, "No results."), unit.Sp(c.Theme.Type.Small), c.Theme.Colours.MutedForeground, 1, text.Start, plain())
			}
			rows := make([]layout.FlexChild, 0, len(options))
			for i, option := range options {
				i, option := i, option
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return ItemProps{Title: option.Label, Pressable: true}.Layout(inner.With(gtx), &state.rows[i], nil)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}),
	)
}

// search asks the server and supersedes the search in flight.
func (a *ActiveSearch) search(c ayra.Context, p ActiveSearchProps, query string, min int) {
	if p.Source == nil || len([]rune(query)) < min {
		a.mu.Lock()
		a.options = nil
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.seq++
	seq := a.seq
	a.loading = true
	a.mu.Unlock()

	redraw := c.Redraw
	go func() {
		found, err := p.Source.Search(ctx, query)
		a.mu.Lock()
		defer a.mu.Unlock()
		if seq != a.seq {
			return
		}
		a.loading = false
		a.err, a.options = err, found
		redraw()
	}()
}

// HoverCard is the state of a panel that opens when the pointer rests on its
// control.
type HoverCard struct {
	panel Menu
	since time.Time
	over  bool
	// byHover is whether the pointer is what opened the panel. One opened by a
	// press stays until it is pressed again, and one opened by hovering goes
	// when the pointer leaves.
	byHover bool
}

// HoverCardProps is a panel that opens when the pointer rests on its control, and
// on a press -- the counterpart of Kyse's HoverCard. A touch screen has no hover
// to open on, so the press is how it opens there, and the same control serves
// both: a pointer that rests for OpenDelay opens it, and one that leaves closes it
// after CloseDelay unless it was opened by a press.
type HoverCardProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes one part, "root".
	ComponentProps
	// Width is how wide the panel is, as for a [PopoverProps].
	Width unit.Dp
	// OpenDelay is how long the pointer rests before the panel opens. Zero takes
	// 400 ms. CloseDelay is how long after it leaves the panel stays. Zero takes
	// 150 ms.
	OpenDelay, CloseDelay time.Duration
	// Above opens the panel upwards.
	Above bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p HoverCardProps) PartNames() []string { return []string{"root"} }

// Layout draws the control and, while it is open, the panel beside it.
func (p HoverCardProps) Layout(c ayra.Context, state *HoverCard, trigger, content ayra.Widget) ayra.Dimensions {
	open, closeAfter := p.OpenDelay, p.CloseDelay
	if open == 0 {
		open = 400 * time.Millisecond
	}
	if closeAfter == 0 {
		closeAfter = 150 * time.Millisecond
	}

	hovered := state.panel.open.click.Hovered()
	switch {
	case hovered && !state.over:
		state.over, state.since = true, c.Now
	case !hovered && state.over:
		state.over, state.since = false, c.Now
	}
	if !state.panel.Showing() {
		state.byHover = false
	}
	switch {
	case state.over && !state.panel.Showing():
		if c.Now.Sub(state.since) >= open {
			state.panel.Open()
			state.byHover = true
		} else {
			c.Execute(op.InvalidateCmd{At: state.since.Add(open)})
		}
	case !state.over && state.panel.Showing() && state.byHover:
		if c.Now.Sub(state.since) >= closeAfter {
			state.panel.Close()
		} else {
			c.Execute(op.InvalidateCmd{At: state.since.Add(closeAfter)})
		}
	}

	return withRoot(c, "hovercard", "default", p.ComponentProps, func(c ayra.Context) ayra.Dimensions {
		return PopoverProps{Width: p.Width, Above: p.Above}.Layout(c, &state.panel, trigger, content)
	})
}

// FormField is one field of a [FormDialogProps].
type FormField struct {
	// Name is the key the field's value is reported under, and the key an error
	// for it is matched by.
	Name string
	// Label names the field, Help is the sentence under it, and Placeholder is
	// shown while it is empty.
	Label, Help, Placeholder string
	// Kind is what the field expects, which decides the on-screen keyboard.
	Kind Kind
	// Required marks the field as one that cannot be left blank.
	Required bool
	// Mask formats what is typed, and the value reported is the raw one. Nil is
	// an ordinary field.
	Mask mask.Mask
	// Value is what the field opens holding.
	Value string
}

// FormDialog is the state of a dialog that holds a form: whether it is open, the
// fields, and the answer.
type FormDialog struct {
	dialog    Dialog
	fields    []Masked
	opened    bool
	submit    Button
	cancel    Button
	submitted bool
	values    map[string]string
}

// Open shows the form with the fields holding their Value, as at first.
func (f *FormDialog) Open() {
	f.opened = false
	f.dialog.Open()
}

// Close hides it, for a screen that saw the server accept the form.
func (f *FormDialog) Close() { f.dialog.Close() }

// Showing reports whether it is on screen.
func (f *FormDialog) Showing() bool { return f.dialog.Showing() }

// Dismissed reports that the person closed it without submitting, once.
func (f *FormDialog) Dismissed() bool { return f.dialog.Dismissed() }

// Submitted reports the values of the fields, raw, once -- and false when the
// form was not submitted. The dialog stays open: it is the server that accepts a
// form, and a form it refuses is still on the screen with the person's answers
// in it, which is what a redirect back to the dialog is on the web.
func (f *FormDialog) Submitted() (map[string]string, bool) {
	if !f.submitted {
		return nil, false
	}
	f.submitted = false
	return f.values, true
}

// collect reads the fields, raw, and holds them for [FormDialog.Submitted].
func (f *FormDialog) collect(p FormDialogProps) {
	values := make(map[string]string, len(p.Fields))
	for i, field := range p.Fields {
		values[field.Name] = f.fields[i].Value(field.Mask)
	}
	f.values, f.submitted = values, true
}

// FormDialogProps is a dialog that holds a form -- the counterpart of Kyse's
// FormDialog. It draws the title, the fields, the server's errors beside the
// fields they belong to, and the two buttons; it decides nothing. The values are
// reported raw, a mask taken off, and the server validates them: a field that
// checks its own answer is a second place the rule lives, and the one that
// disagrees.
type FormDialogProps struct {
	// ComponentProps is what the caller changes on this one instance. It
	// publishes "root", the panel of the dialog.
	ComponentProps
	// Title and Description say what the form asks.
	Title, Description string
	// Fields are the questions.
	Fields []FormField
	// Errors are what the server refused, by field name. An entry under "" is
	// about the form as a whole and is drawn above the buttons.
	Errors map[string]string
	// SubmitLabel and CancelLabel are the two buttons. SubmitVariant is the
	// first one's look: Destructive for a form that removes something.
	SubmitLabel, CancelLabel string
	SubmitVariant            Variant
	// Busy draws the submit button as waiting and refuses a second press, for
	// the time the server takes to answer.
	Busy bool
	// Width is how wide the panel is, as for a [DialogProps].
	Width unit.Dp
	// Alert makes the dialog one that cannot be dismissed by a press outside:
	// it asks something that must be answered.
	Alert bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p FormDialogProps) PartNames() []string { return []string{"root"} }

// Layout draws the dialog over the screen. A closed one draws nothing.
func (p FormDialogProps) Layout(c ayra.Context, state *FormDialog) ayra.Dimensions {
	for len(state.fields) < len(p.Fields) {
		state.fields = append(state.fields, Masked{})
	}
	if state.dialog.Showing() && !state.opened {
		state.opened = true
		for i, field := range p.Fields {
			state.fields[i].SetValue(field.Mask, field.Value)
		}
	}

	if state.dialog.Showing() {
		switch {
		case state.cancel.Clicked(c):
			state.dialog.close()
		case state.submit.Clicked(c) && !p.Busy:
			state.collect(p)
		}
	}

	root := c.Theme.Resolve("formdialog", "root", p.VariantName("default"), theme.Style{}, p.ComponentProps)
	return DialogProps{
		ComponentProps: ComponentProps{Style: root}, Modal: true, Dismissible: !p.Alert, Width: p.Width,
	}.Layout(c, &state.dialog, func(c ayra.Context) ayra.Dimensions {
		t := c.Theme
		rows := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), p.Title, unit.Sp(t.Type.Heading), t.Colours.Foreground, 0, text.Start, semibold())
			}),
		}
		if p.Description != "" {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: 4}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), p.Description, unit.Sp(t.Type.Body), t.Colours.MutedForeground, 0, text.Start, plain())
			}))
		}
		rows = append(rows, layout.Rigid(layout.Spacer{Height: 14}.Layout))

		for i, field := range p.Fields {
			i, field := i, field
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return FieldProps{Label: field.Label, Help: field.Help, Error: p.Errors[field.Name], Required: field.Required}.Layout(c.With(gtx), func(c ayra.Context) ayra.Dimensions {
					return MaskedProps{
						Mask: field.Mask, Placeholder: field.Placeholder, Kind: field.Kind,
						Invalid: p.Errors[field.Name] != "", Disabled: p.Busy,
					}.Layout(c, &state.fields[i])
				})
			}), layout.Rigid(layout.Spacer{Height: 12}.Layout))
		}

		if general := p.Errors[""]; general != "" {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return AlertProps{Title: general, Severity: Failure}.Layout(c.With(gtx))
			}), layout.Rigid(layout.Spacer{Height: 12}.Layout))
		}

		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					return ButtonProps{Label: orDefault(p.CancelLabel, "Cancel"), Variant: Outline}.Layout(inner, &state.cancel)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					return ButtonProps{Label: orDefault(p.SubmitLabel, "Save"), Variant: p.SubmitVariant, Disabled: p.Busy}.Layout(inner, &state.submit)
				}),
			)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(c.Context, rows...)
	})
}
