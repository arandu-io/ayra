// Package css makes a [theme.Theme] from the stylesheet of the product that
// draws with it.
//
// The palette of the default theme is generated from Basecoat, the stylesheet
// the web half of Arandu already uses. A product has its own on top of it -- a
// custom.css that sets --background, --primary, --radius and a few colours of
// its own -- and the native half of the product has to look like the web half
// of the same product, not like Basecoat. Copying the values by hand is how the
// two drift: both compile, both draw, and the only symptom is a button a shade
// off the same button on the page.
//
// So the stylesheet is read. [Parse] takes the product's CSS, [Sheet.Theme]
// answers the theme for a scheme at run time, and [Generate] writes the same
// theme as Go source for a product that would rather commit the result than read
// a file when it starts. Both read the same things:
//
//   - the custom properties declared under :root and under @theme, which are the
//     light scheme, and under .dark, which overrides them in the dark one --
//     later declarations win, as they do in the cascade, and var(--x) is
//     resolved against the scheme it is read in;
//   - a property named like a palette colour (--background, --primary,
//     --sidebar-ring) replaces that colour, and any other colour is carried as
//     a token, [theme.Theme.Token], for the variants and the widgets of the
//     product;
//   - --radius and the other lengths the default metrics name;
//   - --font-sans and --font-mono, whose first family becomes the typeface of
//     the scale, to be carried by the fonts the window is opened with.
//
// What it does not read is everything else in the stylesheet: rules, utilities,
// a custom property whose value is not a colour or a length it knows. Those are
// skipped, not refused -- a product stylesheet is thousands of lines of which
// the tokens are a few dozen, and a reader that stopped at the first
// unfamiliar value could not read it.
package css

import (
	"fmt"
	"image/color"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arandu-io/ayra/internal/colour"
	"github.com/arandu-io/ayra/theme"
)

// Sheet is the tokens of one stylesheet.
type Sheet struct {
	light, dark declarations
}

// declarations are the custom properties of a scheme, in the order they were
// written, the last of a name winning.
type declarations map[string]string

// Options choose what is carried beyond the palette.
type Options struct {
	// TokenPrefixes limits the tokens carried to the ones whose name starts with
	// one of these ("tone-", "box-"). Nil carries every colour the palette has
	// no field for, which for a large stylesheet is hundreds.
	TokenPrefixes []string
}

// Parse reads the tokens of one or more stylesheets. Later sources win over
// earlier ones, as later rules do, so the product's file is passed after the
// ones it overrides.
func Parse(sources ...string) (*Sheet, error) {
	s := &Sheet{light: declarations{}, dark: declarations{}}
	for _, source := range sources {
		if err := s.read(source); err != nil {
			return nil, err
		}
	}
	if len(s.light) == 0 && len(s.dark) == 0 {
		return nil, fmt.Errorf("ayra/theme/css: the stylesheet declares no custom property under :root, @theme or .dark")
	}
	return s, nil
}

