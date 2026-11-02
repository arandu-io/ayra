package ayra_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/arandu-io/ayra/widget"
)

// A native component draws, inside itself, the icons the Kyse component draws.
//
// The pictures a control carries inside itself -- the caret on the accordion, the
// cross on the toast, the eye on the password -- are a decision of the design,
// and a person who learned what one means in the browser reads the same one on
// the phone. widget.InnerIcons answers what each widget draws, by the name of the
// Kyse component it pairs with; this reads Kyse's source for what that component
// draws, and fails when the two disagree.
//
// It lives here, beside parity_test.go, for the same reason: this is the half
// that has to follow. Kyse changes a caret to an arrow and the next run fails
// here until the native half changes with it, or writes down why it cannot.
//
// Three tables hold every exception, and a reason is the whole of each:
//
//	stylesheet  an icon the browser draws from the stylesheet and not from a
//	            component, which Kyse's source therefore cannot say
//	without     an icon Kyse draws that this half does not, with the reason
//	extra       an icon this half draws that Kyse does not, with the reason

// stylesheet are the icons the browser half draws from its stylesheet, as Lucide
// strokes behind a CSS variable, on the components that use the variable. Kyse's
// component source says nothing of them, so they are written down here against the
// icon of Kyse's own set that is the nearest: the native half draws that one.
// theme/basecoat/base.css is the copy of the stylesheet this repository keeps, and
// the test reads the variables out of it.
var stylesheet = map[string]map[string]string{
	// --check-icon is the tick inside a checked box.
	"checkbox": {"Check": "--check-icon"},
	// --check-icon marks the chosen row of a combobox's list.
	"combobox": {"Check": "--check-icon"},
	// --chevron-down-icon-50 is the chevron of a native select.
	"select": {"CaretDown": "--chevron-down-icon-50"},
	// --chevron-down-icon is the chevron at the end of a sidebar group.
	"sidebar": {"CaretDown": "--chevron-down-icon"},
}

// stylesheetIcons says which Lucide icon each variable is, and so which of Kyse's
// the native half draws for it.
var stylesheetIcons = map[string]struct{ lucide, kyse string }{
	"--check-icon":           {"lucide-check", "Check"},
	"--chevron-down-icon":    {"lucide-chevron-down", "CaretDown"},
	"--chevron-down-icon-50": {"lucide-chevron-down", "CaretDown"},
}

// withoutIcons are icons Kyse draws that this half does not, and why.
var withoutIcons = map[string]map[string]string{
	"combobox": {
		"Check": "the list under the field marks no chosen row: choosing one puts its label in the field and closes the list",
	},
	"data-table": {
		"CaretDoubleLeft":  "the footer pages with Pagination, which Kyse draws as words and numbers and not as carets; the native table follows that component",
		"CaretDoubleRight": "the footer pages with Pagination, which Kyse draws as words and numbers and not as carets; the native table follows that component",
		"CaretLeft":        "the footer pages with Pagination, which Kyse draws as words and numbers and not as carets; the native table follows that component",
		"CaretRight":       "the footer pages with Pagination, which Kyse draws as words and numbers and not as carets; the native table follows that component",
		"DotsThree":        "the row's actions are buttons at the end of the row and not a menu behind one",
		"EyeSlash":         "'Hide' is an entry of the menu on a column's heading, which the native table does not have: its columns are hidden and shown from the columns menu",
		"MagnifyingGlass":  "the search inside a filter's menu; a filter's menu here is a list of its options and has no field of its own",
	},
	"password": {
		"CheckCircle": "the requirements checklist, which the native password field does not draw",
		"Circle":      "the requirements checklist, which the native password field does not draw",
	},
	"sidebar": {
		"CaretDown": "the native sidebar is a flat list of entries and has no groups to fold",
	},
	"table": {
		"ArrowDown":    "the plain Table does not sort; the DataTable does, and its sort arrows are listed under data-table",
		"ArrowUp":      "the plain Table does not sort; the DataTable does, and its sort arrows are listed under data-table",
		"ArrowsDownUp": "the plain Table does not sort; the DataTable does, and its sort arrows are listed under data-table",
	},
	"theme-toggle": {
		"Desktop": "the 'system' mode is not in the toggle of this half yet; it is being built on the system-theme branch",
	},
}

// extraIcons are icons this half draws that Kyse does not, and why.
var extraIcons = map[string]map[string]string{
	"carousel": {
		"CaretLeft":  "Kyse draws the steps as the characters of the stylesheet; the native half draws them as the arrows of the set, which are the same shape, so that the step has a name for a screen reader and no word of any language on it",
		"CaretRight": "Kyse draws the steps as the characters of the stylesheet; the native half draws them as the arrows of the set, which are the same shape, so that the step has a name for a screen reader and no word of any language on it",
	},
	"date-time-picker": {
		"CaretLeft":  "the year stepper of the month picker, where Kyse relies on the browser's own month field; the arrows are those of the calendar",
		"CaretRight": "the year stepper of the month picker, where Kyse relies on the browser's own month field; the arrows are those of the calendar",
	},
}

