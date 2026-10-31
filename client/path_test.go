package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/arandu-io/ayra/client"
)

// TestAValueInAPathStaysInItsSegment is a path assembled from something
// somebody typed, or a slug a page sent.
//
// Spliced into text, "../admin/users/7?role=admin#" moved the request to
// another route and added a field the server merges into what it reads from
// the form -- a role nobody chose, on a user nobody meant. A segment is one
// segment whatever its characters are.
func TestAValueInAPathStaysInItsSegment(t *testing.T) {
	var path, raw, role string

	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		path, raw = r.URL.Path, r.URL.EscapedPath()
		role = r.FormValue("role")
		viewData(`{"view":"home","data":null}`)(w, r)
	})

	slug := "../admin/users/7?role=admin#"
	if _, err := c.Post(context.Background(), client.At("teams", slug, "members"), url.Values{"name": {"x"}}); err != nil {
		t.Fatalf("the post was refused: %v", err)
	}

	if role != "" {
		t.Errorf("the slug added a field: role=%q", role)
	}
	if want := "/teams/" + slug + "/members"; path != want {
		t.Errorf("the server saw the route %q, want %q", path, want)
	}
	if want := "/teams/..%2Fadmin%2Fusers%2F7%3Frole=admin%23/members"; raw != want {
		t.Errorf("the request went out as %q, want %q", raw, want)
	}
}

// TestADotSegmentIsRefused keeps the two segments escaping cannot change from
// reaching a server that would read them as a step.
func TestADotSegmentIsRefused(t *testing.T) {
	reached := false
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		reached = true
		viewData(`{"view":"home","data":null}`)(w, r)
	})

	for _, segment := range []string{"", ".", ".."} {
		_, err := c.Get(context.Background(), client.At("teams", segment, "members"))
		if !errors.Is(err, client.ErrSegment) {
			t.Errorf("segment %q answered %v, want a refusal", segment, err)
		}
	}
	if reached {
		t.Error("a refused path reached the server")
	}
}

// TestAQueryArrivesOnlyAsAQuery keeps the one way to send one.
func TestAQueryArrivesOnlyAsAQuery(t *testing.T) {
	var got url.Values
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		viewData(`{"view":"home","data":null}`)(w, r)
	})

	values := url.Values{"page": {"2"}, "q": {"a&b=c"}}
	path := client.At("invoices").Query(values)
	values.Set("page", "9")

	if _, err := c.Get(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if got.Get("page") != "2" || got.Get("q") != "a&b=c" || len(got) != 2 {
		t.Errorf("the server read the query %v", got)
	}
}

// TestAPathSpellsWhatIsSent fixes the text a refusal names a request by.
func TestAPathSpellsWhatIsSent(t *testing.T) {
	for path, want := range map[*client.Path]string{
		ptr(client.At()):                                  "/",
		ptr(client.Path{}):                                "/",
		ptr(client.At("login")):                           "/login",
		ptr(client.At("teams", "a b", "x")):               "/teams/a%20b/x",
		ptr(client.At("a").Query(url.Values{"k": {"v"}})): "/a?k=v",
	} {
		if got := path.String(); got != want {
			t.Errorf("a path spelled %q, want %q", got, want)
		}
	}
}

func ptr(p client.Path) *client.Path { return &p }
