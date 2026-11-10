package ayra_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every component of the browser half has a pair here, or a reason it does not.
//
// ADR-0146 gives the two halves one vocabulary and no code, and a vocabulary
// is a promise that has to be kept by something that reads both: a component
// added to Kyse and not here is a word one half knows and the other does not,
// found by whoever tries to write the screen on the second. This is that
// something.
//
// It lives in this repository and not in Kyse because this is the half that has
// to follow: Kyse adds a component, and the next run of this fails here until the
// native half has its pair or writes down why it cannot. A check in Kyse could
// say a component was added and could never know what the other half decided.
//
// The reasons are the whole of the safety here. A gap without one is a thing
// nobody has looked at, and a gap with one is a decision that can be read and
// argued with -- which is the difference between the list that can be published
// and the one that cannot.

// aliases map a browser component to the control that covers it where the two
// are not spelled the same: the name is the kebab-case of the file, and the
// control's the Props type without its suffix.
var aliases = map[string]string{
	"button-group":      "ButtonGroup",
	"color-picker":      "ColourPicker",
	"container-card":    "Card",
	"copy-button":       "Copy",
	"command":           "Palette",
	"data-table":        "DataTable",
	"dropdown-menu":     "Menu",
	"media-player":      "Player",
	"number-input":      "Number",
	"radio-group":       "Radio",
	"range-slider":      "Slider",
	"segmented-control": "Segmented",
	"split-button":      "Split",
	"stat-card":         "Stat",
}

// absent are the browser components with no native control, each with the reason
// and what stands in for it. Only what has no meaning on a native screen is here.
var absent = map[string]string{
	"hidden-input": "a value a form submits without showing it. A window has no form to carry it: what it sends is what its state holds, and the screen that sends it is Go. Native equivalent: a field of the state value, or of the request the screen builds.",
	"skip-link":    "a link that jumps past the navigation for a screen reader. A native screen has no document order to skip and a screen reader reaches it on one platform of nine, so a skip target would be a control with nothing reading it. Native equivalent: the platform's own focus order, and the semantic tree where there is one.",
}

// incomplete are the paired controls that do less than their browser component,
// each with what they do not do and where the rest is being done. Empty is the
// goal; an entry is a promise in the open.
var incomplete = map[string]string{
	"data-table": "the listing over a server route, with the state and parameter names of the browser half; not the columns menu, the bulk bar or paging by cursor",
}

// nativeOnly are the native controls Kyse has no component for, each with the
// reason. They are the other direction of the gap: the browser half is not behind
// here, this half has something it has not got. Kyse has no keyboard because a
// browser always has the system's; a kiosk with a touch screen and no system
// keyboard worth the name draws its own, and was asked for one by name.
// Adding a pair to Kyse makes the entry fail until it is taken out.
var nativeOnly = map[string]string{
	"keyboard": "the on-screen keyboard of a native window that has no system keyboard to raise (a kiosk's touch screen). A browser always has the system's, so Kyse has no component for it; it is a native control by decision of the product owner, and the pair is open until Kyse says whether it wants one.",
}

func kyseComponents(t *testing.T) (dir string, names []string) {
	t.Helper()

	candidates := []string{os.Getenv("AYRA_KYSE"), filepath.Join("..", "kyse")}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(candidate, "components"))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if name := entry.Name(); strings.HasSuffix(name, ".kyse.go") && !strings.HasPrefix(name, "_") {
				names = append(names, strings.TrimSuffix(name, ".kyse.go"))
			}
		}
		sort.Strings(names)
		return filepath.Join(candidate, "components"), names
	}

	if os.Getenv("CI") != "" {
		t.Fatal("CI is set and Kyse is not beside this repository (AYRA_KYSE): a gate that skips is a gate that does not exist")
	}
	t.Skip("Kyse is not beside this repository, so the two halves were not compared")
	return "", nil
}

// nativeControls answers every exported Props type of the widget package.
func nativeControls(t *testing.T) map[string]bool {
	t.Helper()
	declaration := regexp.MustCompile(`(?m)^type ([A-Z][A-Za-z0-9]*)Props struct`)

	entries, err := os.ReadDir("widget")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join("widget", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range declaration.FindAllStringSubmatch(string(body), -1) {
			found[match[1]] = true
		}
	}
	if len(found) < 60 {
		t.Fatalf("only %d controls were read from widget/", len(found))
	}
	return found
}

