package ayra_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The documents a visitor is pointed at have to be about this repository.
//
// Three of them were another repository's, copied and never edited. The
// security policy sent vulnerability reports to a different project's advisory
// form -- so a reporter would file where nobody here is watching, and the
// embargo clock would start on a report the maintainers never see. The
// contributing guide described a test layout this repository does not have and
// a dependency rule that is not its rule, and pointed contributors at a file in
// a repository they cannot read. The README promised a build with the Go
// toolchain and nothing else, which this project's own CI disproves thirteen
// packages at a time.
//
// None of it failed anything. The project's checklist asks whether each file
// exists, and all three existed.

// thisModule is the repository these documents belong to.
const thisModule = "arandu-io/ayra"

// siblings are the other repositories of this project. A document here naming
// one of them as its own is a document that was copied.
var siblings = []string{
	"arandu-io/framework", "arandu-io/aru", "arandu-io/kyse", "arandu-io/hesape",
	"arandu-io/joaju", "arandu-io/ui", "arandu-io/mcp", "arandu-io/arandu",
	"arandu-io/examples", "arandu-io/package-skeleton", "arandu-io/vscode-arandu",
}

// TestTheSecurityPolicyReportsToThisRepository is the one that matters most.
func TestTheSecurityPolicyReportsToThisRepository(t *testing.T) {
	body := document(t, "SECURITY.md")

	advisory := regexp.MustCompile(`github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/security/advisories`)
	found := advisory.FindAllStringSubmatch(body, -1)
	if len(found) == 0 {
		t.Fatal("the security policy names no advisory form, so there is nowhere to report")
	}

	for _, match := range found {
		if match[1] != thisModule {
			t.Errorf("the security policy sends reports to %s, and this is %s", match[1], thisModule)
		}
	}
}

// TestNoDocumentClaimsToBeAnotherRepository keeps a copied file from describing
// the repository it came from.
func TestNoDocumentClaimsToBeAnotherRepository(t *testing.T) {
	for _, name := range []string{"README.md", "CONTRIBUTING.md", "SECURITY.md", "THIRD_PARTY.md"} {
		body := document(t, name)

		for _, sibling := range siblings {
			if !strings.Contains(body, sibling) {
				continue
			}
			// Naming a sibling is fine -- this project is a family and its
			// documents refer to each other. What is not fine is naming one
			// where the repository names itself: a link to its issues, its
			// advisories, its releases, its own module path.
			for _, owning := range []string{"/security/advisories", "/issues", "/releases", "/pulls", "go get " + sibling} {
				if strings.Contains(body, sibling+owning) || strings.Contains(body, "go get "+sibling) {
					t.Errorf("%s points at %s%s as though this were that repository", name, sibling, owning)
				}
			}
		}
	}
}

// TestNoDocumentSendsAnybodyToARepositoryTheyCannotRead keeps a public document
// from citing a private one.
//
// A contributor following it finds a 404 and no way to tell whether the file
// moved, the link is wrong, or they are not allowed.
func TestNoDocumentSendsAnybodyToARepositoryTheyCannotRead(t *testing.T) {
	// The private repositories of this project. The site is public at its
	// domain and private as a repository, which is why the test looks for the
	// repository spelling rather than the name.
	private := []string{"arandu-io/plans", "arandu-io/docs", "arandu-io/arandu.io", "arandu-io/community.arandu.io", "plans/"}

	for _, name := range []string{"README.md", "CONTRIBUTING.md", "SECURITY.md", "THIRD_PARTY.md"} {
		body := document(t, name)

		for _, repo := range private {
			if strings.Contains(body, repo) {
				t.Errorf("%s sends a reader to %s, which they cannot open", name, repo)
			}
		}
	}
}

// TestTheReadmeSaysWhatLinuxNeeds keeps the install section from promising a
// build this project's own CI disproves.
func TestTheReadmeSaysWhatLinuxNeeds(t *testing.T) {
	readme := document(t, "README.md")

	workflow, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Skipf("no workflow to compare against: %v", err)
	}
	if !strings.Contains(string(workflow), "apt-get install") {
		t.Skip("this project no longer installs system packages to build")
	}

	if !strings.Contains(readme, "Linux") {
		t.Error("the CI installs development packages before it can build, and the README does not say so")
	}
	if strings.Contains(readme, "build with the Go toolchain and nothing else.\nPackaging") {
		t.Error("the README still promises every desktop builds with the Go toolchain alone")
	}
}

// document reads one of the files a visitor is pointed at.
func document(t *testing.T, name string) string {
	t.Helper()

	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("a public repository needs %s: %v", name, err)
	}
	return string(body)
}
