// Package client fetches pages from a running Arandu server and answers what
// each page was going to be drawn from.
//
// A browser asks the same server for the same address and is handed markup,
// because a browser draws markup. A program that draws with its own controls
// asks for the same address and is handed the name of the page and its values
// instead. Both are the same resource in two representations, which is what
// content negotiation is for, so nothing here is a second address, a second
// handler or an API.
//
// Nothing in this package draws, and nothing in it decides. A page arrives as
// the name the server rendered and the values it rendered from; picking a
// screen for that name belongs to the application, because only the
// application knows what its screens are.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ViewMediaType is what this asks for, and what a server that understands the
// question answers with.
//
// The spelling is shared with the server and is therefore not free to change
// on one side: a client asking for a media type the server does not know is
// handed a page of markup, which fails at the first attempt to read a value
// out of it rather than at the request. The module half of this project holds
// a test that fails if these two ever disagree.
const ViewMediaType = "application/vnd.arandu.view+json"

// Page is one answer: the name of the view the server rendered, and the values
// it would have rendered from.
type Page struct {
	// View is the name the server chose. It travels because a single address
	// may answer with one of several pages depending on what it found, so the
	// request is not enough to know what came back.
	View string `json:"view"`

	// Data is what the page was going to be built from, left as it arrived.
	//
	// It is raw and not a map, because unmarshalling it twice -- once into a
	// map here and once into the screen's own type there -- loses the types
	// the second pass wants and costs an allocation per field to do it. Into
	// answers the screen's type directly.
	Data json.RawMessage `json:"data"`
}

// Into decodes a page's values into v, which is a pointer to whatever the
// screen expects.
//
// A page that carried no values leaves v untouched rather than failing: a
// screen with nothing to show is a screen, and a handler is allowed to render
// one.
func (p Page) Into(v any) error {
	if len(p.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(p.Data, v); err != nil {
		return fmt.Errorf("ayra/client: the values of view %q do not fit %T: %w", p.View, v, err)
	}
	return nil
}

// Client asks one server for pages.
//
// It carries a cookie jar, so a session established by signing in is the
// session every later page is fetched under. That is the whole of its
// authentication: this holds no token, mints nothing, and decides nothing
// about what the person may see. The server decides, from the session, the
// same way it decides for a browser.
type Client struct {
	base *url.URL
	http *http.Client
}

// New returns a client for the server at base.
//
// The base has to be absolute, because a relative one has no meaning on a
// device: there is no page it could be relative to.
func New(base string, opts ...Option) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("ayra/client: %q is not an address: %w", base, err)
	}
	if !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("ayra/client: %q has no scheme and host, and a device has no page to resolve it against", base)
	}

	c := &Client{base: u, http: &http.Client{}}
	for _, opt := range opts {
		opt(c)
	}
	if c.http.Jar == nil {
		jar, err := newJar()
		if err != nil {
			return nil, err
		}
		c.http.Jar = jar
	}
	return c, nil
}

// Option configures a client.
type Option func(*Client)

// WithHTTPClient hands the client the transport to use.
//
// It is here for a test and for a device that needs its own transport -- a
// pinned certificate, a proxy the platform requires. A client given one that
// carries no cookie jar keeps working and loses its session between pages,
// which is why New fills one in when the transport arrived without.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// Get fetches the page at path.
//
// The path is relative to the server's base, and the values come back as the
// handler produced them.
func (c *Client) Get(ctx context.Context, path string) (Page, error) {
	return c.do(ctx, http.MethodGet, path, nil, "")
}

// Post submits form values to path and answers the page that followed.
//
// It is a form and not a JSON body because the server it talks to reads forms:
// the same handler, the same validation and the same errors a browser reaches.
// A body of our own would be a second way in, and a second way in is a second
// set of rules about who may pass.
func (c *Client) Post(ctx context.Context, path string, form url.Values) (Page, error) {
	return c.do(ctx, http.MethodPost, path, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (Page, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return Page{}, fmt.Errorf("ayra/client: %q is not a path: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base.ResolveReference(ref).String(), body)
	if err != nil {
		return Page{}, fmt.Errorf("ayra/client: %s %s: %w", method, path, err)
	}
	req.Header.Set("Accept", ViewMediaType)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("ayra/client: %s %s: %w", method, path, err)
	}
	defer res.Body.Close()

	return decode(method, path, res)
}

// decode turns one response into a page, or into the error that says why the
// response was not one.
func decode(method, path string, res *http.Response) (Page, error) {
	if res.StatusCode >= 400 {
		return Page{}, &StatusError{Method: method, Path: path, Status: res.StatusCode}
	}

	if got := mediaType(res.Header.Get("Content-Type")); got != ViewMediaType {
		// Markup came back, which means the server answered the browser's
		// question. Saying so is the whole point of the check: the alternative
		// is a decode failure at the first byte, which reads as a corrupt
		// response rather than as a server that does not speak this.
		return Page{}, fmt.Errorf(
			"ayra/client: %s %s answered %s, not %s -- the server does not offer this page's values",
			method, path, quote(got), ViewMediaType,
		)
	}

	var page Page
	if err := json.NewDecoder(res.Body).Decode(&page); err != nil {
		return Page{}, fmt.Errorf("ayra/client: %s %s: %w", method, path, err)
	}
	if page.View == "" {
		return Page{}, fmt.Errorf("ayra/client: %s %s answered values with no view name", method, path)
	}
	return page, nil
}

// StatusError is what a page refused by the server answers with.
//
// It is a type rather than a string because the caller acts on the number: 401
// sends a person to sign in, 403 says so on the screen they are on, and 404 is
// a screen that no longer exists. A string would make each of those a match on
// prose.
type StatusError struct {
	Method string
	Path   string
	Status int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("ayra/client: %s %s: %s", e.Method, e.Path, http.StatusText(e.Status))
}

// Status answers the status a refusal carried, and zero when err is not one.
//
// It exists so a caller can ask without naming the type at every site, which
// is the same reason the standard library offers this shape.
func Status(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status
	}
	return 0
}

// mediaType strips the parameters from a Content-Type, leaving the type.
func mediaType(header string) string {
	return strings.TrimSpace(strings.Split(header, ";")[0])
}

// quote renders a media type for a message, naming the absence when there was
// no header at all.
func quote(mt string) string {
	if mt == "" {
		return "no content type"
	}
	return `"` + mt + `"`
}
