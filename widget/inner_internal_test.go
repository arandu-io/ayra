package widget

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// widgetSources parses every non-test Go file of this package, by file name.
func widgetSources(t *testing.T) map[string]*ast.File {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		files[path] = file
	}
	return files
}

// TestAWidgetDrawsNoIconThatIsNotInTheTable is the half of the guard that reads
// this package. The icons a component draws inside itself are a decision of
// Kyse's, and the table in inner.go is where this half records it; an
// `icons.Something()` anywhere else is a second source for the same decision,
// which is how a triangle where Kyse draws a caret came to be.
func TestAWidgetDrawsNoIconThatIsNotInTheTable(t *testing.T) {
	for path, file := range widgetSources(t) {
		if path == "inner.go" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "icons" && selector.Sel.Name != "ParsePath" {
				t.Errorf("%s calls icons.%s: a widget gets its inner icons from innerIcon, so that the table of what it draws is the whole of it", path, selector.Sel.Name)
			}
			return true
		})
	}
}

// TestEveryInnerIconIsDrawnAndEveryDrawnOneIsDeclared reads the calls to
// innerIcon and the table, and fails on either side without the other: a name
// asked for that the table does not hold draws nothing, and a name in the table
// that nothing asks for is a claim about what a widget draws that is not true.
func TestEveryInnerIconIsDrawnAndEveryDrawnOneIsDeclared(t *testing.T) {
	components := map[string]bool{} // string literals anywhere in the sources
	asked := map[string]bool{}      // second arguments of innerIcon calls
	literalPairs := [][2]string{}   // calls whose both arguments are literals
	for path, file := range widgetSources(t) {
		if path == "inner.go" {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					if value, err := strconv.Unquote(n.Value); err == nil {
						components[value] = true
					}
				}
			case *ast.CallExpr:
				ident, ok := n.Fun.(*ast.Ident)
				if !ok || ident.Name != "innerIcon" || len(n.Args) != 2 {
					return true
				}
				name, ok := n.Args[1].(*ast.BasicLit)
				if !ok {
					t.Errorf("%s asks innerIcon for an icon by a name that is not written out", path)
					return true
				}
				icon, _ := strconv.Unquote(name.Value)
				asked[icon] = true
				if component, ok := n.Args[0].(*ast.BasicLit); ok {
					value, _ := strconv.Unquote(component.Value)
					literalPairs = append(literalPairs, [2]string{value, icon})
				}
			}
			return true
		})
	}

	for _, pair := range literalPairs {
		if !declared(pair[0], pair[1]) {
			t.Errorf("a widget asks for %s of %q, which the table does not hold: it draws nothing", pair[1], pair[0])
		}
	}
	for _, component := range declaredComponents() {
		if !components[component] {
			t.Errorf("the table holds %q and no widget names it", component)
		}
		for name := range innerIcons[component] {
			if !asked[name] {
				t.Errorf("the table holds %s for %q and no widget asks for it", name, component)
			}
			if innerIcon(component, name).IsZero() {
				t.Errorf("%s of %q is the zero icon", name, component)
			}
		}
	}
}

func TestTheTickIsDrawnWithoutItsShade(t *testing.T) {
	if got := innerIcon("checkbox", "Check"); got.Shade() != "" || got.Body() == "" {
		t.Errorf("the tick of a checkbox has a shade %q: Kyse draws it as a stroke", got.Shade())
	}
	if got := innerIcon("accordion", "CaretDown"); got.Shade() == "" {
		t.Errorf("a caret lost its lighter half")
	}
}
