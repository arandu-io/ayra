package widget

import (
	"image"
	"image/color"
	"slices"
	"sort"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/icons"
	"github.com/arandu-io/ayra/theme"
)

// The icons a component draws inside itself.
//
// A Kyse component writes its own arrows, crosses and eyes into its markup, and
// the ones it writes are a decision of the design: the same caret opens the
// accordion, the combobox and the split button, so a person who learned what it
// means on one reads the next. A native control that drew a triangle of its own
// where Kyse draws a caret would be the second set the icons package exists to
// prevent.
//
// So the table below is the one place where a widget gets one of these. It is
// keyed by the name of the Kyse component the widget pairs with, and holds the
// Kyse function names of the icons that component draws -- the same words
// `grep "icons\." components/*.kyse.go` prints. A widget asks for its icon by
// the pair, and a pair that is not in the table draws nothing, which a test
// finds. Two more tests keep the table honest: one reads the widgets' source
// and fails on an icon called from anywhere but here, and one (the root
// package's icons_parity_test.go) reads Kyse's source and fails when the table
// and Kyse disagree about a component.
//
// The icons are drawn as Kyse's are: the same Phosphor path, with the lighter
// half at a fifth of the strength, in the colour of the text around it.
var innerIcons = map[string]map[string]func() icons.Icon{
	"accordion":         {"CaretDown": icons.CaretDown},
	"breadcrumb":        {"CaretRight": icons.CaretRight},
	"carousel":          {"CaretLeft": icons.CaretLeft, "CaretRight": icons.CaretRight},
	"calendar":          {"CaretLeft": icons.CaretLeft, "CaretRight": icons.CaretRight},
	"checkbox":          {"Check": icons.Check},
	"collapsible":       {"CaretDown": icons.CaretDown},
	"combobox":          {"CaretDown": icons.CaretDown},
	"command":           {"MagnifyingGlass": icons.MagnifyingGlass},
	"data-table":        {"ArrowDown": icons.ArrowDown, "ArrowUp": icons.ArrowUp, "ArrowsDownUp": icons.ArrowsDownUp, "CaretDown": icons.CaretDown, "CaretUpDown": icons.CaretUpDown, "Check": icons.Check, "PlusCircle": icons.PlusCircle, "X": icons.X},
	"date-range-picker": {"CalendarBlank": icons.CalendarBlank, "CaretLeft": icons.CaretLeft, "CaretRight": icons.CaretRight},
	"date-time-picker":  {"CalendarBlank": icons.CalendarBlank, "CaretLeft": icons.CaretLeft, "CaretRight": icons.CaretRight},
	"form-dialog":       {"X": icons.X},
	"keyboard":          {"ArrowFatLineUp": icons.ArrowFatLineUp, "ArrowFatUp": icons.ArrowFatUp, "Backspace": icons.Backspace, "CaretDown": icons.CaretDown},
	"number-input":      {"Minus": icons.Minus, "Plus": icons.Plus},
	"password":          {"Eye": icons.Eye, "EyeSlash": icons.EyeSlash},
	"rating":            {"Star": icons.Star},
	"select":            {"CaretDown": icons.CaretDown},
	"split-button":      {"CaretDown": icons.CaretDown},
	"theme-toggle":      {"Moon": icons.Moon, "Sun": icons.Sun},
	"toast":             {"X": icons.X},
	"tree":              {"CaretRight": icons.CaretRight},
}

// InnerIcons answers, by the name of the Kyse component each widget pairs with,
// the Kyse function names of the icons that widget draws inside itself, sorted.
//
// It exists so that a test can put it beside what Kyse's source says, and so
// that a product can ask which of the set a screen already carries. A widget
// draws nothing that is not listed.
func InnerIcons() map[string][]string {
	out := make(map[string][]string, len(innerIcons))
	for component, set := range innerIcons {
		names := make([]string, 0, len(set))
		for name := range set {
			names = append(names, name)
		}
		sort.Strings(names)
		out[component] = names
	}
	return out
}

// innerIcon is the icon a component draws inside itself, or the zero icon when the
// pair is not in the table.
//
// The tick is the one icon drawn without its shade, because Kyse draws it so
// wherever it puts one: a tick in a menu row hides the lighter path, and the
// tick of a checkbox is a stroke with no second half at all.
func innerIcon(component, name string) icons.Icon {
	build, found := innerIcons[component][name]
	if !found {
		return icons.Icon{}
	}
	if name == "Check" {
		return build().WithoutShade()
	}
	return build()
}

// The sides Kyse's stylesheet gives the icons, in points. size-4 is the one
// nearly every icon takes; the tree's marker and the breadcrumb's separator are
// size-3.5, and the rating's star is size-6.
const (
	innerSide      = 16
	innerSideSmall = 14
	innerSideStar  = 24
)

// drawInner paints an inner icon on a square of this many pixels at the
// current origin and turns it by quarter turns clockwise about its centre, the
// way the stylesheet turns the accordion's caret half a turn and the tree's a
// quarter. The room it takes is the square.
func drawInner(c ayra.Context, icon icons.Icon, side int, ink color.NRGBA, quarters int) ayra.Dimensions {
	if icon.IsZero() || side <= 0 {
		return ayra.Dimensions{}
	}
	quarters = ((quarters % 4) + 4) % 4
	if quarters != 0 {
		half := float32(side) / 2
		turn := op.Affine(f32.AffineId().Rotate(f32.Pt(half, half), float32(quarters)*1.5707964)).Push(c.Ops)
		defer turn.Pop()
	}
	DrawIcon(c, icon, side, ink)
	return ayra.Dimensions{Size: image.Pt(side, side)}
}

// innerButton is a button that holds one icon at the side Kyse's stylesheet gives
// it, with the label kept as the name a screen reader says.
func innerButton(c ayra.Context, state *Button, icon icons.Icon, name string, variant Variant, disabled bool) ayra.Dimensions {
	c.Constraints.Min = image.Point{}
	return ButtonProps{
		ComponentProps: ComponentProps{Style: theme.Style{
			Font:    theme.Font{Size: theme.Pt(innerSide)},
			Padding: theme.Uniform(6),
		}},
		Label: name, Icon: icon, IconOnly: true, Variant: variant, Size: Icon, Disabled: disabled,
	}.Layout(c, state)
}

// innerPixels is a side in points at the density of the context.
func innerPixels(c ayra.Context, points int) int {
	return c.Dp(unit.Dp(points))
}

// declared reports whether a component has the icon in the table. It is for the
// tests that read the source.
func declared(component, name string) bool {
	_, found := innerIcons[component][name]
	return found
}

// declaredComponents are the keys of the table, sorted.
func declaredComponents() []string {
	keys := make([]string, 0, len(innerIcons))
	for key := range innerIcons {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
