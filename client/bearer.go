package client

import (
	"context"
	"fmt"
	"net/http"
)

// Bearer provides the token a client sends as "Authorization: Bearer".
//
// It is called before every request, and a call at a time: the client holds a
// lock while it runs, so requests that find the token spent wait for one
// renewal and then all use the renewed one. It is the application's, because
// where the token is kept and how it is renewed are the application's: this
// package holds no token of its own.
//
// rejected is empty on an ordinary call. When the server answered a request
// with 401 for a token this provider handed out, the client calls it once more
// with that token in rejected, before it gives up; the provider answers a
// renewed one, and answers rejected unchanged only when it has no better, which
// the client reads as the end of the matter. A provider that answers an empty
// token sends the request with no credential, which is how a screen that needs
// none -- the sign-in -- is reached.
type Bearer func(ctx context.Context, rejected string) (string, error)

// WithBearer makes the client send a token with every request, including the
// ones [Client.Download] makes, in place of a session.
//
// The token is sent only to the server the client was made for: a request that
// would leave it is refused before it is sent, and so is a redirect, which is
// what keeps the credential from reaching an address it was not issued for. The
// cookie jar still works, for a server that sets one beside the token.
func WithBearer(b Bearer) Option {
	return func(c *Client) { c.bearer = b }
}

// credential answers the token for one request, under the lock.
func (c *Client) credential(ctx context.Context, rejected string) (string, error) {
	if c.bearer == nil {
		return "", nil
	}
	c.bearerMu.Lock()
	defer c.bearerMu.Unlock()
	return c.bearer(ctx, rejected)
}

// DefaultDownloadLimit is the most [Client.Download] reads: 16 MiB.
const DefaultDownloadLimit = 16 << 20

// File is what a download answers: the bytes, and what the server said they
// are.
type File struct {
	// Data is the content.
	Data []byte
	// ContentType is the media type, without its parameters.
	ContentType string
}

// Download fetches the bytes at path under the client's credential: an image of
// a camera, a document, anything that is not a page of values.
//
// It is the same request as [Client.Get] -- the same session or token, the same
// refusal to leave the server -- with the question changed: it asks for accept
// instead of values (empty is anything), and what comes back is not decoded. A
// refusal is a [*StatusError] with the start of its body, and an answer past
// the limit is a [*TooLargeError]. The limit is [DefaultDownloadLimit], or the
// one [WithDownloadLimit] set.
func (c *Client) Download(ctx context.Context, path Path, accept string) (File, error) {
	if accept == "" {
		accept = "*/*"
	}
	name := path.String()
	res, err := c.send(ctx, http.MethodGet, path, nil, "", accept)
	if err != nil {
		return File{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return File{}, statusError(http.MethodGet, name, res)
	}
	limit := c.download
	if limit <= 0 {
		limit = DefaultDownloadLimit
	}
	data, err := readLimited(http.MethodGet, name, res, limit)
	if err != nil {
		return File{}, err
	}
	if res.StatusCode == http.StatusNoContent || len(data) == 0 && res.StatusCode != http.StatusOK {
		return File{}, fmt.Errorf("ayra/client: GET %s answered %d with no content", name, res.StatusCode)
	}
	return File{Data: data, ContentType: mediaType(res.Header.Get("Content-Type"))}, nil
}

// WithDownloadLimit sets the most [Client.Download] reads once decoded, in
// bytes. Zero or less keeps [DefaultDownloadLimit].
func WithDownloadLimit(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.download = n
		}
	}
}
