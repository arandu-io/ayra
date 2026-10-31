package css_test

import (
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/theme/css"
)

func fixture(t *testing.T) *css.Sheet {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "product.css"))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := css.Parse(string(body))
	if err != nil {
		t.Fatal(err)
	}
	return sheet
}

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 255}
}

// TestTheProductStylesheetBecomesTheProductsTheme is the point of the package:
// the values a product wrote in its stylesheet are the values it draws with,
// in both schemes, with nothing typed twice.
func TestTheProductStylesheetBecomesTheProductsTheme(t *testing.T) {
	sheet := fixture(t)
	light, dark := sheet.Theme(theme.Light), sheet.Theme(theme.Dark)

	for _, c := range []struct {
		name      string
		got, want color.NRGBA
	}{
		{"light background", light.Colours.Background, rgb(0xf4f4f4)},
		{"light card", light.Colours.Card, rgb(0xffffff)},
		{"light primary", light.Colours.Primary, rgb(0x222222)},
		{"light border", light.Colours.Border, rgb(0xd6d6d6)},
		{"light input", light.Colours.Input, rgb(0xc2c2c2)},
		{"dark background", dark.Colours.Background, rgb(0x000000)},
		{"dark card", dark.Colours.Card, rgb(0x111111)},
		{"dark primary", dark.Colours.Primary, rgb(0xffffff)},
		{"dark border", dark.Colours.Border, rgb(0x2a2a2a)},
	} {
		if c.got != c.want {
			t.Errorf("%s is %+v, want %+v", c.name, c.got, c.want)
		}
	}
	if light.Metrics.Radius != 6 || dark.Metrics.Radius != 6 {
		t.Errorf("the radius is %v and %v, want 6: 0.375rem is six points", light.Metrics.Radius, dark.Metrics.Radius)
	}
}

// TestAnAliasIsResolvedInTheSchemeItIsReadIn fixes the cascade: --tone-read is
// a hex value in the light scheme and var(--brand-neon) in the dark one, and a
// reader that froze the alias where it was first seen would carry the light
// colour into the dark theme.
func TestAnAliasIsResolvedInTheSchemeItIsReadIn(t *testing.T) {
	sheet := fixture(t)
	if got, want := sheet.Theme(theme.Light).Token("tone-read"), rgb(0x00875a); got != want {
		t.Errorf("light tone-read is %+v, want %+v", got, want)
	}
	if got, want := sheet.Theme(theme.Dark).Token("tone-read"), rgb(0x00ffa3); got != want {
		t.Errorf("dark tone-read is %+v, want %+v", got, want)
	}
}

func TestTheFontOfTheStylesheetBecomesTheTypeface(t *testing.T) {
	th := fixture(t).Theme(theme.Light)
	if th.Type.Monospace != "JetBrains Mono" {
		t.Errorf("the monospace face is %q, want the first family of --font-mono", th.Type.Monospace)
	}
	if th.Type.Sans != "" {
		t.Errorf("the sans face is %q, but the stylesheet declares none", th.Type.Sans)
	}
}

