package widget

import (
	"image"
	"slices"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
)

// isoDay is how a period travels: the two days as the server's date filters
// read them, whatever the field shows.
const isoDay = "2006-01-02"

// fieldsFor makes the state of the text and period controls a facet needs. The
// states live as long as the table, because a control drawn without its state
// is a control that forgets what was typed.
func (e *tableExtras) fieldsFor(facets []Facet) {
	for _, facet := range facets {
		switch facet.Kind {
		case FacetText:
			if e.typed == nil {
				e.typed = map[string]*Input{}
			}
			if e.typed[facet.Key] == nil {
				e.typed[facet.Key] = &Input{}
			}
		case FacetPeriod:
			if e.span == nil {
				e.span = map[string]*DateRange{}
			}
			if e.span[facet.Key] == nil {
				e.span[facet.Key] = &DateRange{}
			}
		default:
			if e.toggle == nil {
				e.toggle = map[string]*Toggle{}
			}
			for _, option := range facet.Options {
				if key := facet.Key + "\x00" + option.Value; e.toggle[key] == nil {
					e.toggle[key] = &Toggle{}
				}
			}
		}
	}
}

// field draws the control of a text or period facet inline, for the toolbar of a
// table wide enough to hold it.
func (p DataTableProps) field(c ayra.Context, state *DataTable, facet Facet) ayra.Dimensions {
	state.m.fieldsFor(p.Facets)
	c.Constraints.Min = image.Point{}
	c.Constraints.Max.X = min(c.Constraints.Max.X, c.Dp(210))
	if facet.Kind == FacetText {
		return InputProps{
			ComponentProps: ComponentProps{Custom: p.Custom, Parts: p.Parts},
			widget:         "datatable-search",
			Placeholder:    orDefault(facet.Placeholder, facet.Title),
		}.Layout(c, state.m.typed[facet.Key])
	}
	return DateRangePickerProps{
		Placeholder: orDefault(facet.Placeholder, facet.Title),
		Today:       c.Now,
	}.Layout(c, state.m.span[facet.Key])
}

