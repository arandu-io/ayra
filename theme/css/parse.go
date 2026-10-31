package css

import (
	"strings"
)

// read collects the custom properties a stylesheet declares for each scheme.
//
// It is a scanner and not a CSS parser, on purpose: what it needs is the
// top-level rules, their selectors, and the declarations directly inside them,
// and anything nested deeper -- a rule inside a media query, a data URI with a
// brace in it -- has to be stepped over without being understood. It tracks
// comments and quoted strings so that a brace inside either does not end a
// block, and it counts braces for the rest.
func (s *Sheet) read(source string) error {
	scan(stripComments(source), func(selector, body string) {
		switch scheme(selector) {
		case lightScheme:
			collect(body, s.light)
		case darkScheme:
			collect(body, s.dark)
		}
	})
	return nil
}

type schemeKind int

const (
	otherScheme schemeKind = iota
	lightScheme
	darkScheme
)

// scheme says which scheme a rule's selector declares properties for.
//
// Only the bare selectors count. `:root[data-theme="blue"]` and `.dark .card`
// restyle part of a page, not the scheme, and reading them as tokens would
// carry an accent choice into the default theme.
func scheme(selector string) schemeKind {
	selector = strings.Join(strings.Fields(selector), " ")
	switch selector {
	case ":root", "html", "@theme", "@theme inline", "@theme static":
		return lightScheme
	case ".dark", ":root.dark", "html.dark", ".dark:root":
		return darkScheme
	}
	return otherScheme
}

// scan calls visit for every rule at the top level of a stylesheet, and for
// every rule inside an @layer block, with its selector and the text between its
// braces.
func scan(source string, visit func(selector, body string)) {
	i := 0
	for i < len(source) {
		start := i
		// The prelude ends at a brace, or at a semicolon for a statement such
		// as @import.
		for i < len(source) && source[i] != '{' && source[i] != ';' {
			if source[i] == '"' || source[i] == '\'' {
				i = skipString(source, i)
				continue
			}
			i++
		}
		if i >= len(source) {
			return
		}
		if source[i] == ';' {
			i++
			continue
		}
		prelude := strings.TrimSpace(source[start:i])
		bodyStart := i + 1
		i = skipBlock(source, i)
		body := source[bodyStart:min(i-1, len(source))]
		if i > len(source) {
			body = source[bodyStart:]
		}

		if strings.HasPrefix(prelude, "@layer") || strings.HasPrefix(prelude, "@supports") {
			scan(body, visit)
			continue
		}
		visit(prelude, body)
	}
}

// skipBlock returns the index just past the brace that closes the one at i.
func skipBlock(source string, i int) int {
	depth := 0
	for i < len(source) {
		switch source[i] {
		case '"', '\'':
			i = skipString(source, i)
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
		i++
	}
	return i
}

// skipString returns the index just past the string that opens at i.
func skipString(source string, i int) int {
	quote := source[i]
	i++
	for i < len(source) {
		switch source[i] {
		case '\\':
			i += 2
			continue
		case quote:
			return i + 1
		}
		i++
	}
	return i
}

// stripComments removes /* ... */ outside of strings.
func stripComments(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); {
		switch {
		case source[i] == '"' || source[i] == '\'':
			end := skipString(source, i)
			out.WriteString(source[i:min(end, len(source))])
			i = end
		case strings.HasPrefix(source[i:], "/*"):
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				return out.String()
			}
			i += end + 4
		default:
			out.WriteByte(source[i])
			i++
		}
	}
	return out.String()
}

// collect reads the custom properties declared directly in a block, stepping
// over any nested rule.
func collect(body string, into declarations) {
	start := 0
	flush := func(end int) {
		declaration := strings.TrimSpace(body[start:end])
		start = end + 1
		if !strings.HasPrefix(declaration, "--") {
			return
		}
		name, value, found := strings.Cut(declaration, ":")
		if !found {
			return
		}
		name = strings.TrimPrefix(strings.TrimSpace(name), "--")
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!important"))
		if name != "" && value != "" {
			into[name] = value
		}
	}
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '"', '\'':
			i = skipString(body, i) - 1
		case '(':
			// A semicolon inside url(data:...;base64) is not a separator.
			end := strings.IndexByte(body[i:], ')')
			if end >= 0 {
				i += end
			}
		case '{':
			// A nested rule: its prelude is not a declaration, and its body is
			// stepped over.
			i = skipBlock(body, i) - 1
			start = i + 1
		case ';':
			flush(i)
		}
	}
	if start < len(body) {
		flush(len(body))
	}
}
