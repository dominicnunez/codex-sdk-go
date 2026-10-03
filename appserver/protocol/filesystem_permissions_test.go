package protocol_test

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestStructuredPermissionsApprovalRoundTrip(t *testing.T) {
	filesystem := `{"entries":[{"access":"write","path":{"type":"path","path":"relative/../output"}},{"access":"deny","path":{"type":"glob_pattern","pattern":"**/private/*"}},{"access":"read","path":{"type":"special","value":{"kind":"project_roots","subpath":"../shared"}}}],"globScanMaxDepth":3,"read":["../legacy"],"write":["raw/../legacy"]}`
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	called := false
	client.SetApprovalHandlers(codex.ApprovalHandlers{OnPermissionsRequestApproval: func(_ context.Context, params codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
		called = true
		got, err := json.Marshal(params.Permissions.FileSystem)
		if err != nil {
			t.Fatal(err)
		}
		assertPermissionJSON(t, got, []byte(filesystem))
		return codex.PermissionsRequestApprovalResponse{Permissions: codex.GrantedPermissionProfile{FileSystem: params.Permissions.FileSystem}}, nil
	}})
	response, err := mock.InjectServerRequest(context.Background(), codex.Request{JSONRPC: "2.0", ID: codex.RequestID{Value: 7}, Method: "item/permissions/requestApproval", Params: json.RawMessage(`{"cwd":"relative","itemId":"item","threadId":"thread","turnId":"turn","startedAtMs":1,"permissions":{"fileSystem":` + filesystem + `}}`)})
	if err != nil || response.Error != nil || !called {
		t.Fatalf("approval called=%v response=%+v err=%v", called, response, err)
	}
	assertPermissionJSON(t, response.Result, []byte(`{"permissions":{"fileSystem":`+filesystem+`}}`))
}

func assertPermissionJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue, wantValue any
	gotDecoder := json.NewDecoder(bytes.NewReader(got))
	gotDecoder.UseNumber()
	if err := gotDecoder.Decode(&gotValue); err != nil {
		t.Fatal(err)
	}
	wantDecoder := json.NewDecoder(bytes.NewReader(want))
	wantDecoder.UseNumber()
	if err := wantDecoder.Decode(&wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("permissions changed: got=%s want=%s", got, want)
	}
}
