package appserver_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	protocol "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	transport "github.com/dominicnunez/codex-sdk-go/appserver/transport"
)

func TestFilesystemApprovalStdioPublication(t *testing.T) {
	for _, scenario := range []string{"valid", "invalid-request", "invalid-grant"} {
		t.Run(scenario, func(t *testing.T) {
			clientReader, serverWriter := io.Pipe()
			serverReader, clientWriter := io.Pipe()
			t.Cleanup(func() {
				_ = clientReader.Close()
				_ = serverWriter.Close()
				_ = serverReader.Close()
				_ = clientWriter.Close()
			})
			stdio := transport.NewStdioTransport(clientReader, clientWriter)
			t.Cleanup(func() { _ = stdio.Close() })
			client := protocol.NewClient(stdio)
			var calls atomic.Int32
			client.SetApprovalHandlers(protocol.ApprovalHandlers{OnPermissionsRequestApproval: func(_ context.Context, params protocol.PermissionsRequestApprovalParams) (protocol.PermissionsRequestApprovalResponse, error) {
				calls.Add(1)
				fs := params.Permissions.FileSystem
				if scenario == "invalid-grant" {
					zero := uint64(0)
					fs = &protocol.AdditionalFileSystemPermissions{GlobScanMaxDepth: &zero}
				}
				return protocol.PermissionsRequestApprovalResponse{Permissions: protocol.GrantedPermissionProfile{FileSystem: fs}}, nil
			}})
			responses := make(chan []byte, 1)
			go func() {
				scanner := bufio.NewScanner(serverReader)
				if scanner.Scan() {
					responses <- append([]byte(nil), scanner.Bytes()...)
				}
			}()
			fs := `{"entries":[],"read":["../legacy"],"globScanMaxDepth":18446744073709551615}`
			if scenario == "invalid-request" {
				fs = `{"entries":[null]}`
			}
			request := `{"jsonrpc":"2.0","id":"scope","method":"item/permissions/requestApproval","params":{"cwd":"relative","itemId":"item","threadId":"thread","turnId":"turn","startedAtMs":1,"permissions":{"fileSystem":` + fs + `}}}` + "\n"
			written := make(chan error, 1)
			go func() { _, err := io.WriteString(serverWriter, request); written <- err }()
			select {
			case err := <-written:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("request write timed out")
			}
			var wire []byte
			select {
			case wire = <-responses:
			case <-time.After(5 * time.Second):
				t.Fatal("approval response timed out")
			}
			var response protocol.Response
			if err := json.Unmarshal(wire, &response); err != nil {
				t.Fatal(err)
			}
			if scenario == "valid" {
				if calls.Load() != 1 || response.Error != nil || !strings.Contains(string(response.Result), `"entries":[]`) || !strings.Contains(string(response.Result), `18446744073709551615`) {
					t.Fatalf("valid scope changed: calls=%d response=%s", calls.Load(), wire)
				}
				return
			}
			code, expectedCalls := protocol.ErrCodeInvalidParams, int32(0)
			if scenario == "invalid-grant" {
				code, expectedCalls = protocol.ErrCodeInternalError, 1
			}
			if calls.Load() != expectedCalls || response.Error == nil || response.Error.Code != code || len(response.Result) != 0 {
				t.Fatalf("invalid scope produced wrong publication: calls=%d response=%s", calls.Load(), wire)
			}
		})
	}
}