// readFields follows the text and period controls of a table drawn wide: a
// period moves the query when it is completed, a text after a pause in the
// typing. The narrow table asks with its Apply button instead, so they are not
// read there.
func (p DataTableProps) readFields(c ayra.Context, state *DataTable) bool {
	if p.narrow(c) {
		return false
	}
	state.m.fieldsFor(p.Facets)
	debounce := p.Debounce
	if debounce == 0 {
		debounce = 300 * time.Millisecond
	}
	changed := false
	for _, facet := range p.Facets {
		switch facet.Kind {
		case FacetText:
			typed := textOf(state.m.typed[facet.Key].Text())
			current := first(state.query.Filters[facet.Key])
			switch {
			case typed != state.m.seen[facet.Key]:
				if state.m.seen == nil {
					state.m.seen = map[string]string{}
					state.m.seenAt = map[string]time.Time{}
				}
				state.m.seen[facet.Key], state.m.seenAt[facet.Key] = typed, c.Now
			case typed != current && c.Now.Sub(state.m.seenAt[facet.Key]) >= debounce:
				state.query.Filters = withFilter(state.query.Filters, facet.Key, typed)
				changed = true
			}
		case FacetPeriod:
			if state.m.span[facet.Key].Changed() {
				from, to := state.m.span[facet.Key].Range()
				state.query.Filters = withPeriod(state.query.Filters, facet, from, to)
				changed = true
			}
		}
	}
	return changed
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// withFilter is the filters with one facet set to a single value, or without it
// when the value is empty. It never changes the map it was given.
func withFilter(filters map[string][]string, key, value string) map[string][]string {
	next := make(map[string][]string, len(filters)+1)
	for k, v := range filters {
		next[k] = slices.Clone(v)
	}
	if value == "" {
		delete(next, key)
	} else {
		next[key] = []string{value}
	}
	if len(next) == 0 {
		return nil
	}
	return next
}

// withPeriod is the filters with a period set, or without one when it is empty:
// the first day under the facet's key and the last under its end key.
func withPeriod(filters map[string][]string, facet Facet, from, to time.Time) map[string][]string {
	if from.IsZero() {
		return withFilter(withFilter(filters, facet.Key, ""), facet.EndKey, "")
	}
	return withFilter(withFilter(filters, facet.Key, from.Format(isoDay)), facet.EndKey, to.Format(isoDay))
}

// OpenFilters opens the sheet of filters, for a screen that has a button of its
// own for it. The sheet is the narrow table's; a table drawn wide has its
// filters in the toolbar and this opens nothing there.
func (p DataTableProps) OpenFilters(state *DataTable) { p.openSheet(state) }

// openSheet fills the controls of the sheet with the query in force, so that it
// opens on what is applied and not on what was left in it.
func (p DataTableProps) openSheet(state *DataTable) {
	state.m.fieldsFor(p.Facets)
	for _, facet := range p.Facets {
		chosen := state.query.Filters[facet.Key]
		switch facet.Kind {
		case FacetText:
			state.m.typed[facet.Key].SetText(first(chosen))
		case FacetPeriod:
			from, _ := time.Parse(isoDay, first(chosen))
			to, _ := time.Parse(isoDay, first(state.query.Filters[facet.EndKey]))
			if from.IsZero() || to.IsZero() {
				state.m.span[facet.Key].Clear()
			} else {
				state.m.span[facet.Key].Set(from, to)
			}
		default:
			for _, option := range facet.Options {
				state.m.toggle[facet.Key+"\x00"+option.Value].Set(slices.Contains(chosen, option.Value))
			}
		}
	}
	state.m.sheet.Open()
}

// applied is the query's filters as the sheet's controls now say them. A filter
// the table was given that no facet names is kept as it was.
func (p DataTableProps) applied(state *DataTable) map[string][]string {
	next := map[string][]string{}
	named := map[string]bool{}
	for _, facet := range p.Facets {
		named[facet.Key], named[facet.EndKey] = true, true
	}
	for key, values := range state.query.Filters {
		if !named[key] {
			next[key] = slices.Clone(values)
		}
	}
	for _, facet := range p.Facets {
		switch facet.Kind {
		case FacetText:
			if value := textOf(state.m.typed[facet.Key].Text()); value != "" {
				next[facet.Key] = []string{value}
			}
		case FacetPeriod:
			if from, to := state.m.span[facet.Key].Range(); !from.IsZero() {
				next[facet.Key], next[facet.EndKey] = []string{from.Format(isoDay)}, []string{to.Format(isoDay)}
			}
		default:
			var chosen []string
			for _, option := range facet.Options {
				if state.m.toggle[facet.Key+"\x00"+option.Value].On() {
					chosen = append(chosen, option.Value)
				}
			}
			if len(chosen) > 0 {
				next[facet.Key] = chosen
			}
		}
	}
	if len(next) == 0 {
		return nil
	}
	return next
}

// sheetLayout draws the sheet of filters over the table and follows its buttons.
//
// The filters of a phone are a sheet from the bottom, in reach of a thumb, and
// one request: nothing is asked while the controls are being set, and the
// button that applies them asks once. A table given no height to fill -- one
// that grows in a column -- has no bottom to come up from, so the sheet is drawn
// in its place instead of over it.
func (p DataTableProps) sheetLayout(c ayra.Context, state *DataTable) {
	if !state.m.sheet.Showing() {
		return
	}
	state.m.fieldsFor(p.Facets)

	switch {
	case state.m.close.Clicked(c):
		state.m.sheet.Close()
		return
	case state.m.reset.Clicked(c):
		state.query.Filters = nil
		state.query.Page = 1
		state.pages.Show(1)
		state.fetch(c, p)
		p.openSheet(state)
	case state.m.apply.Clicked(c):
		state.query.Filters = p.applied(state)
		state.query.Page = 1
		state.pages.Show(1)
		state.fetch(c, p)
		state.m.sheet.Close()
		return
	}

	content := func(c ayra.Context) ayra.Dimensions { return p.sheet(c, state) }
	if c.Constraints.Max.Y < boundedHeight {
		// A press on the scrim outside the sheet closes it, and one on the sheet
		// reaches the sheet's own buttons.
		DrawerProps{Side: Bottom, Size: 460, Modal: true, Dismissible: true}.Layout(c, &state.m.sheet, content)
		return
	}
	content(c)
}

// sheet is what is inside the sheet: a title, the sections, and the two buttons.
func (p DataTableProps) sheet(c ayra.Context, state *DataTable) ayra.Dimensions {
	t := c.Theme
	sections := []layout.FlexChild{}
	heading := func(label string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return drawText(c.With(gtx), label, unit.Sp(t.Type.Small), t.Colours.MutedForeground, 1, text.Start, semibold())
			})
		})
	}

	for _, facet := range p.Facets {
		facet := facet
		sections = append(sections, heading(facet.Title))
		switch facet.Kind {
		case FacetText:
			sections = append(sections, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return InputProps{
					ComponentProps: ComponentProps{Custom: p.Custom, Parts: p.Parts},
					widget:         "datatable-search",
					Placeholder:    orDefault(facet.Placeholder, facet.Title),
				}.Layout(c.With(gtx), state.m.typed[facet.Key])
			}))
		case FacetPeriod:
			sections = append(sections, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return DateRangePickerProps{Placeholder: orDefault(facet.Placeholder, facet.Title), Today: c.Now}.Layout(c.With(gtx), state.m.span[facet.Key])
			}))
		default:
			for _, option := range facet.Options {
				option := option
				toggle := state.m.toggle[facet.Key+"\x00"+option.Value]
				sections = append(sections, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					toggle.Changed(inner)
					return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return CheckboxProps{Label: orDefault(option.Label, option.Value)}.Layout(inner.With(gtx), toggle)
					})
				}))
			}
		}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(c.Context,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return drawText(c.With(gtx), orDefault(p.Words.Filters, "Filters"), unit.Sp(t.Type.Heading), t.Colours.Foreground, 1, text.Start, semibold())
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					inner := c.With(gtx)
					inner.Constraints.Min = image.Point{}
					return ButtonProps{Label: orDefault(p.Words.Close, "Close"), Variant: Ghost, Size: Small}.Layout(inner, &state.m.close)
				}),
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			state.m.sheetList.Axis = layout.Vertical
			return state.m.sheetList.Layout(gtx, len(sections), func(gtx layout.Context, i int) layout.Dimensions {
				return layoutChild(gtx, sections[i])
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return fill(c.With(gtx), ButtonProps{Label: orDefault(p.Words.Reset, "Reset"), Variant: Outline}, &state.m.reset)
					}),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return fill(c.With(gtx), ButtonProps{Label: orDefault(p.Words.Apply, "Apply")}, &state.m.apply)
					}),
				)
			})
		}),
	)
}

// fill draws a button across the room it was given.
func fill(c ayra.Context, b ButtonProps, state *Button) ayra.Dimensions {
	c.Constraints.Min.X = c.Constraints.Max.X
	return b.Layout(c, state)
}

// layoutChild lays out a flex child on its own, which is what a list of sections
// built as flex children needs.
func layoutChild(gtx layout.Context, child layout.FlexChild) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, child)
}