// Theme answers the theme of a scheme: the default one with the stylesheet's
// tokens laid over it.
func (s *Sheet) Theme(scheme theme.Scheme, options ...Options) theme.Theme {
	var o Options
	if len(options) > 0 {
		o = options[0]
	}

	t := theme.New(scheme)
	merged := declarations{}
	for name, value := range s.light {
		merged[name] = value
	}
	if scheme == theme.Dark {
		for name, value := range s.dark {
			merged[name] = value
		}
	}

	palette := reflect.ValueOf(&t.Colours).Elem()
	metrics := reflect.ValueOf(&t.Metrics).Elem()

	var resolve func(value string, depth int) (color.NRGBA, bool)
	resolve = func(value string, depth int) (color.NRGBA, bool) {
		return resolveColour(value, func(name string) (color.NRGBA, bool) {
			if raw, found := merged[name]; found {
				return resolve(raw, depth+1)
			}
			// A property the product never declared is the default theme's.
			if field := palette.FieldByName(exported(name)); field.IsValid() {
				return field.Interface().(color.NRGBA), true
			}
			return color.NRGBA{}, false
		}, depth)
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := merged[name]

		if name == "font-sans" || name == "font-mono" {
			if face := firstFamily(value); face != "" {
				if name == "font-sans" {
					t.Type.Sans = face
				} else {
					t.Type.Monospace = face
				}
			}
			continue
		}

		if points, ok := resolveLength(value); ok {
			if field := metrics.FieldByName(exported(name)); field.IsValid() && field.Kind() == reflect.Float32 {
				field.SetFloat(points)
			}
			continue
		}

		c, ok := resolve(value, 0)
		if !ok {
			continue
		}
		if field := palette.FieldByName(exported(name)); field.IsValid() && field.Type() == reflect.TypeOf(color.NRGBA{}) {
			field.Set(reflect.ValueOf(c))
			continue
		}
		if wanted(name, o.TokenPrefixes) {
			if t.Styles == nil {
				t.Styles = &theme.Styles{Tokens: map[string]color.NRGBA{}}
			}
			t.Styles.Tokens[name] = c
		}
	}
	return t
}

// Func is the sheet as the function [shell.Config.Theme] and [frame.Options.Theme]
// take.
func (s *Sheet) Func(options ...Options) func(theme.Scheme) theme.Theme {
	return func(scheme theme.Scheme) theme.Theme { return s.Theme(scheme, options...) }
}

func wanted(name string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// exported turns a custom property name into the Go field that carries it.
//
//	--card-foreground -> CardForeground
//	--chart-1         -> Chart1
func exported(name string) string {
	var out strings.Builder
	for _, part := range strings.Split(name, "-") {
		if part == "" {
			continue
		}
		out.WriteString(strings.ToUpper(part[:1]))
		out.WriteString(part[1:])
	}
	return out.String()
}

var (
	hexRe   = regexp.MustCompile(`^#([0-9a-fA-F]{3,8})$`)
	varRe   = regexp.MustCompile(`^var\(\s*--([A-Za-z0-9_-]+)\s*(?:,\s*(.+))?\)$`)
	callRe  = regexp.MustCompile(`^([a-z]+)\(\s*(.*?)\s*\)$`)
	lengthR = regexp.MustCompile(`^(-?[0-9.]+)(px|rem)$`)
	generic = map[string]bool{
		"ui-sans-serif": true, "ui-serif": true, "ui-monospace": true, "ui-rounded": true,
		"system-ui": true, "-apple-system": true, "sans-serif": true, "serif": true,
		"monospace": true, "cursive": true, "fantasy": true, "inherit": true,
		"initial": true, "unset": true, "blinkmacsystemfont": true,
	}
)

// maxDepth bounds the chain of var() references, so that a cycle is an
// unresolved token and not an endless loop.
const maxDepth = 16

// resolveColour reads a CSS colour value. A value it does not understand --
// color-mix, a gradient, a keyword that is not transparent -- is not a colour
// this reader can carry, and answers false.
func resolveColour(value string, lookup func(string) (color.NRGBA, bool), depth int) (color.NRGBA, bool) {
	value = strings.TrimSpace(value)
	if depth > maxDepth {
		return color.NRGBA{}, false
	}

	if value == "transparent" {
		return color.NRGBA{}, true
	}

	if m := hexRe.FindStringSubmatch(value); m != nil {
		return parseHex(m[1])
	}

	if m := varRe.FindStringSubmatch(value); m != nil {
		if lookup != nil {
			if c, ok := lookup(m[1]); ok {
				return c, true
			}
		}
		if m[2] != "" {
			return resolveColour(m[2], lookup, depth+1)
		}
		return color.NRGBA{}, false
	}

	if m := callRe.FindStringSubmatch(value); m != nil {
		args := splitArgs(m[2])
		switch m[1] {
		case "oklch":
			if len(args) < 3 {
				return color.NRGBA{}, false
			}
			l, okL := number(args[0], 1)
			c, okC := number(args[1], 0.4)
			h, okH := number(strings.TrimSuffix(args[2], "deg"), 0)
			if !okL || !okC || !okH {
				return color.NRGBA{}, false
			}
			alpha := 1.0
			if len(args) > 3 {
				a, ok := number(args[3], 1)
				if !ok {
					return color.NRGBA{}, false
				}
				alpha = a
			}
			out := colour.OKLCH(l, c, h, alpha)
			return color.NRGBA{R: out.R, G: out.G, B: out.B, A: out.A}, true
		case "rgb", "rgba":
			if len(args) < 3 {
				return color.NRGBA{}, false
			}
			var rgb [3]uint8
			for i := 0; i < 3; i++ {
				v, ok := number(args[i], 255)
				if !ok {
					return color.NRGBA{}, false
				}
				rgb[i] = uint8(clamp01(v/255)*255 + 0.5)
			}
			alpha := 1.0
			if len(args) > 3 {
				a, ok := number(args[3], 1)
				if !ok {
					return color.NRGBA{}, false
				}
				alpha = a
			}
			return color.NRGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: uint8(clamp01(alpha)*255 + 0.5)}, true
		case "hsl", "hsla":
			if len(args) < 3 {
				return color.NRGBA{}, false
			}
			h, okH := number(strings.TrimSuffix(args[0], "deg"), 360)
			s, okS := number(args[1], 1)
			l, okL := number(args[2], 1)
			if !okH || !okS || !okL {
				return color.NRGBA{}, false
			}
			alpha := 1.0
			if len(args) > 3 {
				a, ok := number(args[3], 1)
				if !ok {
					return color.NRGBA{}, false
				}
				alpha = a
			}
			r, g, b := hslToRGB(h, clamp01(s), clamp01(l))
			return color.NRGBA{R: r, G: g, B: b, A: uint8(clamp01(alpha)*255 + 0.5)}, true
		}
	}
	return color.NRGBA{}, false
}

