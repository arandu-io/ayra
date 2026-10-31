// Package icons is the Phosphor icon set, one function per icon, under the names
// Kyse's icons package gives them.
//
//	widget.IconProps{Icon: icons.Trash(), Label: "Delete this reading"}.Layout(c)
//
// The set is the same on both halves of Arandu: the same Phosphor icons, the same
// duotone weight, the same function names, so that a screen written for the
// browser is written the same way here. It is the same by construction and not
// by care: the files in this package are generated from the ones in Kyse
// (internal/icongen), and a test compares the two names for names whenever Kyse
// is beside this repository.
//
// The shape is Kyse's, and for its reason. The set is 1512 icons and an
// application uses thirty; one function per icon is what lets the linker
// drop the others, and a lookup by name -- which retains the whole catalogue --
// is a separate package, [github.com/arandu-io/ayra/icons/lookup], that an
// application imports on purpose.
//
// What an icon draws as is decided by the widget that draws it: a filled path
// on a 256-unit square, in the colour of the text around it, with the lighter
// half at a fifth of its strength.
package icons

//go:generate go run ../internal/icongen ../../kyse/icons . lookup

// Icon is the geometry of one icon: two paths, the lighter shade and the body,
// in the SVG path syntax of a 256 by 256 square.
//
// It holds data and draws nothing. The widget that draws it parses the paths
// once and keeps the result, so an icon drawn on every frame costs no parsing.
type Icon struct {
	shade string
	body  string
}

// Shade is the path of the lighter half, empty when the icon has none.
func (i Icon) Shade() string { return i.shade }

// Body is the path of the main half.
func (i Icon) Body() string { return i.body }

// WithoutShade is the icon with its lighter half left out, for the places Kyse
// hides it: the tick in a menu row is drawn with `[&_path[opacity]]:hidden`, and
// the shade of Check is a whole square that would turn a tick into a box.
func (i Icon) WithoutShade() Icon { return Icon{body: i.body} }

// IsZero reports whether the icon has no geometry, which is what the zero value
// is and what an icon an application forgot to set draws as: nothing.
func (i Icon) IsZero() bool { return i.body == "" && i.shade == "" }

// Size is the side of the square the paths are drawn on.
const Size = 256
