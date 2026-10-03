package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func captureOpaqueApproval(t *testing.T, method string, payload any) (any, codex.Response, error) {
	t.Helper()
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	defer client.Close()
	var received any
	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnCommandExecutionRequestApproval: func(_ context.Context, p codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
			received = p
			return codex.CommandExecutionRequestApprovalResponse{Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "decline"}}, nil
		},
		OnExecCommandApproval: func(_ context.Context, p codex.ExecCommandApprovalParams) (codex.ExecCommandApprovalResponse, error) {
			received = p
			return codex.ExecCommandApprovalResponse{Decision: codex.ReviewDecisionWrapper{Value: "abort"}}, nil
		},
		OnFileChangeRequestApproval: func(_ context.Context, p codex.FileChangeRequestApprovalParams) (codex.FileChangeRequestApprovalResponse, error) {
			received = p
			return codex.FileChangeRequestApprovalResponse{Decision: codex.FileChangeApprovalDecisionDecline}, nil
		},
		OnApplyPatchApproval: func(_ context.Context, p codex.ApplyPatchApprovalParams) (codex.ApplyPatchApprovalResponse, error) {
			received = p
			return codex.ApplyPatchApprovalResponse{Decision: codex.ReviewDecisionWrapper{Value: "abort"}}, nil
		},
	})
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response, err := mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: 1}, Method: method, Params: data})
	return received, response, err
}

