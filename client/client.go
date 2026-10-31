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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
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

	// Token is the CSRF token of this session, and empty on a page that has
	// none.
	//
	// It is beside the view's name rather than inside the values because it is
	// a fact about the protocol and not about the page. The markup
	// representation of the same address carries it too, in the hidden field a
	// form writes and the header attribute an enhanced request sends; a client
	// that draws for itself has neither, so it is given the value and sends it
	// back on anything that changes something.
	//
	// [Client] does that by itself. The field is here because a caller that
	// builds its own requests still needs to reach it.
	Token string `json:"token"`
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
// session every later page is fetched under. Where the server issues a token
// instead, [WithBearer] sends it on every request. Either way this mints
// nothing and decides nothing about what the person may see: the credential is
// the server's, and the server decides, from it, the same way it decides for a
// browser.
type Client struct {
	base *url.URL
	http *http.Client
	// session is where the cookies outlive the process, and nil when they do
	// not.
	session Store
	// csrf is the token the server last sent, which every request that changes
	// something has to carry back.
	csrf tokens
	// limit is the most one answer may hold once decoded, in bytes.
	limit int64
	// download is the most a download may hold, zero for the default.
	download int64
	// bearer is the credential provider, nil when the client signs in with a
	// session. Calls to it are serialised by bearerMu, so concurrent requests
	// that find the token spent wait for one renewal instead of making several.
	bearer   Bearer
	bearerMu sync.Mutex
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

	c := &Client{base: u, http: &http.Client{}, limit: DefaultPageLimit}
	for _, opt := range opts {
		opt(c)
	}
	// One place decides this, and it is after the options rather than before
	// them: a transport somebody supplied has the same hang as one built here,
	// arriving through a caller who was thinking about certificates rather than
	// about tunnels. Setting it in both places would mean a mutation to either
	// one is repaired by the other, which is where a fault hides.
	if c.http.Timeout == 0 {
		c.http.Timeout = requestTimeout
	}
	if c.http.Jar == nil {
		jar, err := newJar(u, c.session)
		if err != nil {
			return nil, err
		}
		c.http.Jar = jar
	}
	// Set on the transport in place, like the timeout and the jar, because the
	// transport is what follows a redirect and a request has no policy of its
	// own. Whatever the caller's policy decided still applies, after this has
	// refused anything that leaves the server.
	c.http.CheckRedirect = stayOnServer(c.http.CheckRedirect)
	return c, nil
}

// requestTimeout is how long one request may take before it is given up on.
//
// A client with no timeout waits forever, and on a device forever is what a
// train tunnel looks like: the screen stays on "Signing in..." with its
// controls disabled, and the only way out is to kill the application. Thirty
// seconds is longer than any page this fetches should take and short enough
// that a person still believes the button did something.
const requestTimeout = 30 * time.Second

// Option configures a client.
type Option func(*Client)

// WithHTTPClient hands the client the transport to use.
//
// It is here for a test and for a device that needs its own transport -- a
// pinned certificate, a proxy the platform requires. A transport that arrives
// with no cookie jar is given one by New, because a client without one loses
// its session between pages.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// Get fetches the page at path.
//
// The values come back as the handler produced them.
func (c *Client) Get(ctx context.Context, path Path) (Page, error) {
	return c.do(ctx, http.MethodGet, path, nil, "")
}

// Post submits form values to path and answers the page that followed.
//
// It is a form and not a JSON body because the server it talks to reads forms:
// the same handler, the same validation and the same errors a browser reaches.
// A body of our own would be a second way in, and a second way in is a second
// set of rules about who may pass.
func (c *Client) Post(ctx context.Context, path Path, form url.Values) (Page, error) {
	return c.do(ctx, http.MethodPost, path, []byte(form.Encode()), "application/x-www-form-urlencoded")
}

func (c *Client) do(ctx context.Context, method string, path Path, body []byte, contentType string) (Page, error) {
	name := path.String()
	res, err := c.send(ctx, method, path, body, contentType, ViewMediaType)
	if err != nil {
		return Page{}, err
	}
	defer res.Body.Close()

	page, err := decode(method, name, res, c.limit)
	if err != nil {
		return page, err
	}
	c.csrf.remember(page.Token)
	return page, nil
}

