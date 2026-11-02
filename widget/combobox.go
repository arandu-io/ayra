package widget

import (
	"context"
	"sync"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/op"
)

// Option is a value a person chooses by its name: the label is what is drawn and
// typed, and the value is what the application keeps.
type Option struct {
	// Value is what is stored: an identifier, a code.
	Value string
	// Label is what is drawn and searched.
	Label string
}

// Lookup is what an application implements to answer a search from its server:
// the options that match what was typed. It runs off the frame loop, in its own
// goroutine, and the context is cancelled when a newer search supersedes it.
type Lookup interface {
	Search(ctx context.Context, query string) ([]Option, error)
}

// LookupFunc is a function as a [Lookup].
type LookupFunc func(ctx context.Context, query string) ([]Option, error)

// Search calls the function.
func (f LookupFunc) Search(ctx context.Context, query string) ([]Option, error) {
	return f(ctx, query)
}

// Combobox is the state of a field that searches a server as it is typed in and
// holds the option that was chosen.
type Combobox struct {
	field Autocomplete

	mu       sync.Mutex
	options  []Option
	err      error
	loading  bool
	cancel   context.CancelFunc
	seq      int
	asked    string
	typed    string
	typedAt  time.Time
	selected Option
	has      bool
	reported bool
}

// Selected is the option chosen, and false when none is. Typing again after a
// choice un-chooses it: a field showing one text and holding another value is a
// field that submits what nobody can see.
func (b *Combobox) Selected() (Option, bool) { return b.selected, b.has }

// Chosen reports the option that was picked, once, and consumes it.
func (b *Combobox) Chosen() (Option, bool) {
	if !b.reported {
		return Option{}, false
	}
	b.reported = false
	return b.selected, true
}

// Set chooses an option without a search, for a form that opens on a value it
// already holds.
func (b *Combobox) Set(option Option) {
	b.selected, b.has = option, true
	b.field.SetText(option.Label)
	b.typed, b.asked = option.Label, option.Label
}

// Clear forgets the choice and empties the field.
func (b *Combobox) Clear() {
	b.selected, b.has = Option{}, false
	b.field.SetText("")
	b.typed, b.asked = "", ""
}

// Err is the error of the last search, and nil when it worked.
func (b *Combobox) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// ComboboxProps is a field that searches a server and chooses one of what it
// finds -- the counterpart of Kyse's Combobox with a remote source.
//
// The text is a search, and the choice is an [Option]: what a person types is
// never what is stored, which is the identifier of what they chose. The search
// waits for a pause in the typing, a search superseded by a newer one is
// cancelled and its answer discarded, and the server's order is kept: it has
// already decided what matches, and a second filter here would drop what the
// first one found.
type ComboboxProps struct {
	// ComponentProps is what the caller changes on this one instance. A
	// combobox publishes the parts of an [InputProps].
	ComponentProps
	// Source answers the searches. Nil draws a field that finds nothing.
	Source Lookup
	// Placeholder is shown while the field is empty.
	Placeholder string
	// MinChars is how many characters are typed before the server is asked. Zero
	// takes one.
	MinChars int
	// Debounce is how long the search waits for the next keystroke. Zero takes
	// 250 ms.
	Debounce time.Duration
	// Limit is how many options are drawn at once. Zero draws six.
	Limit int
	// Empty is said when the server found nothing, Searching while it looks, and
	// Failed when the search itself failed. Failed empty says the error the
	// source returned.
	Empty, Searching, Failed string
	// Disabled draws it as unavailable and refuses to open.
	Disabled bool
	// Invalid draws the field as rejected. What was wrong is said beside it, by
	// whoever knows; a failed search marks it too, and says why under it.
	Invalid bool
}

// PartNames are the parts this widget publishes, by the names the web
// component publishes in data-part.
func (p ComboboxProps) PartNames() []string {
	return []string{"root", "focus", "invalid", "placeholder"}
}

// Layout draws the field and, while there are options, the list under it.
func (p ComboboxProps) Layout(c ayra.Context, state *Combobox) ayra.Dimensions {
	min := p.MinChars
	if min <= 0 {
		min = 1
	}
	debounce := p.Debounce
	if debounce == 0 {
		debounce = 250 * time.Millisecond
	}

	typed := state.field.Text()
	switch {
	case typed != state.typed:
		state.typed, state.typedAt = typed, c.Now
		if state.has && typed != state.selected.Label {
			state.selected, state.has = Option{}, false
		}
	case typed != state.asked && c.Now.Sub(state.typedAt) >= debounce:
		state.asked = typed
		if len([]rune(typed)) >= min && !(state.has && typed == state.selected.Label) {
			state.search(c, p, typed)
		}
	case typed != state.asked:
		c.Execute(op.InvalidateCmd{At: state.typedAt.Add(debounce)})
	}

	state.mu.Lock()
	options, loading, failure := state.options, state.loading, state.err
	state.mu.Unlock()

	entries := make([]string, len(options))
	for i, option := range options {
		entries[i] = option.Label
	}
	empty := p.Empty
	problem := false
	switch {
	case loading:
		entries, empty = nil, orDefault(p.Searching, "Searching...")
	case failure != nil:
		entries, empty, problem = nil, orDefault(p.Failed, failure.Error()), true
	}
	if len([]rune(typed)) < min {
		entries, empty = nil, ""
	}

	dims := AutocompleteProps{
		ComponentProps: p.ComponentProps, widget: "combobox",
		Entries: entries, Placeholder: p.Placeholder, Limit: p.Limit,
		Empty: empty, Disabled: p.Disabled, Unfiltered: true,
		Invalid: p.Invalid || problem, problem: problem,
		tail: innerIcon("combobox", "CaretDown"),
	}.Layout(c, &state.field)

	if chosen := state.field.Chosen(); chosen >= 0 && chosen < len(options) {
		state.selected, state.has, state.reported = options[chosen], true, true
		state.typed, state.asked = options[chosen].Label, options[chosen].Label
	}
	return dims
}

// search asks the server and supersedes the search in flight.
func (b *Combobox) search(c ayra.Context, p ComboboxProps, query string) {
	if p.Source == nil {
		return
	}
	b.mu.Lock()
	if b.cancel != nil {
		b.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.seq++
	seq := b.seq
	b.loading = true
	b.mu.Unlock()

	redraw := c.Redraw
	go func() {
		found, err := p.Source.Search(ctx, query)
		b.mu.Lock()
		defer b.mu.Unlock()
		if seq != b.seq {
			return
		}
		b.loading = false
		if err != nil {
			b.err, b.options = err, nil
		} else {
			b.err, b.options = nil, found
		}
		redraw()
	}()
}
