package module_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"

	hesapehttp "github.com/arandu-io/hesape/http"
)

// TestTheClientAsksForWhatTheServerAnswers is the one place the two halves of
// the negotiation are compared.
//
// The client spells the media type itself, because it runs on a device and
// must not carry the server's packages to say one string. Two spellings can
// drift, and the drift is quiet: the client asks for something the server does
// not know, the server answers the browser's question instead, and markup
// arrives where values were expected -- at the far end, on somebody's phone.
//
// This module is where the comparison can happen, because it is the half that
// already has the server in its graph. It reads the client's constant out of
// the source rather than importing it: importing would put the drawing half,
// and the engine under it, into the graph of every server that registers this.
func TestTheClientAsksForWhatTheServerAnswers(t *testing.T) {
	const (
		file = "../client/client.go"
		name = "ViewMediaType"
	)

	asked := constantIn(t, file, name)

	if asked != hesapehttp.ViewDataMediaType {
		t.Errorf(
			"the client asks for %q and the server answers %q, so every page would arrive as markup",
			asked, hesapehttp.ViewDataMediaType,
		)
	}
}

// constantIn answers the string a named constant is declared with.
//
// It fails rather than answering an empty string when the constant is gone: a
// rename that this could not find would otherwise compare "" against the
// server's spelling, report a mismatch, and send whoever reads it looking for
// a drift that is really a missing declaration.
func constantIn(t *testing.T, file, name string) string {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash(file), nil, 0)
	if err != nil {
		t.Fatalf("%s could not be read: %v", file, err)
	}

	for _, declaration := range parsed.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, declared := range value.Names {
				if declared.Name != name || i >= len(value.Values) {
					continue
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatalf("%s.%s is not a string literal, and this can only compare one", file, name)
				}
				unquoted, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatalf("%s.%s could not be read: %v", file, name, err)
				}
				return unquoted
			}
		}
	}

	t.Fatalf("%s declares no constant named %s, so the two halves of the negotiation cannot be compared", file, name)
	return ""
}
