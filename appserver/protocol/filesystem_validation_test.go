package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestFilesystemPermissionVariants(t *testing.T) {
	paths := []string{
		`{"type":"path","path":""}`, `{"type":"path","path":"relative/../raw"}`,
		`{"type":"glob_pattern","pattern":""}`, `{"type":"glob_pattern","pattern":"**/../*.go"}`,
	}
	for _, kind := range []string{"root", "minimal", "project_roots", "tmpdir", "slash_tmp", "unknown"} {
		value := fmt.Sprintf(`{"kind":%q}`, kind)
		if kind == "project_roots" {
			value = `{"kind":"project_roots","subpath":"../raw"}`
		}
		if kind == "unknown" {
			value = `{"kind":"unknown","path":"","subpath":""}`
		}
		paths = append(paths, `{"type":"special","value":`+value+`}`)
	}
	for _, access := range []string{"read", "write", "deny"} {
		for _, path := range paths {
			input := fmt.Sprintf(`{"entries":[{"access":%q,"path":%s},{"access":%q,"path":%s}],"globScanMaxDepth":18446744073709551615}`, access, path, access, path)
			var scope codex.AdditionalFileSystemPermissions
			if err := json.Unmarshal([]byte(input), &scope); err != nil {
				t.Fatal(input, err)
			}
			output, err := json.Marshal(scope)
			if err != nil {
				t.Fatal(err)
			}
			// JSON text equality is normalized using Number, not float64, elsewhere.
			assertPermissionJSON(t, output, []byte(input))
			if !strings.Contains(string(output), `18446744073709551615`) {
				t.Fatalf("depth lost: %s", output)
			}
		}
	}
}

func TestFilesystemProgrammaticUnionValues(t *testing.T) {
	values := []codex.FileSystemPath{
		codex.PathFileSystemPath{Path: "../raw"}, codex.GlobPatternFileSystemPath{Pattern: "**/raw"},
	}
	for _, value := range []codex.FileSystemSpecialPath{
		codex.RootFileSystemSpecialPath{}, codex.MinimalFileSystemSpecialPath{},
		codex.ProjectRootsFileSystemSpecialPath{Subpath: ptr("../raw")},
		codex.TmpdirFileSystemSpecialPath{}, codex.SlashTmpFileSystemSpecialPath{},
		codex.UnknownFileSystemSpecialPath{Path: "", Subpath: ptr("")},
	} {
		pointer := reflect.New(reflect.TypeOf(value))
		pointer.Elem().Set(reflect.ValueOf(value))
		for _, special := range []codex.FileSystemSpecialPath{value, pointer.Interface().(codex.FileSystemSpecialPath)} {
			values = append(values, codex.SpecialFileSystemPath{Value: codex.FileSystemSpecialPathWrapper{Value: special}})
		}
	}
	for _, value := range values {
		pointer := reflect.New(reflect.TypeOf(value))
		pointer.Elem().Set(reflect.ValueOf(value))
		for _, path := range []codex.FileSystemPath{value, pointer.Interface().(codex.FileSystemPath)} {
			entry := codex.FileSystemSandboxEntry{Access: codex.FileSystemAccessModeDeny, Path: codex.FileSystemPathWrapper{Value: path}}
			wire, err := json.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			var decoded codex.FileSystemSandboxEntry
			if err := json.Unmarshal(wire, &decoded); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(decoded)
			if err != nil {
				t.Fatal(err)
			}
			assertPermissionJSON(t, encoded, wire)
		}
	}
}

func TestFilesystemDirectValidationAndReceiverIsolation(t *testing.T) {
	for _, invalid := range []string{
		`null`, `[]`, `{"globScanMaxDepth":"3"}`,
		`{"entries":[{"access":"read","path":{"type":null,"path":""}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":null}}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":"unknown","path":null}}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":99,"kind":"root"}}}]}`,
		`{"entries":[{"access":"read","path":{"type":"path","path":null,"path":"later"}}]}`,
		`{"entries":[{"access":"read","path":null,"path":{"type":"path","path":"later"}}]}`,
	} {
		scope := codex.AdditionalFileSystemPermissions{Read: []string{"original"}}
		if err := json.Unmarshal([]byte(invalid), &scope); err == nil {
			t.Fatalf("invalid scope accepted: %s", invalid)
		}
		if len(scope.Read) != 1 || scope.Read[0] != "original" || scope.Entries != nil {
			t.Fatalf("failed decode changed receiver: %+v", scope)
		}
	}
	// The profile closes its property set; nested filesystem/path objects are
	// open and unrelated variant properties do not change selected-field rules.
	var profile codex.RequestPermissionProfile
	if err := json.Unmarshal([]byte(`{"extra":true}`), &profile); err == nil {
		t.Fatal("unknown request profile property accepted")
	}
	if err := json.Unmarshal([]byte(`{"fileSystem":{"extra":true,"entries":[{"access":"read","extra":true,"path":{"type":"path","path":"","pattern":99}}]}}`), &profile); err != nil {
		t.Fatal(err)
	}
	for _, special := range []string{
		`{"kind":"project_roots"}`, `{"kind":"project_roots","subpath":null}`,
		`{"kind":"unknown","path":""}`, `{"kind":"unknown","path":"","subpath":null}`,
		`{"kind":"root","extra":true}`, `{"kind":"project_roots","extra":true}`,
	} {
		var scope codex.AdditionalFileSystemPermissions
		if err := json.Unmarshal([]byte(`{"entries":[{"access":"read","path":{"type":"special","extra":true,"value":`+special+`}}]}`), &scope); err != nil {
			t.Fatal(special, err)
		}
		if _, err := json.Marshal(scope); err != nil {
			t.Fatal(special, err)
		}
	}
}

