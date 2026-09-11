package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra/client"
)

// signing is a server that hands out a session and answers differently
// depending on whether it sees one.
//
// It is the smallest thing that can tell a carried session from a lost one: a
// client that arrives with the cookie gets the page a signed-in person gets,
// and a client that does not gets sent to sign in.
func signing(t *testing.T, cookie *http.Cookie) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, cookie)
			_, _ = w.Write([]byte(`{"view":"home","data":{}}`))
		case "/logout":
			http.SetCookie(w, &http.Cookie{Name: cookie.Name, Value: "", Path: "/", MaxAge: -1})
			_, _ = w.Write([]byte(`{"view":"auth.login","data":{}}`))
		default:
			// The value and not only the name: two servers hand out a cookie
			// called the same thing, and a check that asked only whether one
			// was present would answer "signed in" to the other server's
			// session -- which is the fault a shared store produces.
			sent, err := r.Cookie(cookie.Name)
			if err != nil || sent.Value != cookie.Value {
				_, _ = w.Write([]byte(`{"view":"auth.login","data":{}}`))
				return
			}
			_, _ = w.Write([]byte(`{"view":"home","data":{}}`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// TestASessionIsLostWithoutAStore fixes what the default is, so that the test
// below is measuring the store and not something else.
func TestASessionIsLostWithoutAStore(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/"})

	first, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if page, err := ask(t, first); page.View != "home" {
		t.Fatalf("the same client lost its session within one run: %q (%v)", page.View, err)
	}

	second, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, second); page.View != "auth.login" {
		t.Errorf("a client with no store carried a session across runs: %q", page.View)
	}
}

// TestASessionSurvivesTheApplicationClosing is the whole of what the store is
// for.
//
// A browser tab closing ends a session; an application quitting does not, and
// the person who quit it expects to come back to where they were rather than to
// a password field.
func TestASessionSurvivesTheApplicationClosing(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/"})
	store := fileStore(t)

	first, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	// A second client is the application opened again: same store, nothing
	// carried over in memory.
	second, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	page, err := ask(t, second)
	if err != nil {
		t.Fatal(err)
	}
	if page.View != "home" {
		t.Errorf("the application opened again and was a stranger: %q", page.View)
	}
}

// TestSigningOutIsNotUndoneByRestarting fixes the fault that a store creates:
// a session written down stays written down, and signing out has to reach it.
func TestSigningOutIsNotUndoneByRestarting(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/"})
	store := fileStore(t)

	first, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/logout"); err != nil {
		t.Fatal(err)
	}

	second, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, second); page.View != "auth.login" {
		t.Errorf("signing out came back after a restart: %q", page.View)
	}
}

// TestADeletionSpelledWithAnExpiryIsAlsoADeletion keeps a signed-out session
// from coming back.
//
// A server may end a cookie's life either way: an age of zero or less, or a
// time already past. A jar that read only one spelling would carry a session
// the server had ended, for as long as the other spelling was in use, and the
// person would find themselves signed in again after quitting.
func TestADeletionSpelledWithAnExpiryIsAlsoADeletion(t *testing.T) {
	held := &http.Cookie{Name: "session", Value: "held", Path: "/"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, held)
			_, _ = w.Write([]byte(`{"view":"home","data":{}}`))
		case "/logout":
			http.SetCookie(w, &http.Cookie{
				Name:    held.Name,
				Value:   "",
				Path:    "/",
				Expires: time.Now().Add(-time.Hour),
			})
			_, _ = w.Write([]byte(`{"view":"auth.login","data":{}}`))
		default:
			sent, err := r.Cookie(held.Name)
			if err != nil || sent.Value != held.Value {
				_, _ = w.Write([]byte(`{"view":"auth.login","data":{}}`))
				return
			}
			_, _ = w.Write([]byte(`{"view":"home","data":{}}`))
		}
	}))
	defer server.Close()

	store := fileStore(t)

	first, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/logout"); err != nil {
		t.Fatal(err)
	}

	second, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, second); page.View != "auth.login" {
		t.Errorf("a session deleted with an expiry came back after a restart: %q", page.View)
	}
}