// send makes one request, with the credential the client was given, and answers
// the response unread.
//
// A response that was refused for its token is asked for once more, after the
// provider has been told which token was refused: a token that expired between
// being issued and being used is the ordinary case, and a screen that failed on
// it would be a screen that failed for something the application can repair.
// Only a request whose body can be sent again is retried, which is every one
// this client makes.
func (c *Client) send(ctx context.Context, method string, path Path, body []byte, contentType, accept string) (*http.Response, error) {
	address, err := path.on(c.base)
	if err != nil {
		return nil, err
	}
	name := path.String()

	rejected := ""
	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(onServer(ctx, c.base), method, address.String(), reader)
		if err != nil {
			return nil, fmt.Errorf("ayra/client: %s %s: %w", method, name, err)
		}
		req.Header.Set("Accept", accept)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		c.csrf.carry(req, c.base)

		token, err := c.credential(ctx, rejected)
		if err != nil {
			return nil, fmt.Errorf("ayra/client: %s %s: the credential could not be obtained: %w", method, name, err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("ayra/client: %s %s: %w", method, name, err)
		}
		if !sameServer(c.base, res.Request.URL) {
			// The redirect policy refuses this before anything is sent. A
			// transport that followed one by other means still does not have its
			// answer read: a page from elsewhere is not drawn, and its token is not
			// the one sent back to the server.
			res.Body.Close()
			return nil, fmt.Errorf("%w: answered from %s://%s", ErrOffServer, res.Request.URL.Scheme, res.Request.URL.Host)
		}
		if res.StatusCode == http.StatusUnauthorized && token != "" && attempt == 0 {
			res.Body.Close()
			rejected = token
			continue
		}
		return res, nil
	}
}

// decode turns one response into a page, or into the error that says why the
// response was not one.
func decode(method, path string, res *http.Response, limit int64) (Page, error) {
	if res.StatusCode >= 400 {
		return Page{}, statusError(method, path, res)
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

	body, err := readLimited(method, path, res, limit)
	if err != nil {
		return Page{}, err
	}
	var page Page
	if err := json.Unmarshal(body, &page); err != nil {
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
	// Method is the verb the refused request carried.
	Method string
	// Path is the address that was asked for.
	Path string
	// Status is the code the server answered with, and it is what a caller
	// acts on.
	Status int
	// ContentType is the media type the refusal's body was sent as, without its
	// parameters.
	ContentType string
	// Body is the start of what the server said with the refusal, at most
	// [ErrorBodyLimit] bytes of it. A server that explains a refusal -- a sign-in
	// barrier, a forbidden request with a code the screen acts on -- does it
	// here, and a client that dropped the body could tell the person only that
	// something was refused.
	Body []byte
}

// ErrorBodyLimit is how much of a refusal's body a [StatusError] keeps: 64 KiB.
// A refusal is a sentence and a code, and the cap is what stops a server that
// answers an error with a page from filling a phone's memory.
const ErrorBodyLimit = 64 << 10

// statusError builds the error for a refused response, reading the start of its
// body. A body that cannot be read leaves the error without one, because the
// status is the part that decides what happens next.
func statusError(method, path string, res *http.Response) *StatusError {
	body, _ := io.ReadAll(io.LimitReader(res.Body, ErrorBodyLimit))
	return &StatusError{
		Method: method, Path: path, Status: res.StatusCode,
		ContentType: mediaType(res.Header.Get("Content-Type")), Body: body,
	}
}

// Into decodes the body of a refusal as JSON into v, which is a pointer to
// whatever the screen expects -- the server's error envelope, with its code and
// its messages. It answers an error when the body is not JSON or does not fit,
// and the caller then falls back on the status.
func (e *StatusError) Into(v any) error {
	if len(e.Body) == 0 {
		return errors.New("ayra/client: the refusal carried no body")
	}
	if err := json.Unmarshal(e.Body, v); err != nil {
		return fmt.Errorf("ayra/client: the body of the %d from %s %s does not fit %T: %w", e.Status, e.Method, e.Path, v, err)
	}
	return nil
}

// Error names the request and what the server answered.
//
// The number as well as the name, and a name of this package's own where the
// protocol has none. A server may answer with a status outside the registered
// set -- this one does, for a request whose CSRF token is missing or spent --
// and the standard text for such a status is the empty string. What a developer
// saw was the request, a colon, and nothing after it.
func (e *StatusError) Error() string {
	return fmt.Sprintf("ayra/client: %s %s: %d %s", e.Method, e.Path, e.Status, statusName(e.Status))
}

// statusName is what a status is called.
//
// The protocol's own name where there is one, and this package's where there is
// not. The one that matters is the token: it is the first thing a native client
// meets when it reaches an application that protects its forms, and a blank
// error is the worst possible thing to meet it with.
func statusName(status int) string {
	if name := http.StatusText(status); name != "" {
		return name
	}
	if status == statusCSRFExpired {
		return "the CSRF token was missing or is no longer valid"
	}
	return "answered with a status this does not have a name for"
}

// statusCSRFExpired is what a server of this project answers when a request
// that changes something arrives without a usable token.
//
// It is not a registered status and there is no standard name for it, which is
// why it is written down here. The number is the server's and this only reads
// it: a client that refused it locally would be a client deciding something the
// server decides.
const statusCSRFExpired = 419

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