var kyseIcon = regexp.MustCompile(`icons\.([A-Z][A-Za-z0-9]*)\(`)

// kyseIcons answers, by component, the icons the source of each draws inside it.
// A line that is a comment is not a drawing.
func kyseIcons(t *testing.T) map[string][]string {
	t.Helper()
	dir, names := kyseComponents(t)

	found := map[string][]string{}
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(dir, name+".kyse.go"))
		if err != nil {
			t.Fatal(err)
		}
		set := map[string]bool{}
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, "<svg") {
				t.Errorf("%s draws an svg of its own: this test reads icons.Something() calls and has to learn this form before it can say the native half agrees", name)
			}
			for _, match := range kyseIcon.FindAllStringSubmatch(line, -1) {
				set[match[1]] = true
			}
		}
		for icon := range set {
			found[name] = append(found[name], icon)
		}
		sort.Strings(found[name])
	}
	if len(found) < 15 {
		t.Fatalf("only %d components with icons were read from Kyse", len(found))
	}
	return found
}

func TestEachNativeComponentDrawsTheIconsItsKyseComponentDraws(t *testing.T) {
	fromKyse := kyseIcons(t)
	_, components := kyseComponents(t)
	drawn := widget.InnerIcons()

	for _, name := range components {
		want := map[string]bool{}
		for _, icon := range fromKyse[name] {
			want[icon] = true
		}
		for icon := range stylesheet[name] {
			want[icon] = true
		}
		for icon := range withoutIcons[name] {
			delete(want, icon)
		}
		for icon := range extraIcons[name] {
			want[icon] = true
		}

		got := map[string]bool{}
		for _, icon := range drawn[name] {
			got[icon] = true
		}

		for icon := range want {
			if !got[icon] {
				t.Errorf("%s: Kyse draws %s and the native half does not. Draw it, or say in withoutIcons why it cannot", name, icon)
			}
		}
		for icon := range got {
			if !want[icon] {
				t.Errorf("%s: the native half draws %s and Kyse does not. Remove it, or say in extraIcons why it stays", name, icon)
			}
		}
	}

	for name := range drawn {
		if !slices.Contains(components, name) {
			t.Errorf("widget.InnerIcons names %q, which is not a component of Kyse", name)
		}
	}
}

func TestNoExceptionOfTheIconsIsStale(t *testing.T) {
	fromKyse := kyseIcons(t)
	drawn := widget.InnerIcons()

	for name, set := range withoutIcons {
		for icon, reason := range set {
			if reason == "" {
				t.Errorf("%s: withoutIcons says nothing of why %s is absent", name, icon)
			}
			if !slices.Contains(fromKyse[name], icon) && stylesheet[name][icon] == "" {
				t.Errorf("%s: withoutIcons lists %s, which Kyse does not draw there", name, icon)
			}
			if slices.Contains(drawn[name], icon) {
				t.Errorf("%s: withoutIcons lists %s, which the native half draws", name, icon)
			}
		}
	}
	for name, set := range extraIcons {
		for icon, reason := range set {
			if reason == "" {
				t.Errorf("%s: extraIcons says nothing of why %s stays", name, icon)
			}
			if slices.Contains(fromKyse[name], icon) {
				t.Errorf("%s: extraIcons lists %s, which Kyse draws there too", name, icon)
			}
			if !slices.Contains(drawn[name], icon) {
				t.Errorf("%s: extraIcons lists %s, which the native half does not draw", name, icon)
			}
		}
	}
	for name, set := range stylesheet {
		for icon := range set {
			if slices.Contains(fromKyse[name], icon) {
				t.Errorf("%s: %s is in stylesheet and in Kyse's source; the source is the better witness, so drop the entry", name, icon)
			}
		}
	}
}

func TestTheStylesheetIconsAreTheOnesTheCopyOfTheStylesheetDefines(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("theme", "basecoat", "base.css"))
	if err != nil {
		t.Fatal(err)
	}
	css := string(body)

	for name, set := range stylesheet {
		for icon, variable := range set {
			meaning, found := stylesheetIcons[variable]
			if !found {
				t.Errorf("%s: %s is drawn from %s, which this test does not know the meaning of", name, icon, variable)
				continue
			}
			if meaning.kyse != icon {
				t.Errorf("%s: %s is a %s, whose nearest icon in Kyse's set is %s and not %s", name, variable, meaning.lucide, meaning.kyse, icon)
			}
			declaration := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(variable) + `:.*` + regexp.QuoteMeta(meaning.lucide))
			if !declaration.MatchString(css) {
				t.Errorf("%s: base.css does not define %s as a %s any more", name, variable, meaning.lucide)
			}
		}
	}
}