// TestForgettingEndsTheSessionOnThisSide is the other half of signing out: the
// server forgetting is not the client forgetting.
func TestForgettingEndsTheSessionOnThisSide(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/"})
	store := fileStore(t)

	talk, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if err := talk.Forget(); err != nil {
		t.Fatal(err)
	}

	if page, _ := ask(t, talk); page.View != "auth.login" {
		t.Errorf("the client still carries the session it forgot: %q", page.View)
	}

	again, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, again); page.View != "auth.login" {
		t.Errorf("the store still holds the session the client forgot: %q", page.View)
	}
}

// TestAnExpiredSessionIsNotCarried keeps a store from outliving the life the
// server gave what is in it.
func TestAnExpiredSessionIsNotCarried(t *testing.T) {
	server := signing(t, &http.Cookie{
		Name:    "session",
		Value:   "held",
		Path:    "/",
		Expires: time.Now().Add(700 * time.Millisecond),
	})
	store := fileStore(t)

	first, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(900 * time.Millisecond)

	second, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, second); page.View != "auth.login" {
		t.Errorf("a session past its expiry was carried into a new run: %q", page.View)
	}
}

// TestTwoServersAreTwoSessions keeps a development server's session from
// signing somebody in to a real one, and keeps switching between them from
// losing one each time.
func TestTwoServersAreTwoSessions(t *testing.T) {
	development := signing(t, &http.Cookie{Name: "session", Value: "dev", Path: "/"})
	production := signing(t, &http.Cookie{Name: "session", Value: "prod", Path: "/"})
	store := fileStore(t)

	dev, err := client.New(development.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dev.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	// Signing in to the second must not be signing out of the first.
	prod, err := client.New(production.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prod.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	for name, server := range map[string]*httptest.Server{"development": development, "production": production} {
		back, err := client.New(server.URL, client.WithSession(store))
		if err != nil {
			t.Fatal(err)
		}
		if page, _ := ask(t, back); page.View != "home" {
			t.Errorf("the %s session was lost when the other was signed in to: %q", name, page.View)
		}
	}
}

// TestTheSessionFileIsReadableByNobodyElse is the one property a file holding a
// credential has to have.
func TestTheSessionFileIsReadableByNobodyElse(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/"})
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	store, err := client.FileSession("Proof")
	if err != nil {
		t.Skipf("this account has no configuration directory: %v", err)
	}

	talk, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	path := findSession(t, dir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("the session file is %o, and it holds what proves who somebody is", mode)
	}

	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if mode := parent.Mode().Perm(); mode != 0o700 {
		t.Errorf("the directory holding the session is %o", mode)
	}
}

// TestACorruptSessionFileIsNotAnApplicationThatWillNotStart fixes the worst
// thing a session file can do.
func TestACorruptSessionFileIsNotAnApplicationThatWillNotStart(t *testing.T) {
	server := signing(t, &http.Cookie{Name: "session", Value: "held", Path: "/"})
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	store, err := client.FileSession("Proof")
	if err != nil {
		t.Skipf("this account has no configuration directory: %v", err)
	}

	talk, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	path := findSession(t, dir)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	again, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatalf("a byte out of place in the session file stopped the application starting: %v", err)
	}
	if page, _ := ask(t, again); page.View != "auth.login" {
		t.Errorf("a corrupt session file produced a session: %q", page.View)
	}
}

// TestAnApplicationNameCannotEscapeItsOwnDirectory keeps a name with a
// separator in it from writing the session somewhere else entirely.
//
// The configuration directory is put inside a larger one this test owns, and
// what is asserted is that nothing landed in the larger one: a session written
// past the end of the path it was joined to is invisible to a walk of that
// path, so counting files inside it proves nothing.
func TestAnApplicationNameCannotEscapeItsOwnDirectory(t *testing.T) {
	outer := t.TempDir()
	config := filepath.Join(outer, "config")
	if err := os.MkdirAll(config, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", config)

	for _, name := range []string{"../escaped", "../../escaped", "..", "/etc/nothing", "a/b", "."} {
		store, err := client.FileSession(name)
		if err != nil {
			t.Fatalf("%q was refused rather than made safe: %v", name, err)
		}
		if err := store.Save("http://example.test", []*http.Cookie{{Name: "session", Value: "x"}}); err != nil {
			t.Fatalf("%q could not be saved: %v", name, err)
		}
	}

	// Measured against the directory the platform actually chose, not against
	// the one this test set: on some platforms the configuration directory is
	// a fixed path under the account's home rather than the variable, and a
	// name that escaped would land inside the home and outside that path --
	// which a check written against the home would call fine.
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("this account has no configuration directory: %v", err)
	}

	if len(sessionFiles(t, base)) == 0 {
		t.Fatal("nothing was written anywhere, so nothing was checked")
	}
	for _, path := range sessionFiles(t, outer) {
		rel, err := filepath.Rel(base, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Errorf("a session was written to %s, outside %s", path, base)
		}
	}
}

// TestAStoreIsRequiredToHaveAName keeps two applications from sharing one
// session file by both being nameless.
func TestAStoreIsRequiredToHaveAName(t *testing.T) {
	if _, err := client.FileSession(""); err == nil {
		t.Error("a session file with no application name was accepted")
	}
}

// TestTheSessionFileHoldsNoMoreThanItNeeds keeps the file from growing fields
// that describe how a browser should have handled a cookie it never handled.
func TestTheSessionFileHoldsNoMoreThanItNeeds(t *testing.T) {
	store := client.MemorySession()
	if err := store.Save("http://example.test", []*http.Cookie{{Name: "session", Value: "held"}}); err != nil {
		t.Fatal(err)
	}

	cookies, err := store.Load("http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 1 || cookies[0].Value != "held" {
		t.Fatalf("the memory store answered %v", cookies)
	}

	// And what it hands back is the caller's, not its own.
	cookies[0].Value = "changed"
	again, err := store.Load("http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if again[0].Value != "held" {
		t.Error("writing to what the store answered changed what it holds")
	}
}

// ask fetches the root page, which is the one that says whether there is a
// session.
func ask(t *testing.T, c *client.Client) (client.Page, error) {
	t.Helper()
	return c.Get(context.Background(), "/")
}

// fileStore is a store under a directory this test owns.
func fileStore(t *testing.T) client.Store {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	store, err := client.FileSession("Proof")
	if err != nil {
		t.Skipf("this account has no configuration directory: %v", err)
	}
	return store
}

// findSession answers the one session file under a directory.
func findSession(t *testing.T, dir string) string {
	t.Helper()

	found := sessionFiles(t, dir)
	if len(found) != 1 {
		t.Fatalf("%d session files under %s, want one: %v", len(found), dir, found)
	}
	return found[0]
}

// sessionFiles answers every session file under a directory.
func sessionFiles(t *testing.T, dir string) []string {
	t.Helper()

	var found []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Base(path) == "session.json" {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// TestADeletionWithAnAgeOfZeroIsADeletion is the spelling the store cannot
// catch on its own.
//
// A deletion arrives two ways, and only one of them survives the round trip: an
// expiry already past is still on the cookie when it is read back, so the store
// drops it either way. An age of zero is not stored at all -- it describes how
// long from now, and "now" is gone by the next run -- so if the jar does not act
// on it as it arrives, nothing ever will.
//
// The value is deliberately not emptied here. A server that ends a session by
// sending the same value with an age of zero is unusual and allowed, and it is
// the case where a jar that ignored the age would carry a cookie that still
// looks valid.
func TestADeletionWithAnAgeOfZeroIsADeletion(t *testing.T) {
	held := &http.Cookie{Name: "session", Value: "held", Path: "/"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, held)
			_, _ = w.Write([]byte(`{"view":"home","data":{}}`))
		case "/logout":
			http.SetCookie(w, &http.Cookie{Name: held.Name, Value: held.Value, Path: "/", MaxAge: -1})
			_, _ = w.Write([]byte(`{"view":"auth.login","data":{}}`))
		default:
			sent, err := r.Cookie(held.Name)
			if err != nil || sent.Value != held.Value {
				_, _ = w.Write([]byte(`{"view":"auth.login","data":{}}`))
				return
			}
			_, _ = w.Write([]byte(`{"view":"home","data":{}}`))
		}
	}))
	defer server.Close()

	store := fileStore(t)

	first, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Get(context.Background(), "/logout"); err != nil {
		t.Fatal(err)
	}

	second, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, second); page.View != "auth.login" {
		t.Errorf("a session ended with an age of zero came back after a restart: %q", page.View)
	}
}