// TestWhatIsNotATokenIsSteppedOver covers the other half of reading a real
// stylesheet: it is mostly things that are not tokens.
func TestWhatIsNotATokenIsSteppedOver(t *testing.T) {
	sheet, err := css.Parse(`
/* a comment with a { brace and a --fake: #ff0000; declaration */
@import "other.css";
@layer base {
  :root { --primary: rgb(10 20 30 / 50%); --inset: clamp(1rem, 2vw, 3rem); }
}
@media (min-width: 40rem) { :root { --primary: #ffffff; } }
:root[data-theme="blue"] { --primary: #0000ff; }
.card { --primary: #00ff00; background: url("data:image/svg+xml;utf8,<svg>{;}</svg>"); }
:root { --ring: hsl(0 100% 50%); --background: #abc; --muted: var(--missing, #123456); }
`)
	if err != nil {
		t.Fatal(err)
	}
	th := sheet.Theme(theme.Light)

	if want := (color.NRGBA{R: 10, G: 20, B: 30, A: 128}); th.Colours.Primary != want {
		t.Errorf("primary is %+v, want %+v: only the bare :root counts, in or out of @layer", th.Colours.Primary, want)
	}
	if want := rgb(0xff0000); th.Colours.Ring != want {
		t.Errorf("ring is %+v, want %+v", th.Colours.Ring, want)
	}
	if want := rgb(0xaabbcc); th.Colours.Background != want {
		t.Errorf("background is %+v, want %+v", th.Colours.Background, want)
	}
	if want := rgb(0x123456); th.Colours.Muted != want {
		t.Errorf("muted is %+v, want %+v: the fallback of var()", th.Colours.Muted, want)
	}
	if th.HasToken("fake") {
		t.Error("a declaration inside a comment became a token")
	}
}

func TestAStylesheetWithNoTokensIsRefused(t *testing.T) {
	if _, err := css.Parse(`.card { color: red; }`); err == nil {
		t.Error("a stylesheet with no custom property was accepted, and would draw the default theme under the product's name")
	}
}

func TestTokenPrefixesLimitWhatIsCarried(t *testing.T) {
	th := fixture(t).Theme(theme.Light, css.Options{TokenPrefixes: []string{"tone-"}})
	for name := range th.Styles.Tokens {
		if !strings.HasPrefix(name, "tone-") {
			t.Errorf("the token %q was carried, and only tone- was asked for", name)
		}
	}
	if len(th.Styles.Tokens) == 0 {
		t.Error("no token was carried")
	}
}

// TestTheGeneratedSourceIsTheRuntimeTheme proves the two paths agree: the Go
// the generator writes, built and run, answers the same theme as reading the
// stylesheet does. It is also compared with the committed golden, so a change
// to the generator is a diff somebody reads.
func TestTheGeneratedSourceIsTheRuntimeTheme(t *testing.T) {
	sheet := fixture(t)
	options := css.Options{TokenPrefixes: []string{"tone-", "box-"}}

	source, err := css.Generate(sheet, "brand", "product.css", options)
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "theme_gen.go.golden")
	if os.Getenv("AYRA_UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, source, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(source) {
		t.Error("the generated source is not the committed one; set AYRA_UPDATE_GOLDEN=1 to rewrite it after reading the difference")
	}

	// Built inside the module so the imports resolve against the real go.mod.
	root := filepath.Join("..", "..", ".themegen-compile-check")
	t.Cleanup(func() { os.RemoveAll(root) })
	os.RemoveAll(root)
	if err := os.MkdirAll(filepath.Join(root, "brand"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixturePath, _ := filepath.Abs(filepath.Join("testdata", "product.css"))
	if err := os.WriteFile(filepath.Join(root, "brand", "theme_gen.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	main := `package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/arandu-io/ayra/theme"
	"github.com/arandu-io/ayra/theme/css"
	"github.com/arandu-io/ayra/.themegen-compile-check/brand"
)

func main() {
	body, _ := os.ReadFile(` + "`" + fixturePath + "`" + `)
	sheet, err := css.Parse(string(body))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, s := range []theme.Scheme{theme.Light, theme.Dark} {
		got, want := brand.Theme(s), sheet.Theme(s, css.Options{TokenPrefixes: []string{"tone-", "box-"}})
		if !reflect.DeepEqual(got, want) {
			fmt.Println("the generated theme differs from the runtime one in", s)
			os.Exit(1)
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	run := exec.Command("go", "run", ".")
	run.Dir = root
	run.Env = append(os.Environ(), "GOWORK=off")
	if output, err := run.CombinedOutput(); err != nil {
		t.Errorf("the generated source does not build and agree with the runtime theme:\n%s", output)
	}
}
