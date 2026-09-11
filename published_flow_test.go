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
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/arandu-io/ayra/client"
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