func dispatchOpaqueApproval(t *testing.T, method string, payload any) {
	t.Helper()
	received, response, err := captureOpaqueApproval(t, method, payload)
	if err != nil || response.Error != nil || received == nil {
		t.Errorf("%s: handler not reached, response=%+v err=%v", method, response, err)
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	requireWireFields(t, received, string(data))
	var result struct {
		Decision string `json:"decision"`
	}
	want := "decline"
	if method == "execCommandApproval" || method == "applyPatchApproval" {
		want = "abort"
	}
	if err := json.Unmarshal(response.Result, &result); err != nil || result.Decision != want {
		t.Fatalf("handler rejection not retained: %s, %v", response.Result, err)
	}
}

func TestOpaqueApprovalMalformedPaths(t *testing.T) {
	for _, method := range []string{"item/commandExecution/requestApproval", "execCommandApproval", "item/fileChange/requestApproval", "applyPatchApproval"} {
		t.Run(method, func(t *testing.T) {
			base := map[string]any{"itemId": "item", "startedAtMs": 1, "threadId": "thread", "turnId": "turn"}
			field := "cwd"
			if method == "execCommandApproval" || method == "applyPatchApproval" {
				base = map[string]any{"callId": "call", "conversationId": "thread", "cwd": "relative", "command": []string{}, "parsedCmd": []any{}, "fileChanges": map[string]any{}}
			}
			if method == "item/fileChange/requestApproval" || method == "applyPatchApproval" {
				field = "grantRoot"
			}
			check := func(name string, payload map[string]any) {
				t.Helper()
				t.Run(name, func(t *testing.T) {
					received, response, err := captureOpaqueApproval(t, method, payload)
					if received != nil || !errors.Is(err, codex.ErrInvalidParams) || response.Result != nil {
						t.Fatalf("malformed path admitted: callback=%+v response=%+v err=%v", received, response, err)
					}
				})
			}
			base[field] = 123
			check("wrong-type-"+field, base)
			if method == "execCommandApproval" {
				base[field] = nil
				check("null-required-cwd", base)
				delete(base, field)
				check("missing-required-cwd", base)
			}
			if field != "cwd" {
				return
			}
			base["cwd"] = "relative"
			read := map[string]any{"type": "read", "name": "file"}
			if method == "execCommandApproval" {
				read["cmd"] = "cat"
				base["parsedCmd"] = []any{read}
			} else {
				read["command"] = "cat"
				base["commandActions"] = []any{read}
			}
			check("missing-required-read-path", base)
			read["path"] = nil
			check("null-required-read-path", base)
			read["path"] = 123
			check("wrong-type-read-path", base)
		})
	}
}

func TestOpaqueLegacyOptionalParsedPaths(t *testing.T) {
	for _, present := range []bool{false, true} {
		list := map[string]any{"type": "list_files", "cmd": "ls"}
		search := map[string]any{"type": "search", "cmd": "rg"}
		if present {
			list["path"], search["path"] = nil, nil
		}
		received, response, err := captureOpaqueApproval(t, "execCommandApproval", map[string]any{
			"callId": "call", "conversationId": "thread", "cwd": "relative", "command": []string{}, "parsedCmd": []any{list, search},
		})
		if err != nil || response.Error != nil || received == nil {
			t.Fatalf("optional parsed path dispatch failed: %+v %v", response, err)
		}
		params := received.(codex.ExecCommandApprovalParams)
		if params.ParsedCmd[0].Value.(*codex.ListFilesParsedCommand).Path != nil || params.ParsedCmd[1].Value.(*codex.SearchParsedCommand).Path != nil {
			t.Fatal("absent or null legacy optional path changed")
		}
	}
}

func TestOpaqueApprovalPathDispatch(t *testing.T) {
	for _, value := range []string{"relative/workspace", "../work", "/work/../repo", `C:\work\..\repo`, "", "空間/資料", "opaque\x00value"} {
		t.Run(value, func(t *testing.T) {
			dispatchOpaqueApproval(t, "item/commandExecution/requestApproval", map[string]any{
				"itemId": "item", "startedAtMs": 1, "threadId": "thread", "turnId": "turn", "cwd": value,
			})
			dispatchOpaqueApproval(t, "execCommandApproval", map[string]any{
				"callId": "call", "conversationId": "thread", "cwd": value, "command": []string{}, "parsedCmd": []any{},
			})
			dispatchOpaqueApproval(t, "item/fileChange/requestApproval", map[string]any{
				"itemId": "item", "startedAtMs": 1, "threadId": "thread", "turnId": "turn", "grantRoot": value,
			})
			dispatchOpaqueApproval(t, "applyPatchApproval", map[string]any{
				"callId": "call", "conversationId": "thread", "grantRoot": value,
				"fileChanges": map[string]any{value: map[string]any{"type": "update", "unified_diff": "diff", "move_path": value}},
			})
		})
	}
}

func TestOpaqueCommandPathsWithAndWithoutCwd(t *testing.T) {
	for _, cwd := range []string{"", "relative/base", "/workspace/project"} {
		for _, value := range []string{"../file", "/work/../file", "", `C:\work\..\file`} {
			modern := map[string]any{"itemId": "item", "startedAtMs": 1, "threadId": "thread", "turnId": "turn"}
			modern["commandActions"] = []any{
				map[string]any{"type": "read", "command": "cat", "name": "file", "path": value},
				map[string]any{"type": "listFiles", "command": "ls", "path": value},
				map[string]any{"type": "search", "command": "rg", "path": value},
			}
			if cwd != "" {
				modern["cwd"] = cwd
			}
			dispatchOpaqueApproval(t, "item/commandExecution/requestApproval", modern)
			dispatchOpaqueApproval(t, "execCommandApproval", map[string]any{
				"callId": "call", "conversationId": "thread", "cwd": cwd, "command": []string{},
				"parsedCmd": []any{
					map[string]any{"type": "read", "cmd": "cat", "name": "file", "path": value},
					map[string]any{"type": "list_files", "cmd": "ls", "path": value},
					map[string]any{"type": "search", "cmd": "rg", "path": value},
				},
			})
		}
	}
}

func TestOpaqueApprovalNullablePaths(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	defer client.Close()
	var calls int
	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnCommandExecutionRequestApproval: func(_ context.Context, p codex.CommandExecutionRequestApprovalParams) (codex.CommandExecutionRequestApprovalResponse, error) {
			calls++
			if p.Cwd != nil || p.CommandActions == nil || (*p.CommandActions)[0].Value.(*codex.ListFilesCommandAction).Path != nil || (*p.CommandActions)[1].Value.(*codex.SearchCommandAction).Path != nil {
				t.Error("null or absent optional paths changed")
			}
			return codex.CommandExecutionRequestApprovalResponse{Decision: codex.CommandExecutionApprovalDecisionWrapper{Value: "decline"}}, nil
		},
	})
	for _, cwd := range []string{``, `,"cwd":null`} {
		response, err := mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: 1}, Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"itemId":"item","startedAtMs":1,"threadId":"thread","turnId":"turn","commandActions":[{"type":"listFiles","command":"ls"},{"type":"search","command":"rg","path":null}]` + cwd + `}`)})
		if err != nil || response.Error != nil {
			t.Fatalf("nullable path dispatch failed: %+v %v", response, err)
		}
	}
	if calls != 2 {
		t.Fatalf("callback calls=%d, want 2", calls)
	}
	for _, raw := range []string{`{}`, `{"grantRoot":null}`} {
		var legacy codex.ApplyPatchApprovalParams
		var current codex.FileChangeRequestApprovalParams
		var optional map[string]any
		if err := json.Unmarshal([]byte(raw), &optional); err != nil {
			t.Fatal(err)
		}
		optional["callId"], optional["conversationId"], optional["fileChanges"] = "call", "thread", map[string]any{}
		data, err := json.Marshal(optional)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &legacy); err != nil || legacy.GrantRoot != nil {
			t.Fatalf("legacy nullable grantRoot: %+v %v", legacy, err)
		}
		optional["itemId"], optional["startedAtMs"], optional["threadId"], optional["turnId"] = "item", 1, "thread", "turn"
		data, err = json.Marshal(optional)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &current); err != nil || current.GrantRoot != nil {
			t.Fatalf("current nullable grantRoot: %+v %v", current, err)
		}
	}
}

func FuzzOpaqueApprovalPathPreservation(f *testing.F) {
	for _, value := range []string{"", "../path", "/a/../b", `C:\a\..\b`, "opaque\x00value", "空間/資料"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 4096 {
			t.Skip()
		}
		dispatchOpaqueApproval(t, "item/commandExecution/requestApproval", map[string]any{
			"itemId": "item", "startedAtMs": 1, "threadId": "thread", "turnId": "turn", "cwd": value,
			"commandActions": []any{map[string]any{"type": "read", "command": "cat", "name": "file", "path": value}},
		})
	})
}
