package protocol

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Inner SDK codecs must bound native diagnostics before encoding/json wraps
// them. An outer client helper cannot distinguish an omitted inner SDK owner
// from an application codec's error chain.
func TestProtocolUsesOwnedJSONSerializer(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if name != "encoding/json" {
				continue
			}
			alias := "json"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			if alias == "." {
				t.Errorf("%s: dot JSON imports obscure serialization ownership", path)
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
				owner, ok := selector.X.(*ast.Ident)
				if ok && owner.Name == alias && (selector.Sel.Name == "Marshal" || selector.Sel.Name == "MarshalIndent" || selector.Sel.Name == "NewEncoder") {
					t.Errorf("%s: SDK protocol serialization must use its owned diagnostic boundary", positions.Position(call.Pos()))
				}
				return true
			})
		}
	}
}