// TestAnExpiredSessionIsNotLeftInTheFile keeps the store from growing.
//
// A jar drops an expired cookie when it is asked for one, so an application
// works correctly with stale entries still on disk. What it does not do is
// remove them: the file keeps every session the person ever had, each one a
// credential that is no longer any use to them and is still of use to whoever
// reads the file.
//
// Written against the store rather than through a client, and deliberately: a
// test that signs in and waits for an expiry is a test that measures how busy
// the machine was, and it fails on the machine that was busiest.
func TestAnExpiredSessionIsNotLeftInTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	store, err := client.FileSession("Proof")
	if err != nil {
		t.Skipf("this account has no configuration directory: %v", err)
	}

	const base = "http://example.test"
	alive := &http.Cookie{Name: "session", Value: "held", Path: "/", Expires: time.Now().Add(time.Hour)}
	if err := store.Save(base, []*http.Cookie{alive}); err != nil {
		t.Fatal(err)
	}

	path := findSession(t, dir)
	written, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(written), "held") {
		t.Fatalf("a session within its life was not written: %v / %s", err, written)
	}

	// The same cookie, with the life the server would give it to end it.
	past := &http.Cookie{Name: "session", Value: "held", Path: "/", Expires: time.Now().Add(-time.Hour)}
	if err := store.Save(base, []*http.Cookie{past}); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "held") {
		t.Errorf("a session past its expiry is still written on disk: %s", body)
	}

	if cookies, err := store.Load(base); err != nil || len(cookies) != 0 {
		t.Errorf("the store answered %d cookies past their expiry (%v)", len(cookies), err)
	}
}

// TestClearingOneServerLeavesTheOthers keeps signing out of one from signing
// out of every server the application was ever pointed at.
func TestClearingOneServerLeavesTheOthers(t *testing.T) {
	development := signing(t, &http.Cookie{Name: "session", Value: "dev", Path: "/"})
	production := signing(t, &http.Cookie{Name: "session", Value: "prod", Path: "/"})
	store := fileStore(t)

	for _, server := range []*httptest.Server{development, production} {
		talk, err := client.New(server.URL, client.WithSession(store))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := talk.Get(context.Background(), "/login"); err != nil {
			t.Fatal(err)
		}
	}

	out, err := client.New(development.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Forget(); err != nil {
		t.Fatal(err)
	}

	back, err := client.New(development.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, back); page.View != "auth.login" {
		t.Errorf("the session that was signed out of is still there: %q", page.View)
	}

	other, err := client.New(production.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := ask(t, other); page.View != "home" {
		t.Errorf("signing out of one server signed out of the other: %q", page.View)
	}
}

// serverBase answers the address a session is filed under, which is the scheme
// and the host of a server and nothing else.
func serverBase(t *testing.T, address string) string {
	t.Helper()

	u, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
}
