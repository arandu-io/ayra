package client_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/ayra/client"
)

// serve answers every request with the handler and returns a client pointed at
// it.
func serve(t *testing.T, h http.HandlerFunc) *client.Client {
	t.Helper()

	server := httptest.NewServer(h)
	t.Cleanup(server.Close)

	c, err := client.New(server.URL)
	if err != nil {
		t.Fatalf("the client refused a server that exists: %v", err)
	}
	return c
}

// viewData answers one page in the negotiated representation.
func viewData(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)
		w.Write([]byte(body))
	}
}

// TestTheRequestAsksForValuesRatherThanMarkup fixes the one header the whole
// design rests on.
//
// Without it the server answers the browser's question, and it answers it
// correctly -- so the failure is a page of markup arriving where values were
// expected, at the far end, rather than a request that was wrong.
func TestTheRequestAsksForValuesRatherThanMarkup(t *testing.T) {
	var accept string

	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		viewData(`{"view":"home","data":null}`)(w, r)
	})

	if _, err := c.Get(context.Background(), "/"); err != nil {
		t.Fatalf("the page was refused: %v", err)
	}
	if accept != client.ViewMediaType {
		t.Errorf("the request asked for %q, want %q", accept, client.ViewMediaType)
	}
}

// TestThePageCarriesTheNameTheServerChose keeps the client from assuming the
// address it asked for is the page it received.
func TestThePageCarriesTheNameTheServerChose(t *testing.T) {
	c := serve(t, viewData(`{"view":"auth.sign-in","data":{"email":"a@b.c"}}`))

	page, err := c.Get(context.Background(), "/dashboard")
	if err != nil {
		t.Fatalf("the page was refused: %v", err)
	}
	if page.View != "auth.sign-in" {
		t.Errorf("the page is %q, want the name the server chose", page.View)
	}

	var values struct {
		Email string `json:"email"`
	}
	if err := page.Into(&values); err != nil {
		t.Fatalf("the values did not fit: %v", err)
	}
	if values.Email != "a@b.c" {
		t.Errorf("the values arrived as %+v", values)
	}
}

// TestAPageWithNoValuesIsStillAPage keeps a screen that shows nothing from
// being an error.
func TestAPageWithNoValuesIsStillAPage(t *testing.T) {
	c := serve(t, viewData(`{"view":"about"}`))

	page, err := c.Get(context.Background(), "/about")
	if err != nil {
		t.Fatalf("a page with no values was refused: %v", err)
	}

	values := struct {
		Email string `json:"email"`
	}{Email: "untouched"}
	if err := page.Into(&values); err != nil {
		t.Fatalf("decoding nothing failed: %v", err)
	}
	if values.Email != "untouched" {
		t.Errorf("decoding nothing wrote %q", values.Email)
	}
}

// TestMarkupIsReportedAsAServerThatDoesNotSpeakThis fixes which of two
// diagnoses the caller gets.
//
// Decoding markup as values fails at the first byte and reads as a corrupt
// response, which sends whoever is reading it looking at the network. The
// actual fault is a server that has not been taught this representation, and
// the message has to say that instead.
func TestMarkupIsReportedAsAServerThatDoesNotSpeakThis(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!doctype html><title>Home</title>"))
	})

	_, err := c.Get(context.Background(), "/")
	if err == nil {
		t.Fatal("markup was accepted as values")
	}
	if !strings.Contains(err.Error(), "text/html") {
		t.Errorf("the message does not say what came back instead: %v", err)
	}
}

// TestARefusalCarriesItsStatus keeps the caller from matching on prose.
//
// The number is what it acts on: 401 sends a person to sign in, 403 says so on
// the screen they are already on, and 404 is a screen that no longer exists.
func TestARefusalCarriesItsStatus(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		c := serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		})

		_, err := c.Get(context.Background(), "/private")
		if err == nil {
			t.Fatalf("%d was accepted as a page", status)
		}
		if got := client.Status(err); got != status {
			t.Errorf("the refusal carried %d, want %d", got, status)
		}
	}
}

// TestSomethingThatIsNotARefusalCarriesNoStatus keeps Status from inventing
// one for an error that never had a response.
func TestSomethingThatIsNotARefusalCarriesNoStatus(t *testing.T) {
	c := serve(t, viewData(`not json`))

	_, err := c.Get(context.Background(), "/")
	if err == nil {
		t.Fatal("a body that is not values was accepted")
	}
	if got := client.Status(err); got != 0 {
		t.Errorf("a decode failure carried status %d", got)
	}
}

// TestValuesWithNoViewNameAreRefused keeps a page nobody can draw from looking
// like a page.
func TestValuesWithNoViewNameAreRefused(t *testing.T) {
	c := serve(t, viewData(`{"data":{"email":"a@b.c"}}`))

	if _, err := c.Get(context.Background(), "/"); err == nil {
		t.Fatal("values with no view name were accepted")
	}
}

// TestTheSessionSurvivesBetweenPages fixes the failure that is otherwise
// silent: every page answers, each one as a stranger.
func TestTheSessionSurvivesBetweenPages(t *testing.T) {
	var seen []string

	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("session"); err == nil {
			seen = append(seen, cookie.Value)
		} else {
			seen = append(seen, "")
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "s-1", Path: "/"})
		}
		viewData(`{"view":"home","data":null}`)(w, r)
	})

	ctx := context.Background()
	if _, err := c.Post(ctx, "/sign-in", url.Values{"email": {"a@b.c"}}); err != nil {
		t.Fatalf("signing in was refused: %v", err)
	}
	if _, err := c.Get(ctx, "/dashboard"); err != nil {
		t.Fatalf("the second page was refused: %v", err)
	}

	if len(seen) != 2 || seen[0] != "" || seen[1] != "s-1" {
		t.Errorf("the session did not travel: %q", seen)
	}
}

