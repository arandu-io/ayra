// Command tokengen turns the vendored stylesheet's custom properties into Go.
//
// It exists so that the palette has one source. A palette typed twice drifts,
// and the drift is invisible: both halves compile, both draw, and the only
// symptom is a button that is a shade off from the same button on the web.
//
// The output is committed. A generator whose output is not in the tree is a
// build step every consumer has to run, and this module is imported rather
// than built -- so what is imported has to be the finished file. A test
// re-runs the generator and refuses a difference, in both directions: a
// stylesheet edited without regenerating fails, and a generated file edited by
// hand fails too.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// property matches one custom property declaration and its value.
var property = regexp.MustCompile(`--([a-z0-9-]+):\s*([^;]+);`)

// oklchCall matches the colour function, with an optional alpha after a slash.
var oklchCall = regexp.MustCompile(`^oklch\(\s*([0-9.]+)\s+([0-9.]+)\s+([0-9.]+)\s*(?:/\s*([0-9.]+)(%?)\s*)?\)$`)

// hexColour matches the three-byte form, which the chart colours use.
var hexColour = regexp.MustCompile(`^#([0-9a-fA-F]{6})$`)

// remLength matches a length in rem, which is how the radius is written.
var remLength = regexp.MustCompile(`^([0-9.]+)rem$`)

// pxLength matches a length already in pixels.
var pxLength = regexp.MustCompile(`^([0-9.]+)px$`)

// alias matches a property whose value is another property.
var alias = regexp.MustCompile(`^var\(--([a-z0-9-]+)\)$`)

// notColours are the properties this generator refuses to carry, by name,
// with the reason.
//
// They are listed rather than skipped by shape, because a value this
// generator does not recognise has to stop it: silently dropping one is how a
// field ends up carrying the zero colour and drawing black on black. Naming
// them is the difference between a decision and an omission.
//
// The three icons are inline SVG in a data URI. They are the marks a select
// and a checkbox draw, and on this side those are paths in the drawing layer
// rather than images fetched by a stylesheet -- so carrying the bytes across
// would be carrying a second copy of a mark that is already drawn.
var notColours = map[string]string{
	"chevron-down-icon":    "an inline SVG mark, drawn as a path on this side",
	"chevron-down-icon-50": "an inline SVG mark, drawn as a path on this side",
	"check-icon":           "an inline SVG mark, drawn as a path on this side",
}

// scheme is one block of the stylesheet: the properties under one selector.
type scheme struct {
	name    string
	colours map[string]Colour
	lengths map[string]float64
	// aliases are the properties whose value is another property, left
	// unresolved. A stylesheet resolves one at the point of use, not at the
	// point of declaration, so an alias written once under the light selector
	// answers with the dark colour inside a dark page. Freezing it where it is
	// written would carry the light colour into the dark scheme.
	aliases map[string]string
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: tokengen <stylesheet> <output>")
		os.Exit(2)
	}

	source, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "tokengen:", err)
		os.Exit(1)
	}

	light, err := read(string(source), ":root")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tokengen:", err)
		os.Exit(1)
	}
	dark, err := read(string(source), ".dark")
	if err != nil {
		fmt.Fprintln(os.Stderr, "tokengen:", err)
		os.Exit(1)
	}

	// The dark block overrides; what it does not name it inherits, which is
	// what a cascade does and what makes the two blocks the size they are.
	for name, colour := range light.colours {
		if _, overridden := dark.colours[name]; !overridden {
			dark.colours[name] = colour
		}
	}
	for name, target := range light.aliases {
		if _, overridden := dark.aliases[name]; !overridden {
			dark.aliases[name] = target
		}
	}

	// Aliases resolve last, and each against its own scheme. That is the whole
	// reason they are not resolved where they are read.
	if err := resolve(&light); err != nil {
		fmt.Fprintln(os.Stderr, "tokengen:", err)
		os.Exit(1)
	}
	if err := resolve(&dark); err != nil {
		fmt.Fprintln(os.Stderr, "tokengen:", err)
		os.Exit(1)
	}

	// A property in one scheme and not the other is a hole nothing else would
	// report: the field exists, carries the zero colour, and draws black on
	// black in whichever scheme forgot it.
	for name := range light.colours {
		if _, found := dark.colours[name]; !found {
			fmt.Fprintf(os.Stderr, "tokengen: %q is declared in the light scheme and not in the dark one\n", name)
			os.Exit(1)
		}
	}
	for name := range dark.colours {
		if _, found := light.colours[name]; !found {
			fmt.Fprintf(os.Stderr, "tokengen: %q is declared in the dark scheme and not in the light one\n", name)
			os.Exit(1)
		}
	}

	out, err := format.Source(emit(light, dark))
	if err != nil {
		fmt.Fprintln(os.Stderr, "tokengen: formatting the generated source:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "tokengen:", err)
		os.Exit(1)
	}
}

