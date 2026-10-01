package client

import (
	"fmt"
	"io"
	"net/http"
)

// DefaultPageLimit is the most one answer may hold once decoded, in bytes,
// unless [WithPageLimit] says otherwise: 8 MiB.
//
// The transport inflates a compressed answer before this reads it, so a body
// a few hundred kilobytes long on the wire can arrive as hundreds of megabytes.
// On a phone that is the application killed by the system with nothing on the
// screen to say why. Eight megabytes is far more than a page of values a
// screen can show and far less than a device can hold.
const DefaultPageLimit = 8 << 20

// WithPageLimit sets the most one answer may hold once decoded, in bytes.
//
// An answer past it is refused with a [*TooLargeError] rather than read. A
// limit of zero or less keeps [DefaultPageLimit]: an unlimited answer is the
// fault the limit exists to stop, so it is not something an option can ask
// for.
func WithPageLimit(n int64) Option {
	return func(c *Client) {
		if n > 0 {
			c.limit = n
		}
	}
}

// TooLargeError is what a request answers when the page is larger than the
// client's limit.
//
// It is a type because the caller may act on it -- a screen that asked for a
// list too long to carry can ask for less -- and the limit it carries is the
// one that was applied.
type TooLargeError struct {
	// Method is the verb the request carried.
	Method string
	// Path is the address that was asked for.
	Path string
	// Limit is the most the answer was allowed to hold, in bytes.
	Limit int64
}

// Error names the request and the limit it went past.
func (e *TooLargeError) Error() string {
	return fmt.Sprintf("ayra/client: %s %s answered more than %d bytes, the most a page may hold", e.Method, e.Path, e.Limit)
}

// readLimited answers the whole body, or a [*TooLargeError] once it holds more
// than limit bytes.
//
// A declared length past the limit is refused before anything is read. The
// declaration is not trusted the other way: a body is read to one byte past
// the limit whatever it claimed, because the length a server declares is the
// compressed one, or a lie.
func readLimited(method, path string, res *http.Response, limit int64) ([]byte, error) {
	if res.ContentLength > limit {
		return nil, &TooLargeError{Method: method, Path: path, Limit: limit}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("ayra/client: %s %s: %w", method, path, err)
	}
	if int64(len(body)) > limit {
		return nil, &TooLargeError{Method: method, Path: path, Limit: limit}
	}
	return body, nil
}