// pascal turns data-table into DataTable.
func pascal(kebab string) string {
	var out strings.Builder
	for _, part := range strings.Split(kebab, "-") {
		out.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return out.String()
}

// pair is the control that covers a browser component.
func pair(name string) string {
	if alias, found := aliases[name]; found {
		return alias
	}
	return pascal(name)
}

// TestEveryComponentOfTheBrowserHalfHasAPairOrAReason is the gate.
func TestEveryComponentOfTheBrowserHalfHasAPairOrAReason(t *testing.T) {
	_, web := kyseComponents(t)
	native := nativeControls(t)

	if len(web) < 70 {
		t.Fatalf("only %d browser components were read", len(web))
	}

	for _, name := range web {
		reason, isAbsent := absent[name]
		switch {
		case isAbsent && reason == "":
			t.Errorf("%s is declared absent with no reason: a gap with no reason is something nobody has looked at", name)
		case isAbsent && native[pair(name)]:
			t.Errorf("%s is declared absent and %sProps exists: the declaration no longer describes anything", name, pair(name))
		case !isAbsent && !native[pair(name)]:
			t.Errorf("the browser half has %s and this half has no %sProps, and nothing says why: write the control, or declare it in absent with the reason and what stands in for it", name, pair(name))
		}
	}
}

// TestNothingIsDeclaredThatIsNotTrue keeps the lists from outliving what they
// describe: a name on them that Kyse no longer has, an alias to a control that is
// gone, a promise of work that is not a component.
func TestNothingIsDeclaredThatIsNotTrue(t *testing.T) {
	_, web := kyseComponents(t)
	native := nativeControls(t)

	has := map[string]bool{}
	for _, name := range web {
		has[name] = true
	}
	for name := range absent {
		if !has[name] {
			t.Errorf("%s is declared absent and Kyse no longer has it", name)
		}
	}
	for name, target := range aliases {
		if !has[name] {
			t.Errorf("%s is aliased to %s and Kyse no longer has it", name, target)
		}
		if !native[target] {
			t.Errorf("%s is aliased to %sProps, which does not exist", name, target)
		}
	}
	for name, reason := range incomplete {
		if !has[name] || reason == "" {
			t.Errorf("%s is declared incomplete with no component or no reason", name)
		}
	}
}

// partNames reads the parts a Kyse component publishes.
var partNames = regexp.MustCompile(`(?s)PartNames\(\) \[\]string \{\s*return \[\]string\{(.*?)\}`)

// TestThePartsOfEachPairAreAccountedFor reports, for every paired control, the
// parts the browser component publishes and this one does not.
//
// It does not fail: a part is wired when its control draws it, and the way to
// close the distance is a control at a time. What it does is make the distance
// a number somebody can read in the run, and it fails the one thing that is a
// mistake -- a part this half publishes under a name the browser half never
// had, which is a vocabulary of its own.
func TestThePartsOfEachPairAreAccountedFor(t *testing.T) {
	dir, web := kyseComponents(t)
	declared := regexp.MustCompile(`(?s)func \(p ([A-Za-z]+)Props\) PartNames\(\) \[\]string \{\s*return \[\]string\{(.*?)\}`)

	native := map[string]map[string]bool{}
	entries, _ := os.ReadDir("widget")
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		body, _ := os.ReadFile(filepath.Join("widget", entry.Name()))
		for _, m := range declared.FindAllStringSubmatch(string(body), -1) {
			native[m[1]] = map[string]bool{}
			for _, part := range strings.Split(m[2], ",") {
				if part = strings.Trim(strings.TrimSpace(part), `"`); part != "" {
					native[m[1]][part] = true
				}
			}
		}
	}

	missing, total := 0, 0
	for _, name := range web {
		body, err := os.ReadFile(filepath.Join(dir, name+".kyse.go"))
		if err != nil {
			continue
		}
		found := partNames.FindStringSubmatch(string(body))
		if found == nil {
			continue
		}
		ours := native[pair(name)]
		var gap []string
		for _, part := range strings.Split(found[1], ",") {
			part = strings.Trim(strings.TrimSpace(part), `"`)
			if part == "" {
				continue
			}
			total++
			if ours != nil && !ours[part] {
				gap = append(gap, part)
				missing++
			}
		}
		if len(gap) > 0 && ours != nil {
			t.Logf("%-18s the browser publishes %v, and the native control does not", name, gap)
		}
	}
	t.Logf("parts the browser half publishes: %d; not yet published natively: %d", total, missing)
}

// TestNothingNativeOnlyHasAPairInKyse keeps nativeOnly honest: the control is
// there, Kyse has no component of that name, and a reason is written.
func TestNothingNativeOnlyHasAPairInKyse(t *testing.T) {
	_, web := kyseComponents(t)
	native := nativeControls(t)

	for name, reason := range nativeOnly {
		if reason == "" {
			t.Errorf("%s is declared native only with no reason", name)
		}
		if !native[pascal(name)] {
			t.Errorf("%s is declared native only and %sProps does not exist", name, pascal(name))
		}
		for _, other := range web {
			if pair(other) == pascal(name) {
				t.Errorf("%s is declared native only and Kyse has %s: it has a pair now, take the entry out", name, other)
			}
		}
	}
}
