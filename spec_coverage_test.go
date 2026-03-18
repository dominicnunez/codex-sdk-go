package codex

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecCoverage verifies that every JSON schema in specs/ has a corresponding Go type.
// This ensures the SDK covers the entire Codex protocol specification.
func TestSpecCoverage(t *testing.T) {
	// Map schema filenames to Go type names
	schemaToType := make(map[string]string)

	// Walk through all spec files
	err := filepath.Walk("specs", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".json") && !strings.Contains(path, "EventMsg.json") {
			// Extract type name from filename (remove .json extension)
			filename := filepath.Base(path)
			typeName := strings.TrimSuffix(filename, ".json")
			schemaToType[path] = typeName
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk specs directory: %v", err)
	}

	if len(schemaToType) == 0 {
		t.Fatal("found 0 JSON schemas in specs/ — tests must run from the package directory")
	}
	t.Logf("Found %d JSON schemas in specs/", len(schemaToType))

	// Track missing types
	var missing []string
	var found []string
	declaredTypes := loadDeclaredTypes(t)

	// For each schema, check if we have the Go type defined
	for schemaPath, typeName := range schemaToType {
		// Read the schema to understand its structure
		data, err := os.ReadFile(schemaPath)
		if err != nil {
			t.Errorf("Failed to read schema %s: %v", schemaPath, err)
			continue
		}

		// Parse schema to understand if it's a top-level type or embedded
		var schema map[string]interface{}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Errorf("Failed to parse schema %s: %v", schemaPath, err)
			continue
		}

		// Check if this type exists in our codebase
		typeExists := declaredTypes[typeName]

		if typeExists {
			found = append(found, typeName)
		} else {
			missing = append(missing, typeName+" ("+schemaPath+")")
		}
	}

	t.Logf("Coverage: %d/%d types implemented", len(found), len(schemaToType))

	// Classify missing types: implemented differently vs actually missing.
	//
	// "Implemented differently" means the spec type has a Go equivalent under
	// a different name or pattern:
	//
	//   JSON-RPC envelope types (6) — implemented as Request, Response,
	//     Notification, Error, RequestID in jsonrpc.go with Go-idiomatic names.
	//     JSONRPCMessage is a union of the other three; Go uses separate types.
	//
	//   Method dispatch enums (4) — ClientRequest, ServerRequest,
	//     ServerNotification, ClientNotification are string enums listing every
	//     method name. Go dispatches via switch statements and per-service
	//     method strings instead of a standalone enum type.
	//
	//   RequestId (1) — implemented as RequestID (Go convention capitalizes
	//     acronyms).
	//
	//   codex_app_server_protocol.schemas (1) — root schema container file,
	//     not a type. All nested definitions are implemented as individual
	//     Go types in their respective domain files.
	//
	//   RawResponseItemCompletedNotification (1) — schema file exists at
	//     specs/v2/ but the type is not referenced in ServerNotification.json.
	//     It is not part of the wire protocol; implementing it would be dead code.
	//
	var implementedDifferently []string
	var actualGaps []string

	for _, m := range missing {
		if strings.Contains(m, "JSONRPCRequest") || strings.Contains(m, "JSONRPCResponse") ||
			strings.Contains(m, "JSONRPCNotification") || strings.Contains(m, "JSONRPCMessage") ||
			strings.Contains(m, "JSONRPCError") || strings.Contains(m, "JSONRPCErrorError") ||
			strings.HasPrefix(m, "ServerNotification (") || strings.HasPrefix(m, "ClientNotification (") ||
			strings.HasPrefix(m, "ServerRequest (") || strings.HasPrefix(m, "ClientRequest (") ||
			strings.Contains(m, "RequestId (") ||
			strings.Contains(m, "codex_app_server_protocol.schemas") ||
			strings.Contains(m, "codex_app_server_protocol.v2.schemas") ||
			strings.Contains(m, "RawResponseItemCompletedNotification") {
			implementedDifferently = append(implementedDifferently, m)
		} else {
			actualGaps = append(actualGaps, m)
		}
	}

	if len(implementedDifferently) > 0 {
		t.Logf("Implemented differently (Go-idiomatic equivalents exist):")
		for _, g := range implementedDifferently {
			t.Logf("  - %s", g)
		}
	}

	if len(actualGaps) > 0 {
		t.Errorf("MISSING TYPES - these should be implemented:")
		for _, g := range actualGaps {
			t.Errorf("  - %s", g)
		}
	} else {
		t.Logf("✓ All expected types are implemented")
	}
}

func loadDeclaredTypes(t *testing.T) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	declared := make(map[string]bool)
	goFilesSeen := false

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		goFilesSeen = true
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				declared[typeSpec.Name.Name] = true
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("loadDeclaredTypes failed: %v", err)
	}
	if !goFilesSeen {
		t.Fatal("loadDeclaredTypes found no .go files — tests must run from the package directory")
	}

	return declared
}
