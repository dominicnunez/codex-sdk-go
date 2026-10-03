package protocol_test

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Build the wire model independently of the Go union constructors, then compare
// complete semantic scope after decoding and encoding. Strings remain opaque.
func FuzzFilesystemPermissionRoundTrip(f *testing.F) {
	f.Add("../raw", uint64(1), uint8(0), uint8(0))
	f.Add("", ^uint64(0), uint8(2), uint8(7))
	f.Add("**/private/雪", uint64(9007199254740993), uint8(1), uint8(2))
	f.Fuzz(func(t *testing.T, opaque string, depth uint64, access, variant uint8) {
		if len(opaque) > 4096 || !utf8.ValidString(opaque) {
			t.Skip()
		}
		if depth == 0 {
			depth = 1
		}
		mode := []string{"read", "write", "deny"}[access%3]
		var path map[string]any
		switch variant % 8 {
		case 0:
			path = map[string]any{"type": "path", "path": opaque}
		case 1:
			path = map[string]any{"type": "glob_pattern", "pattern": opaque}
		default:
			kind := []string{"root", "minimal", "project_roots", "tmpdir", "slash_tmp", "unknown"}[(variant%8)-2]
			value := map[string]any{"kind": kind}
			if kind == "project_roots" || kind == "unknown" {
				value["subpath"] = opaque
			}
			if kind == "unknown" {
				value["path"] = opaque
			}
			path = map[string]any{"type": "special", "value": value}
		}
		model := map[string]any{"entries": []any{map[string]any{"access": mode, "path": path}}, "globScanMaxDepth": depth, "read": []string{opaque}, "write": []string{opaque}}
		wire, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		var scope codex.AdditionalFileSystemPermissions
		if err := json.Unmarshal(wire, &scope); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(scope)
		if err != nil {
			t.Fatal(err)
		}
		assertPermissionJSON(t, encoded, wire)
	})
}
