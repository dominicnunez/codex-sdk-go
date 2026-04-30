package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

type emptyObjectMethodCase struct {
	name   string
	method string
	call   func(*codex.Client) error
}

func emptyObjectMethodCases() []emptyObjectMethodCase {
	closeStdin := true
	deltaBase64 := "aGVsbG8="
	force := true
	recursive := true
	forceRemoteSync := true

	return []emptyObjectMethodCase{
		{
			name:   "account_logout",
			method: "account/logout",
			call: func(client *codex.Client) error {
				_, err := client.Account.Logout(context.Background())
				return err
			},
		},
		{
			name:   "fs_write_file",
			method: "fs/writeFile",
			call: func(client *codex.Client) error {
				_, err := client.Fs.WriteFile(context.Background(), codex.FsWriteFileParams{
					Path:       "/tmp/file.txt",
					DataBase64: "ZGF0YQ==",
				})
				return err
			},
		},
		{
			name:   "fs_create_directory",
			method: "fs/createDirectory",
			call: func(client *codex.Client) error {
				_, err := client.Fs.CreateDirectory(context.Background(), codex.FsCreateDirectoryParams{
					Path:      "/tmp/dir",
					Recursive: &recursive,
				})
				return err
			},
		},
		{
			name:   "fs_remove",
			method: "fs/remove",
			call: func(client *codex.Client) error {
				_, err := client.Fs.Remove(context.Background(), codex.FsRemoveParams{
					Path:      "/tmp/file.txt",
					Recursive: &recursive,
					Force:     &force,
				})
				return err
			},
		},
		{
			name:   "fs_copy",
			method: "fs/copy",
			call: func(client *codex.Client) error {
				_, err := client.Fs.Copy(context.Background(), codex.FsCopyParams{
					SourcePath:      "/tmp/source.txt",
					DestinationPath: "/tmp/dest.txt",
					Recursive:       &recursive,
				})
				return err
			},
		},
		{
			name:   "command_write",
			method: "command/exec/write",
			call: func(client *codex.Client) error {
				_, err := client.Command.Write(context.Background(), codex.CommandExecWriteParams{
					ProcessID:   "proc-123",
					CloseStdin:  &closeStdin,
					DeltaBase64: &deltaBase64,
				})
				return err
			},
		},
		{
			name:   "command_terminate",
			method: "command/exec/terminate",
			call: func(client *codex.Client) error {
				_, err := client.Command.Terminate(context.Background(), codex.CommandExecTerminateParams{
					ProcessID: "proc-123",
				})
				return err
			},
		},
		{
			name:   "command_resize",
			method: "command/exec/resize",
			call: func(client *codex.Client) error {
				_, err := client.Command.Resize(context.Background(), codex.CommandExecResizeParams{
					ProcessID: "proc-123",
					Size:      codex.CommandExecTerminalSize{Cols: 120, Rows: 40},
				})
				return err
			},
		},
		{
			name:   "turn_interrupt",
			method: "turn/interrupt",
			call: func(client *codex.Client) error {
				_, err := client.Turn.Interrupt(context.Background(), codex.TurnInterruptParams{
					ThreadID: "thread-123",
					TurnID:   "turn-456",
				})
				return err
			},
		},
		{
			name:   "plugin_uninstall",
			method: "plugin/uninstall",
			call: func(client *codex.Client) error {
				_, err := client.Plugin.Uninstall(context.Background(), codex.PluginUninstallParams{
					PluginID:        "plugin-1",
					ForceRemoteSync: &forceRemoteSync,
				})
				return err
			},
		},
		{
			name:   "thread_set_name",
			method: "thread/name/set",
			call: func(client *codex.Client) error {
				_, err := client.Thread.SetName(context.Background(), codex.ThreadSetNameParams{
					ThreadID: "thread-123",
					Name:     "Renamed thread",
				})
				return err
			},
		},
		{
			name:   "thread_archive",
			method: "thread/archive",
			call: func(client *codex.Client) error {
				_, err := client.Thread.Archive(context.Background(), codex.ThreadArchiveParams{
					ThreadID: "thread-123",
				})
				return err
			},
		},
		{
			name:   "thread_compact_start",
			method: "thread/compact/start",
			call: func(client *codex.Client) error {
				_, err := client.Thread.CompactStart(context.Background(), codex.ThreadCompactStartParams{
					ThreadID: "thread-123",
				})
				return err
			},
		},
		{
			name:   "external_agent_config_import",
			method: "externalAgentConfig/import",
			call: func(client *codex.Client) error {
				_, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{
					MigrationItems: []codex.ExternalAgentConfigMigrationItem{
						{Description: "Config file", ItemType: codex.MigrationItemTypeConfig},
					},
				})
				return err
			},
		},
		{
			name:   "mcp_refresh",
			method: "config/mcpServer/reload",
			call: func(client *codex.Client) error {
				_, err := client.Mcp.Refresh(context.Background())
				return err
			},
		},
	}
}

func TestEmptyObjectResponseMethodsAcceptObjectResults(t *testing.T) {
	for _, tc := range emptyObjectMethodCases() {
		t.Run(tc.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)

			if err := transport.SetResponseData(tc.method, map[string]interface{}{}); err != nil {
				t.Fatalf("SetResponseData(%q): %v", tc.method, err)
			}

			if err := tc.call(client); err != nil {
				t.Fatalf("call failed: %v", err)
			}
		})
	}
}

func TestEmptyObjectResponseMethodsRejectMissingNullAndNonObjectResults(t *testing.T) {
	badResponses := []struct {
		name            string
		response        codex.Response
		wantEmptyResult bool
	}{
		{
			name:            "missing_result",
			response:        codex.Response{JSONRPC: "2.0"},
			wantEmptyResult: true,
		},
		{
			name: "null_result",
			response: codex.Response{
				JSONRPC: "2.0",
				Result:  json.RawMessage(`null`),
			},
			wantEmptyResult: true,
		},
		{
			name: "non_object_result",
			response: codex.Response{
				JSONRPC: "2.0",
				Result:  json.RawMessage(`[]`),
			},
		},
	}

	for _, tc := range emptyObjectMethodCases() {
		for _, bad := range badResponses {
			t.Run(tc.name+"/"+bad.name, func(t *testing.T) {
				transport := NewMockTransport()
				client := codex.NewClient(transport)
				transport.SetResponse(tc.method, bad.response)

				err := tc.call(client)
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if bad.wantEmptyResult {
					if !errors.Is(err, codex.ErrEmptyResult) {
						t.Fatalf("error = %v; want ErrEmptyResult", err)
					}
					return
				}

				if errors.Is(err, codex.ErrEmptyResult) {
					t.Fatalf("error = %v; did not expect ErrEmptyResult", err)
				}
				if !errors.Is(err, codex.ErrResultNotObject) {
					t.Fatalf("error = %v; want ErrResultNotObject", err)
				}
				if !strings.Contains(err.Error(), "cannot unmarshal") {
					t.Fatalf("error = %v; want JSON shape failure details", err)
				}
			})
		}
	}
}