// splitArgs splits the inside of a colour function on commas, spaces and the
// slash before an alpha, which are the three ways CSS separates them.
func splitArgs(inside string) []string {
	inside = strings.NewReplacer(",", " ", "/", " ").Replace(inside)
	return strings.Fields(inside)
}

// number reads a number or a percentage; a percentage is that fraction of full.
func number(s string, full float64) (float64, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		return v / 100 * full, err == nil
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}

func parseHex(digits string) (color.NRGBA, bool) {
	switch len(digits) {
	case 3, 4:
		var expanded strings.Builder
		for _, r := range digits {
			expanded.WriteRune(r)
			expanded.WriteRune(r)
		}
		digits = expanded.String()
	case 6, 8:
	default:
		return color.NRGBA{}, false
	}
	n, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	if len(digits) == 6 {
		return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, true
	}
	return color.NRGBA{R: uint8(n >> 24), G: uint8(n >> 16), B: uint8(n >> 8), A: uint8(n)}, true
}

func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	to := func(v float64) uint8 { return uint8(clamp01(v+m)*255 + 0.5) }
	return to(r), to(g), to(b)
}

// resolveLength reads a length in px or rem. One rem is sixteen points, which is
// what the browser default is and what the default palette was drawn against.
func resolveLength(value string) (float64, bool) {
	m := lengthR.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	if m[2] == "rem" {
		v *= 16
	}
	return v, true
}

// firstFamily is the first family of a font-family list, if it names a face and
// not a generic family. A list that opens with ui-sans-serif names no face of
// the product's, and the scale keeps its default.
func firstFamily(list string) string {
	first := strings.TrimSpace(strings.SplitN(list, ",", 2)[0])
	first = strings.Trim(first, `"'`)
	if first == "" || generic[strings.ToLower(first)] || strings.HasPrefix(first, "var(") {
		return ""
	}
	return first
}