func TestInvalidFilesystemScopeStopsApproval(t *testing.T) {
	for _, scope := range []string{
		`{"entries":[null]}`, `{"entries":[{}]}`, `{"entries":[{"access":null,"path":{"type":"path","path":"x"}}]}`,
		`{"entries":[{"access":"future","path":{"type":"path","path":"x"}}]}`,
		`{"entries":[{"access":"read","path":null}]}`,
		`{"entries":[{"access":"read","path":{}}]}`,
		`{"entries":[{"access":"read","path":{"type":"path"}}]}`,
		`{"entries":[{"access":"read","path":{"type":"path","path":null}}]}`,
		`{"entries":[{"access":"read","path":{"type":"glob_pattern","pattern":99}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":null}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":"unknown"}}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":"future"}}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":"project_roots","subpath":99}}}]}`,
		`{"entries":[{"access":"read","path":{"type":"path","path":99,"path":"later"}}]}`,
		`{"entries":[{"access":99,"access":"read","path":{"type":"path","path":"x"}}]}`,
		`{"entries":[{"access":"read","path":{"type":"future","type":"path","path":"x"}}]}`,
		`{"entries":[{"access":"read","path":{"type":"special","value":{"kind":"future","kind":"root"}}}]}`,
		`{"globScanMaxDepth":0}`, `{"globScanMaxDepth":-1}`, `{"globScanMaxDepth":1.5}`, `{"globScanMaxDepth":18446744073709551616}`,
		`{"globScanMaxDepth":0,"globScanMaxDepth":3}`, `{"read":[null],"read":["later"]}`, `{"write":[null]}`,
	} {
		t.Run(scope, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			called := false
			client.SetApprovalHandlers(codex.ApprovalHandlers{OnPermissionsRequestApproval: func(context.Context, codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
				called = true
				return codex.PermissionsRequestApprovalResponse{}, nil
			}})
			response, err := mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: 7}, Method: "item/permissions/requestApproval", Params: json.RawMessage(`{"cwd":"","itemId":"item","threadId":"thread","turnId":"turn","startedAtMs":0,"permissions":{"fileSystem":` + scope + `}}`)})
			if called || len(response.Result) != 0 || (err == nil && response.Error == nil) {
				t.Fatalf("invalid scope admitted: called=%v response=%+v err=%v", called, response, err)
			}
			if !errors.Is(err, codex.ErrInvalidParams) {
				t.Fatalf("invalid scope lost error classification: %v", err)
			}
		})
	}
}

func TestInvalidGrantNeverProducesSuccess(t *testing.T) {
	zero := uint64(0)
	var nilPath *codex.PathFileSystemPath
	var nilSpecial *codex.RootFileSystemSpecialPath
	for _, fs := range []codex.AdditionalFileSystemPermissions{
		{GlobScanMaxDepth: &zero},
		{Entries: ptr([]codex.FileSystemSandboxEntry{{Access: "future", Path: codex.FileSystemPathWrapper{Value: codex.PathFileSystemPath{Path: "x"}}}})},
		{Entries: ptr([]codex.FileSystemSandboxEntry{{Access: codex.FileSystemAccessModeRead}})},
		{Entries: ptr([]codex.FileSystemSandboxEntry{{Access: codex.FileSystemAccessModeRead, Path: codex.FileSystemPathWrapper{Value: nilPath}}})},
		{Entries: ptr([]codex.FileSystemSandboxEntry{{Access: codex.FileSystemAccessModeRead, Path: codex.FileSystemPathWrapper{Value: codex.SpecialFileSystemPath{Value: codex.FileSystemSpecialPathWrapper{Value: nilSpecial}}}}})},
	} {
		mock := NewMockTransport()
		client := codex.NewClient(mock)
		client.SetApprovalHandlers(codex.ApprovalHandlers{OnPermissionsRequestApproval: func(context.Context, codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
			return codex.PermissionsRequestApprovalResponse{Permissions: codex.GrantedPermissionProfile{FileSystem: &fs}}, nil
		}})
		response, err := mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: 7}, Method: "item/permissions/requestApproval", Params: json.RawMessage(`{"cwd":"","itemId":"item","threadId":"thread","turnId":"turn","startedAtMs":0,"permissions":{}}`)})
		if len(response.Result) != 0 || (err == nil && response.Error == nil) {
			t.Fatalf("invalid grant published: response=%+v err=%v", response, err)
		}
	}
}

func TestOptionalFilesystemValues(t *testing.T) {
	for _, input := range []string{`{}`, `{"entries":null,"globScanMaxDepth":null}`, `{"entries":[]}`, `{"entries":[],"read":[],"write":[]}`, `{"read":null,"write":null}`} {
		var scope codex.AdditionalFileSystemPermissions
		if err := json.Unmarshal([]byte(input), &scope); err != nil {
			t.Fatal(err)
		}
		output, err := json.Marshal(scope)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(input, `"entries":[]`) && !strings.Contains(string(output), `"entries":[]`) {
			t.Fatalf("explicit empty entries lost: %s", output)
		}
	}
}
