package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestValueDiagnosticModelService(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	for _, value := range []string{strings.Repeat("x", 8<<20), strings.Repeat("\x00", 1<<18), strings.Repeat("日本語", 1<<18)} {
		quoted, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		body := `{"data":[` + strings.TrimSuffix(optionalArrayModel, "}") + `,"inputModalities":[` + string(quoted) + `]}]}`
		mock.SetResponse("model/list", codex.Response{Result: json.RawMessage(body)})
		_, err = client.Model.List(context.Background(), codex.ModelListParams{})
		if err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "inputModality") || !strings.Contains(err.Error(), "bytes omitted") {
			if err == nil {
				t.Fatal("invalid modality admitted")
			}
			t.Fatalf("unbounded or contextless model diagnostic: %d bytes", len(err.Error()))
		}
	}
	mock.SetResponse("model/list", codex.Response{Result: json.RawMessage(`{"data":[` + strings.TrimSuffix(optionalArrayModel, "}") + `,"inputModalities":["text"]}]}`)})
	if _, err := client.Model.List(context.Background(), codex.ModelListParams{}); err != nil {
		t.Fatalf("valid model response after rejection: %v", err)
	}
}

func TestValueDiagnosticNestedServices(t *testing.T) {
	for _, plugin := range []bool{false, true} {
		method := "config/read"
		if plugin {
			method = "plugin/read"
		}
		t.Run(method, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			for _, large := range []string{strings.Repeat("x", 1<<20), strings.Repeat("\x00", 1<<18)} {
				var payload any
				if plugin {
					detail := issue74PluginDetail()
					detail["scheduledTasks"] = []any{map[string]any{"key": "k", "name": "N", "prompt": "p", "schedule": map[string]any{"type": large}}}
					payload = map[string]any{"plugin": detail}
				} else {
					payload = map[string]any{"config": map[string]any{"apps": map[string]any{large: nil}}, "origins": map[string]any{}}
				}
				if err := mock.SetResponseData(method, payload); err != nil {
					t.Fatal(err)
				}
				var err error
				if plugin {
					_, err = client.Plugin.Read(context.Background(), codex.PluginReadParams{PluginName: "calendar", MarketplacePath: "/tmp/plugins"})
				} else {
					_, err = client.Config.Read(context.Background(), codex.ConfigReadParams{})
				}
				if err == nil {
					t.Fatal("invalid nested value admitted")
				}
				if len(err.Error()) > 2048 || !strings.Contains(err.Error(), "bytes omitted") {
					t.Fatalf("unbounded nested diagnostic: %d bytes", len(err.Error()))
				}
			}
		})
	}
}

func TestValueDiagnosticNotificationAndRequest(t *testing.T) {
	mock := NewMockTransport()
	reports, calls := 0, 0
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		reports++
		if (method != "command/exec/outputDelta" && method != "mcpServer/elicitation/request") || err == nil || len(err.Error()) > 2048 {
			t.Error("unbounded notification diagnostic or lost origin")
		}
	}))
	t.Cleanup(func() { _ = client.Close() })
	client.OnCommandExecOutputDelta(func(codex.CommandExecOutputDeltaNotification) { calls++ })
	for _, value := range []string{strings.Repeat("x", 1<<20), "stdout"} {
		body, err := json.Marshal(map[string]any{"capReached": false, "deltaBase64": "", "processId": "p", "stream": value})
		if err != nil {
			t.Fatal(err)
		}
		mock.InjectServerNotification(context.Background(), codex.Notification{Method: "command/exec/outputDelta", Params: body})
	}
	if reports != 1 || calls != 1 {
		t.Fatalf("notification admission/recovery: reports=%d calls=%d", reports, calls)
	}
	requests := 0
	client.SetApprovalHandlers(codex.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, codex.McpServerElicitationRequestParams) (codex.McpServerElicitationRequestResponse, error) {
		requests++
		return codex.McpServerElicitationRequestResponse{Action: codex.McpServerElicitationActionDecline}, nil
	}})
	for _, value := range []string{strings.Repeat("x", 1<<20), "url"} {
		body, err := json.Marshal(map[string]any{"serverName": "s", "threadId": "t", "mode": value, "message": "m", "elicitationId": "e", "url": "https://example.test"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: "r"}, Method: "mcpServer/elicitation/request", Params: body})
		if value != "url" {
			if err == nil || !errors.Is(err, codex.ErrInvalidParams) || len(err.Error()) > 2048 || requests != 0 {
				t.Error("unbounded request diagnostic, lost classification or invalid handler admission")
			}
		} else if err != nil || requests != 1 {
			t.Errorf("valid request recovery: %v, calls=%d", err, requests)
		}
	}
	if reports != 2 {
		t.Fatalf("diagnostic callback reports=%d, want both rejected owners", reports)
	}
}

func TestValueDiagnosticOutbound(t *testing.T) {
	large := strings.Repeat("x", 1<<20)
	for _, value := range []any{codex.AutoCompactTokenLimitScope(large), codex.RequestID{Value: json.Number(strings.Repeat("9", 1<<20))}} {
		data, err := json.Marshal(value)
		var typed *json.MarshalerError
		if err == nil || !errors.As(err, &typed) || len(err.Error()) > 1800 || len(data) != 0 {
			t.Error("unbounded outbound diagnostic, lost marshaler context or invalid publication")
		}
	}
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	scope := codex.PermissionGrantScope(large)
	client.SetApprovalHandlers(codex.ApprovalHandlers{OnPermissionsRequestApproval: func(context.Context, codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
		return codex.PermissionsRequestApprovalResponse{Scope: &scope}, nil
	}})
	request := codex.Request{ID: codex.RequestID{Value: "r"}, Method: "item/permissions/requestApproval", Params: json.RawMessage(`{"cwd":"/tmp","itemId":"i","permissions":{},"startedAtMs":1,"threadId":"t","turnId":"u"}`)}
	response, err := mock.InjectServerRequest(context.Background(), request)
	if err == nil || len(err.Error()) > 1800 || len(response.Result) != 0 {
		t.Error("invalid outbound scope returned a success payload or unbounded error")
	}
	scope = codex.PermissionGrantScopeTurn
	response, err = mock.InjectServerRequest(context.Background(), request)
	if err != nil || len(response.Result) == 0 {
		t.Fatalf("valid approval response after rejection: %v", err)
	}
}

func BenchmarkValueDiagnosticModel(b *testing.B) {
	for _, size := range []int{32, 8 << 20} {
		name := "small"
		if size > 32 {
			name = "8MiB"
		}
		b.Run(name, func(b *testing.B) {
			mock := NewMockTransport()
			body := `{"data":[` + strings.TrimSuffix(optionalArrayModel, "}") + `,"inputModalities":["` + strings.Repeat("x", size) + `"]}]}`
			mock.SetResponse("model/list", codex.Response{Result: json.RawMessage(body)})
			client := codex.NewClient(mock)
			b.Cleanup(func() { _ = client.Close() })
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := client.Model.List(context.Background(), codex.ModelListParams{}); err == nil {
					b.Fatal("invalid modality admitted")
				}
				mock.mu.Lock()
				mock.SentRequests = mock.SentRequests[:0]
				mock.mu.Unlock()
			}
		})
	}
}
