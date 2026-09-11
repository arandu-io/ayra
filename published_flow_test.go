package ayra_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The published tree is the only part of this repository nothing else compiles
// against, and it is the part every project runs. That it builds is already
// held; what it does when a server answers was not.
//
// The flow it has to get right is the one every application built on this
// starts with: a form is filled in, the server answers with a page, and the
// screen becomes that page. Every step of it is a place to get stuck -- the
// request going out with the wrong values, the answer arriving and never being
// applied, the screen mapping picking the page it came from. Driving a window
// with synthetic presses proves this too, and proves it once, on one machine,
// when the pointer lands where the test expected.
//
// So the starter's own files are staged with a test beside them and run in
// place, against a server this test stands up.

// flowTest is the test written next to the staged sources. It is in their
// package, so it reaches the screens and the state the way the program does.
const flowTest = `package main

import (
	"context"
	"image"
	"sync"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/arandu-io/ayra"
	"github.com/arandu-io/ayra/client"
	"github.com/arandu-io/ayra/engine/f32"
	"github.com/arandu-io/ayra/engine/font/gofont"
	"github.com/arandu-io/ayra/engine/io/input"
	"github.com/arandu-io/ayra/engine/io/pointer"
	"github.com/arandu-io/ayra/engine/layout"
	"github.com/arandu-io/ayra/engine/op"
	"github.com/arandu-io/ayra/engine/text"
	"github.com/arandu-io/ayra/engine/unit"
	"github.com/arandu-io/ayra/theme"
)

// TestSigningInReachesTheHomeScreen is the whole of what a starter has to do.
func TestSigningInReachesTheHomeScreen(t *testing.T) {
	var posted url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		if r.URL.Path == "/login" {
			_ = r.ParseForm()
			posted = r.PostForm
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "held", Path: "/"})
			_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{"name":"Paulo"}}` + "`" + `))
			return
		}
		if _, err := r.Cookie("session"); err != nil {
			_, _ = w.Write([]byte(` + "`" + `{"view":"auth.login","data":{}}` + "`" + `))
			return
		}
		_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{"name":"Paulo"}}` + "`" + `))
	}))
	defer server.Close()

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{server: talk}

	if app.screen != SignIn {
		t.Fatalf("the application opens on screen %v; until there is a session there is nothing to draw", app.screen)
	}

	page, err := talk.Post(context.Background(), "/login", url.Values{
		"email":    {"paulo@hyz.is"},
		"password": {"a-password"},
	})
	if err != nil {
		t.Fatalf("signing in was refused: %v", err)
	}
	if got := posted.Get("email"); got != "paulo@hyz.is" {
		t.Errorf("the server was sent the address %q", got)
	}

	if err := app.show(page); err != nil {
		t.Fatalf("the answer did not become a screen: %v", err)
	}
	if app.screen != Home {
		t.Errorf("after signing in the application is on screen %v", app.screen)
	}
	if app.home.values.Name != "Paulo" {
		t.Errorf("the home screen holds the name %q, and the server sent Paulo", app.home.values.Name)
	}

	// And the session it was handed is carried, which is what makes the next
	// page reachable without signing in again.
	next, err := talk.Get(context.Background(), "/")
	if err != nil {
		t.Fatalf("the page after signing in was refused: %v", err)
	}
	if next.View != "home" {
		t.Errorf("the page after signing in is %q, so the session was not carried", next.View)
	}
}

// TestAnEndedSessionReturnsToSignIn fixes what a screen does when the server
// stops recognising it.
func TestAnEndedSessionReturnsToSignIn(t *testing.T) {
	app := &App{}
	app.screen = Home
	app.signIn.values.Error = "left over"

	if err := app.show(client.Page{View: "auth.login"}); err != nil {
		t.Fatal(err)
	}
	if app.screen != SignIn {
		t.Errorf("the application stayed on screen %v after the session ended", app.screen)
	}
	if app.signIn.values.Error != "" {
		t.Errorf("what was on the form belonged to the session that ended, and it is still there: %q", app.signIn.values.Error)
	}
}

// TestASessionKeptFromLastTimeOpensWhereItLeftOff is what the store is worth to
// somebody using the application.
//
// Keeping the session is only half of it. An application that kept one and
// still opened on the form would ask for a password it did not need, and the
// second sign-in is what proves the first was pointless.
func TestASessionKeptFromLastTimeOpensWhereItLeftOff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "held", Path: "/"})
			_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{"name":"Paulo"}}` + "`" + `))
			return
		}
		if sent, err := r.Cookie("session"); err != nil || sent.Value != "held" {
			_, _ = w.Write([]byte(` + "`" + `{"view":"auth.login","data":{}}` + "`" + `))
			return
		}
		_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{"name":"Paulo"}}` + "`" + `))
	}))
	defer server.Close()

	store := client.MemorySession()

	// The first run signs in.
	first, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	page, err := first.Post(context.Background(), "/login", url.Values{"email": {"paulo@hyz.is"}})
	if err != nil {
		t.Fatal(err)
	}
	signed := &App{server: first}
	if err := signed.show(page); err != nil {
		t.Fatal(err)
	}
	if signed.screen != Home {
		t.Fatalf("signing in did not reach the home screen: %v", signed.screen)
	}

	// The second run is the application opened again. It asks, and the answer
	// is the page a known person gets.
	second, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	again, err := second.Get(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}

	opened := &App{server: second}
	if opened.screen != SignIn {
		t.Fatalf("the application does not start on the form: %v", opened.screen)
	}
	if err := opened.show(again); err != nil {
		t.Fatal(err)
	}
	if opened.screen != Home {
		t.Errorf("a session kept from last time still opened on the form")
	}
	if opened.home.values.Name != "Paulo" {
		t.Errorf("the page that came back holds the name %q", opened.home.values.Name)
	}
}

// TestTheApplicationAsksTheServerBeforeShowingAnything fixes an application
// that opens on the form whatever the server would have said.
//
// Driven through Layout and not through the asking itself, because the fault
// this guards against is the call being absent from the frame: a test that
// calls it directly passes on an application that never does.
func TestTheApplicationAsksTheServerBeforeShowingAnything(t *testing.T) {
	asked := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked++
		mu.Unlock()

		w.Header().Set("Content-Type", client.ViewMediaType)
		_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{"name":"Paulo"}}` + "`" + `))
	}))
	defer server.Close()

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{server: talk}

	app.Layout(frame(t))
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return asked > 0 })

	// The answer becomes the screen on the frame after it arrives, which is
	// also where a second question would be put.
	for range 4 {
		app.Layout(frame(t))
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	times := asked
	mu.Unlock()

	if times != 1 {
		t.Errorf("the server was asked %d times for the page to open on", times)
	}
	if app.screen != Home {
		t.Errorf("the answer never became a screen: the application is on %v", app.screen)
	}
}

// TestSigningOutForgetsTheSessionHere is the half a request to the server does
// not do.
//
// Without it the cookie is still carried and a kept session is still on disk,
// so the next start signs back in to a session the server has already thrown
// away -- and what comes back is a refusal nobody can explain.
func TestSigningOutForgetsTheSessionHere(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)

		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "held", Path: "/"})
			_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{}}` + "`" + `))
			return
		}
		// Deliberately silent about the cookie on the way out: this is the
		// server that ends the session on its own side and says nothing, which
		// is the case this side has to handle by itself.
		if sent, err := r.Cookie("session"); err == nil && sent.Value == "held" {
			if r.URL.Path == "/logout" {
				_, _ = w.Write([]byte(` + "`" + `{"view":"auth.login","data":{}}` + "`" + `))
				return
			}
			_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{}}` + "`" + `))
			return
		}
		_, _ = w.Write([]byte(` + "`" + `{"view":"auth.login","data":{}}` + "`" + `))
	}))
	defer server.Close()

	store := client.MemorySession()
	talk, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	// Through the screen's own way out, which is the half that is easy to
	// leave behind.
	app := &App{server: talk}
	page, err := app.signOut(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if page.View != "auth.login" {
		t.Errorf("signing out answered %q", page.View)
	}

	if answer, err := talk.Get(context.Background(), "/"); err != nil || answer.View != "auth.login" {
		t.Errorf("the client still carries the session it signed out of: %q (%v)", answer.View, err)
	}

	next, err := client.New(server.URL, client.WithSession(store))
	if err != nil {
		t.Fatal(err)
	}
	if answer, err := next.Get(context.Background(), "/"); err != nil || answer.View != "auth.login" {
		t.Errorf("the session survived signing out and a restart: %q (%v)", answer.View, err)
	}
}

// TestTheSessionInTheConfigReachesTheClient keeps a field somebody filled in
// from being a field nothing reads.
func TestTheSessionInTheConfigReachesTheClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", client.ViewMediaType)
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "held", Path: "/"})
		}
		_, _ = w.Write([]byte(` + "`" + `{"view":"home","data":{}}` + "`" + `))
	}))
	defer server.Close()

	store := client.MemorySession()

	talk, err := talkTo(Config{Server: server.URL, Session: store})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := talk.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}

	kept, err := store.Load(serverBase(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) == 0 {
		t.Error("the store in the configuration was never given the session")
	}

	// And with no store the session is kept nowhere, which is the default.
	plain, err := talkTo(Config{Server: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.Get(context.Background(), "/login"); err != nil {
		t.Fatal(err)
	}
	if again, _ := store.Load(serverBase(t, server.URL)); len(again) != len(kept) {
		t.Error("a client with no store wrote to one anyway")
	}
}

// serverBase answers the address a session is filed under: the scheme and the
// host, and nothing else.
func serverBase(t *testing.T, address string) string {
	t.Helper()

	u, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
}

// TestPressingSignOutAsksTheServer is the wiring between the button and the
// request, which is the one part of signing out that no amount of testing the
// request itself can reach.
//
// A layout that computed the way out and never called it draws a button that
// does nothing, and every other test of signing out passes.
func TestPressingSignOutAsksTheServer(t *testing.T) {
	var mu sync.Mutex
	out := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logout" {
			mu.Lock()
			out++
			mu.Unlock()
		}
		w.Header().Set("Content-Type", client.ViewMediaType)
		_, _ = w.Write([]byte(` + "`" + `{"view":"auth.login","data":{}}` + "`" + `))
	}))
	defer server.Close()

	talk, err := client.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	app := &App{server: talk}
	app.asked = true // the opening question is not what this is about
	app.screen = Home

	device := &presses{ops: new(op.Ops), size: image.Pt(400, 600)}

	drawn := device.draw(app)
	if drawn.Size.Y <= 0 {
		t.Fatal("the home screen drew nothing to press")
	}

	// The way out is the last thing in the column, and the column is centred.
	device.click(drawn.Size.X/2, drawn.Size.Y/2+40)
	device.draw(app)

	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return out > 0 })

	mu.Lock()
	asked := out
	mu.Unlock()
	if asked != 1 {
		t.Errorf("pressing the way out asked the server %d times", asked)
	}
}

// presses is a frame with a pointer attached, which the plain frame has not
// got.
type presses struct {
	router input.Router
	ops    *op.Ops
	size   image.Point
}

// draw runs one frame and hands the operations to the router.
func (d *presses) draw(app *App) ayra.Dimensions {
	d.ops.Reset()

	gtx := layout.Context{
		Ops:         d.ops,
		Source:      d.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: d.size},
	}
	dims := app.Layout(ayra.Context{
		Context: gtx,
		Theme:   theme.New(theme.Light),
		Shaper:  text.NewShaper(text.WithCollection(gofont.Collection())),
	})

	d.router.Frame(d.ops)
	return dims
}

// click queues a press and the letting go that completes it.
func (d *presses) click(x, y int) {
	at := f32.Pt(float32(x), float32(y))
	d.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
}

// frame is one frame, without a window and without a GPU.
//
// It carries a palette and a shaper because Layout draws: a context with
// neither is one that gets as far as the first piece of text and stops. The
// whole point of driving Layout here rather than the asking inside it is that
// the frame is where the fault would be, so the frame has to be a real one.
func frame(t *testing.T) ayra.Context {
	t.Helper()

	return ayra.Context{
		Context: layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(400, 600)},
		},
		Theme:  theme.New(theme.Light),
		Shaper: text.NewShaper(text.WithCollection(gofont.Collection())),
	}
}

// waitFor gives a request on its own goroutine time to finish.
func waitFor(t *testing.T, done func() bool) {
	t.Helper()

	for range 200 {
		if done() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the request never finished")
}
`

// TestThePublishedTreeSignsIn runs the starter's own flow where the starter is.
func TestThePublishedTreeSignsIn(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(source, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("%s holds no Go file", source)
	}

	// Its own staging directory, not the compile gate's: two tests sharing one
	// would be two tests whose order decides whether either has any files.
	root := ".published-flow-check"
	staging := filepath.Join(root, "native")
	t.Cleanup(func() { os.RemoveAll(root) })
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(staging, filepath.Base(file)), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(staging, "flow_test.go"), []byte(flowTest), 0o644); err != nil {
		t.Fatal(err)
	}

	run := exec.Command("go", "test", "-count=1", "-v", ".")
	run.Dir = staging
	run.Env = append(os.Environ(), "GOWORK=off")
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("the published tree does not sign in:\n%s", output)
	}
}