// TestAFormIsSubmittedAsAForm keeps the native client on the handler a browser
// reaches, with the same validation and the same errors.
func TestAFormIsSubmittedAsAForm(t *testing.T) {
	var contentType, email string

	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		r.ParseForm()
		email = r.PostFormValue("email")
		viewData(`{"view":"home","data":null}`)(w, r)
	})

	_, err := c.Post(context.Background(), "/sign-in", url.Values{"email": {"a@b.c"}})
	if err != nil {
		t.Fatalf("the form was refused: %v", err)
	}
	if contentType != "application/x-www-form-urlencoded" {
		t.Errorf("the form was sent as %q", contentType)
	}
	if email != "a@b.c" {
		t.Errorf("the server read the email as %q", email)
	}
}

// TestAnAddressWithNoHostIsRefused keeps a relative base from reaching a device.
//
// There is no page for it to be relative to there, so the first request would
// fail with something about a missing scheme, far from the line that set it.
func TestAnAddressWithNoHostIsRefused(t *testing.T) {
	for _, base := range []string{"", "/api", "example.com"} {
		if _, err := client.New(base); err == nil {
			t.Errorf("%q was accepted as a server address", base)
		}
	}
}

// TestATransportWithoutAJarStillKeepsItsSession fixes the quiet half of
// WithHTTPClient.
//
// A caller supplying a transport is doing it for a certificate or a proxy, not
// to opt out of having a session -- and losing one produces pages that all
// answer, every one of them anonymous.
func TestATransportWithoutAJarStillKeepsItsSession(t *testing.T) {
	var seen []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("session"); err == nil {
			seen = append(seen, cookie.Value)
		} else {
			seen = append(seen, "")
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "s-1", Path: "/"})
		}
		viewData(`{"view":"home","data":null}`)(w, r)
	}))
	t.Cleanup(server.Close)

	c, err := client.New(server.URL, client.WithHTTPClient(&http.Client{}))
	if err != nil {
		t.Fatalf("a transport without a jar was refused: %v", err)
	}

	ctx := context.Background()
	if _, err := c.Get(ctx, "/"); err != nil {
		t.Fatalf("the first page was refused: %v", err)
	}
	if _, err := c.Get(ctx, "/dashboard"); err != nil {
		t.Fatalf("the second page was refused: %v", err)
	}

	if len(seen) != 2 || seen[1] != "s-1" {
		t.Errorf("a supplied transport lost the session: %q", seen)
	}
}

// TestASuppliedJarIsKept keeps the fill-in above from overwriting a jar a
// caller had a reason to bring -- one primed with a session taken from
// somewhere else, or one that persists to disk.
func TestASuppliedJarIsKept(t *testing.T) {
	var seen string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("session"); err == nil {
			seen = cookie.Value
		}
		viewData(`{"view":"home","data":null}`)(w, r)
	}))
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(base, []*http.Cookie{{Name: "session", Value: "brought-along", Path: "/"}})

	c, err := client.New(server.URL, client.WithHTTPClient(&http.Client{Jar: jar}))
	if err != nil {
		t.Fatalf("a supplied jar was refused: %v", err)
	}
	if _, err := c.Get(context.Background(), "/"); err != nil {
		t.Fatalf("the page was refused: %v", err)
	}

	// The session has to reach the server, not merely survive in the jar the
	// caller still holds a reference to.
	if seen != "brought-along" {
		t.Errorf("the session the caller brought did not travel: %q", seen)
	}
}

// TestARequestThatNeverAnswersIsGivenUpOn fixes the failure a device sees and a
// desk does not.
//
// A client with no timeout waits forever. On a phone, forever is a train
// tunnel: the screen stays on "Signing in..." with its controls disabled, and
// the only way out is to kill the application. The server here accepts the
// connection and never replies, which is exactly what a captive network does.
func TestARequestThatNeverAnswersIsGivenUpOn(t *testing.T) {
	silence := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-silence
	}))

	// The channel is released before the server is closed, and the order is the
	// point: Close waits for the handlers to return, and a handler parked on a
	// channel nobody closed waits for the test's own deadline. Cleanups run
	// last-registered-first, so this one has to be registered after.
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(silence) })

	c, err := client.New(server.URL, client.WithHTTPClient(&http.Client{Timeout: 150 * time.Millisecond}))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := c.Get(context.Background(), "/")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a request that never answered was reported as a page")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the request was never given up on, and on a device that is a screen nobody can leave")
	}
}

// TestAClientAlwaysHasATimeout keeps the fix above from being one a caller can
// remove by accident.
//
// A supplied transport with no timeout is the same hang, arriving through
// somebody who was thinking about certificates rather than about tunnels.
func TestAClientAlwaysHasATimeout(t *testing.T) {
	transport := &http.Client{}

	if _, err := client.New("https://example.test", client.WithHTTPClient(transport)); err != nil {
		t.Fatal(err)
	}
	if transport.Timeout == 0 {
		t.Error("a supplied transport kept its lack of a timeout, and a request on it can hang forever")
	}
}