// read collects the properties declared under one selector.
//
// The block is found by the selector and its closing brace, which is enough
// because the stylesheet this reads is a flat list of declarations with no
// nesting. A stylesheet that grew a nested rule would end the block early, and
// the missing-property check above is what would report it.
func read(source, selector string) (scheme, error) {
	s := scheme{
		name:    selector,
		colours: map[string]Colour{},
		lengths: map[string]float64{},
		aliases: map[string]string{},
	}

	start := strings.Index(source, selector+" {")
	if start < 0 {
		return s, fmt.Errorf("the stylesheet declares no %s block", selector)
	}
	rest := source[start:]
	end := strings.Index(rest, "\n}")
	if end < 0 {
		return s, fmt.Errorf("the %s block is not closed", selector)
	}

	for _, match := range property.FindAllStringSubmatch(rest[:end], -1) {
		name, value := match[1], strings.TrimSpace(match[2])

		if call := oklchCall.FindStringSubmatch(value); call != nil {
			l, _ := strconv.ParseFloat(call[1], 64)
			c, _ := strconv.ParseFloat(call[2], 64)
			h, _ := strconv.ParseFloat(call[3], 64)
			alpha := 1.0
			if call[4] != "" {
				alpha, _ = strconv.ParseFloat(call[4], 64)
				if call[5] == "%" {
					alpha /= 100
				}
			}
			s.colours[name] = oklchToRGB(l, c, h, alpha)
			continue
		}

		if hex := hexColour.FindStringSubmatch(value); hex != nil {
			n, _ := strconv.ParseUint(hex[1], 16, 32)
			s.colours[name] = Colour{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
			continue
		}

		if reason, refused := notColours[name]; refused {
			_ = reason
			continue
		}

		if value == "transparent" {
			s.colours[name] = Colour{}
			continue
		}

		if target := alias.FindStringSubmatch(value); target != nil {
			s.aliases[name] = target[1]
			continue
		}

		if length := pxLength.FindStringSubmatch(value); length != nil {
			px, _ := strconv.ParseFloat(length[1], 64)
			s.lengths[name] = px
			continue
		}

		if length := remLength.FindStringSubmatch(value); length != nil {
			rem, _ := strconv.ParseFloat(length[1], 64)
			// One rem is sixteen points, which is what the browser default is
			// and what the web half of this palette was drawn against.
			s.lengths[name] = rem * 16
			continue
		}

		// Anything else is a value this generator does not know how to carry
		// across, and carrying it wrong is worse than not carrying it.
		return s, fmt.Errorf("%s: --%s is %q, which is not a colour or a length in rem", selector, name, value)
	}

	if len(s.colours) == 0 {
		return s, fmt.Errorf("the %s block declares no colour", selector)
	}
	return s, nil
}

// resolve replaces each alias with the colour it names, in its own scheme.
func resolve(s *scheme) error {
	for name, target := range s.aliases {
		colour, found := s.colours[target]
		if !found {
			return fmt.Errorf("%s: --%s points at --%s, which nothing declares", s.name, name, target)
		}
		s.colours[name] = colour
	}
	return nil
}

// emit writes the Go source for both schemes.
func emit(light, dark scheme) []byte {
	names := make([]string, 0, len(light.colours))
	for name := range light.colours {
		names = append(names, name)
	}
	sort.Strings(names)

	lengths := make([]string, 0, len(light.lengths))
	for name := range light.lengths {
		lengths = append(lengths, name)
	}
	sort.Strings(lengths)

	var b bytes.Buffer
	b.WriteString("// Code generated by internal/tokengen from theme/basecoat/base.css. DO NOT EDIT.\n\n")
	b.WriteString("package theme\n\n")
	b.WriteString("import \"image/color\"\n\n")

	b.WriteString("// Palette is the colour of every named surface, in one scheme.\n")
	b.WriteString("//\n")
	b.WriteString("// The names are the ones the stylesheet declares, so a colour called the\n")
	b.WriteString("// same thing on both halves of the product is the same colour.\n")
	b.WriteString("type Palette struct {\n")
	for _, name := range names {
		fmt.Fprintf(&b, "\t%s color.NRGBA\n", exported(name))
	}
	b.WriteString("}\n\n")

	b.WriteString("// Metrics are the lengths the stylesheet declares, in points.\n")
	b.WriteString("type Metrics struct {\n")
	for _, name := range lengths {
		fmt.Fprintf(&b, "\t%s float32\n", exported(name))
	}
	b.WriteString("}\n\n")

	writePalette(&b, "lightPalette", light, names)
	writePalette(&b, "darkPalette", dark, names)
	writeMetrics(&b, "metrics", light, lengths)

	return b.Bytes()
}

func writePalette(b *bytes.Buffer, ident string, s scheme, names []string) {
	fmt.Fprintf(b, "var %s = Palette{\n", ident)
	for _, name := range names {
		c := s.colours[name]
		fmt.Fprintf(b, "\t%s: color.NRGBA{R: %d, G: %d, B: %d, A: %d},\n", exported(name), c.R, c.G, c.B, c.A)
	}
	b.WriteString("}\n\n")
}

func writeMetrics(b *bytes.Buffer, ident string, s scheme, names []string) {
	fmt.Fprintf(b, "var %s = Metrics{\n", ident)
	for _, name := range names {
		fmt.Fprintf(b, "\t%s: %g,\n", exported(name), s.lengths[name])
	}
	b.WriteString("}\n")
}

// exported turns a custom property name into a Go field name.
//
//	--card-foreground -> CardForeground
//	--chart-1         -> Chart1
func exported(name string) string {
	parts := strings.Split(name, "-")
	var out strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		out.WriteString(strings.ToUpper(part[:1]))
		out.WriteString(part[1:])
	}
	return out.String()
}
