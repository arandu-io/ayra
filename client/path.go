package client

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Path is where on the server a request goes: a sequence of segments from the
// server's root, and the query sent with it.
//
// It is built from segments rather than parsed from text because a path is
// assembled from values -- a slug from a page, a name somebody typed -- and a
// value spliced into a string is read as whatever its characters spell. A
// slug of "../admin/users/7?role=admin#" in "/teams/" + slug + "/members"
// moved the request to another route and added a field the server merges into
// its input. Here each segment is escaped on its own, so a '/', '?' or '#'
// inside one is a character of that segment and nothing else, and the query
// arrives only through [Path.Query].
//
// The zero value is the server's root.
type Path struct {
	segments []string
	query    url.Values
}

// At returns the path made of segments, in order, from the server's root.
//
// At() is the root itself. A segment that is empty, "." or ".." is refused
// when the request is made: escaping does not change those, and a server that
// normalises the path would read them as a step up or as nothing, which is the
// route rewritten again by other means.
func At(segments ...string) Path {
	return Path{segments: append([]string(nil), segments...)}
}

// Query returns a copy of the path that sends values as its query string.
//
// It replaces any query set before, and the values are copied, so changing
// the map afterwards does not change the path.
func (p Path) Query(values url.Values) Path {
	copied := make(url.Values, len(values))
	for key, list := range values {
		copied[key] = append([]string(nil), list...)
	}
	p.query = copied
	return p
}

// String answers the path as it is sent: each segment escaped, and the query
// encoded after a '?' when there is one.
func (p Path) String() string {
	s := "/" + strings.Join(escaped(p.segments), "/")
	if len(p.query) > 0 {
		s += "?" + p.query.Encode()
	}
	return s
}

// ErrSegment is what a request answers when one of its path's segments is
// empty, "." or "..".
var ErrSegment = errors.New("ayra/client: a path segment cannot be empty, \".\" or \"..\"")

// on answers the address of p on the server at base.
//
// The scheme, the host and the credentials are the base's and nothing else
// can set them, so a path cannot name another server. The base's own path is
// not prefixed: a path is from the server's root.
func (p Path) on(base *url.URL) (*url.URL, error) {
	for index, segment := range p.segments {
		if segment == "" || segment == "." || segment == ".." {
			return nil, fmt.Errorf("%w: segment %d of %s is %q", ErrSegment, index, p, segment)
		}
	}

	raw := "/" + strings.Join(escaped(p.segments), "/")
	return &url.URL{
		Scheme:   base.Scheme,
		User:     base.User,
		Host:     base.Host,
		Path:     "/" + strings.Join(p.segments, "/"),
		RawPath:  raw,
		RawQuery: p.query.Encode(),
	}, nil
}

// escaped answers each segment escaped on its own.
func escaped(segments []string) []string {
	out := make([]string, len(segments))
	for index, segment := range segments {
		out[index] = url.PathEscape(segment)
	}
	return out
}
