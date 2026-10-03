package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestNumericDiagnosticConfigService(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	number := "1" + strings.Repeat("0", 1<<20)
	config := `{"model_context_window":` + number + `}`
	// An independent stdlib model supplies the numeric error metadata. Config's
	// existing custom owner restores the public struct name after wire decoding.
	var reference struct {
		ModelContextWindow *int64 `json:"model_context_window"`
	}
	var want *json.UnmarshalTypeError
	if !errors.As(json.Unmarshal([]byte(config), &reference), &want) {
		t.Fatal("reference did not produce a numeric overflow error")
	}
	mock.SetResponse("config/read", codex.Response{Result: json.RawMessage(`{"config":` + config + `,"origins":{}}`)})
	result, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
	var got *json.UnmarshalTypeError
	if err == nil || !errors.As(err, &got) {
		t.Fatal("overflow lost typed rejection")
	}
	if !reflect.DeepEqual(result, codex.ConfigReadResponse{}) {
		t.Fatal("overflow published a partial response")
	}
	if got.Type != reflect.TypeFor[int64]() || got.Type != want.Type || got.Offset != want.Offset || got.Field != want.Field || got.Struct != "Config" {
		t.Fatalf("numeric error metadata changed: type=%v offset=%d field=%q struct=%q", got.Type, got.Offset, got.Field, got.Struct)
	}
	if !errors.Is(err, got) {
		t.Fatal("wrapper lost its typed cause")
	}
	if len(err.Error()) > 2048 || len(got.Value) > 1024 || !strings.Contains(got.Value, "bytes omitted") {
		t.Fatalf("unbounded numeric diagnostics: error=%d bytes, retained value=%d bytes", len(err.Error()), len(got.Value))
	}
	mock.SetResponse("config/read", codex.Response{Result: json.RawMessage(`{"config":{"model_context_window":123},"origins":{}}`)})
	result, err = client.Config.Read(context.Background(), codex.ConfigReadParams{})
	if err != nil || result.Config == nil || result.Config.ModelContextWindow == nil || *result.Config.ModelContextWindow != 123 {
		t.Fatalf("valid response did not recover: %v", err)
	}
}

type numericDiagnosticCustomDecoder struct {
	Err error
}

func (v *numericDiagnosticCustomDecoder) UnmarshalJSON([]byte) error { return v.Err }

func TestNumericDiagnosticPreservesApplicationErrorOwnership(t *testing.T) {
	owned := &json.UnmarshalTypeError{
		Value: "number " + strings.Repeat("9", 1<<20),
		Type:  reflect.TypeFor[int64](), Offset: 23, Struct: "Application", Field: "value",
	}
	original := *owned
	value := numericDiagnosticCustomDecoder{Err: owned}
	receiver := codex.OptionalNullable[numericDiagnosticCustomDecoder]{Present: true, Value: &value}
	err := json.Unmarshal([]byte("1"), &receiver)
	if !errors.Is(err, owned) || !reflect.DeepEqual(*owned, original) || receiver.Value != &value || !receiver.Present {
		t.Fatal("application-owned custom error identity, metadata or receiver ownership changed")
	}
}

type numericDiagnosticPromotedDecoder struct{ numericDiagnosticCustomDecoder }
type numericDiagnosticTextDecoder struct{ Err error }

func (v *numericDiagnosticTextDecoder) UnmarshalText([]byte) error { return v.Err }

func assertNumericDiagnosticApplicationOwnership[T any](t *testing.T, body string, factory func(error) *T) {
	t.Helper()
	owned := &json.UnmarshalTypeError{Value: "number " + strings.Repeat("9", 1<<18), Type: reflect.TypeFor[int64](), Offset: 23, Struct: "Application", Field: "value"}
	referenceOwned := *owned
	reference := factory(&referenceOwned)
	if !errors.Is(json.Unmarshal([]byte(body), reference), &referenceOwned) {
		t.Fatal("reference did not delegate to the application codec")
	}
	value := factory(owned)
	receiver := codex.OptionalNullable[T]{Present: true, Value: value}
	err := json.Unmarshal([]byte(body), &receiver)
	var typed *json.UnmarshalTypeError
	if !errors.Is(err, owned) || !errors.As(err, &typed) || typed != owned || !reflect.DeepEqual(*owned, referenceOwned) || receiver.Value != value || !receiver.Present {
		t.Fatal("generic delegation changed application error identity, metadata or receiver ownership")
	}
}

func TestNumericDiagnosticGenericExtensionOwnership(t *testing.T) {
	t.Run("reused-interface", func(t *testing.T) {
		assertNumericDiagnosticApplicationOwnership(t, "1", func(err error) *any {
			var value any = &numericDiagnosticCustomDecoder{Err: err}
			return &value
		})
	})
	t.Run("nested-interface", func(t *testing.T) {
		type record struct{ Value any }
		assertNumericDiagnosticApplicationOwnership(t, `{"Value":1}`, func(err error) *record { return &record{Value: &numericDiagnosticCustomDecoder{Err: err}} })
	})
	t.Run("reused-slice-interface", func(t *testing.T) {
		assertNumericDiagnosticApplicationOwnership(t, "[1]", func(err error) *[]any {
			value := []any{&numericDiagnosticCustomDecoder{Err: err}}
			return &value
		})
	})
	t.Run("promoted-method", func(t *testing.T) {
		assertNumericDiagnosticApplicationOwnership(t, "1", func(err error) *numericDiagnosticPromotedDecoder {
			return &numericDiagnosticPromotedDecoder{numericDiagnosticCustomDecoder{Err: err}}
		})
	})
	t.Run("text-method", func(t *testing.T) {
		assertNumericDiagnosticApplicationOwnership(t, `"1"`, func(err error) *numericDiagnosticTextDecoder { return &numericDiagnosticTextDecoder{Err: err} })
	})
	t.Run("nested-generic", func(t *testing.T) {
		assertNumericDiagnosticApplicationOwnership(t, "1", func(err error) *codex.OptionalNullable[numericDiagnosticCustomDecoder] {
			return &codex.OptionalNullable[numericDiagnosticCustomDecoder]{Present: true, Value: &numericDiagnosticCustomDecoder{Err: err}}
		})
	})
}

func requireNumericDiagnostic(t *testing.T, err error, target reflect.Type) {
	t.Helper()
	var typed *json.UnmarshalTypeError
	if err == nil || !errors.As(err, &typed) || typed.Type != target || len(typed.Value) > 2048 || len(err.Error()) > 4096 || !strings.Contains(typed.Value, "bytes omitted") {
		t.Fatal("native overflow lost bounded typed rejection")
	}
}

func TestNumericDiagnosticSignedAndUnsignedCarriers(t *testing.T) {
	large := "1" + strings.Repeat("0", 1<<18)
	t.Run("signed", func(t *testing.T) {
		mock := NewMockTransport()
		client := codex.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		mock.SetResponse("command/exec", codex.Response{Result: json.RawMessage(`{"exitCode":` + large + `,"stdout":"","stderr":""}`)})
		result, err := client.Command.Exec(context.Background(), codex.CommandExecParams{Command: []string{"synthetic"}})
		requireNumericDiagnostic(t, err, reflect.TypeFor[int32]())
		if result != (codex.CommandExecResponse{}) {
			t.Fatal("rejected command response was published")
		}
		mock.SetResponse("command/exec", codex.Response{Result: json.RawMessage(`{"exitCode":0,"stdout":"ok","stderr":""}`)})
		result, err = client.Command.Exec(context.Background(), codex.CommandExecParams{Command: []string{"synthetic"}})
		if err != nil || result.Stdout != "ok" {
			t.Fatal("valid command response did not recover")
		}
	})
	t.Run("unsigned", func(t *testing.T) {
		span := codex.ByteRange{Start: 3, End: 4}
		requireNumericDiagnostic(t, json.Unmarshal([]byte(`{"start":`+large+`,"end":5}`), &span), reflect.TypeFor[uint]())
		if span != (codex.ByteRange{Start: 3, End: 4}) {
			t.Fatal("failed unsigned record replaced its receiver")
		}
		if err := json.Unmarshal([]byte(`{"start":5,"end":6}`), &span); err != nil || span.Start != 5 || span.End != 6 {
			t.Fatal("unsigned record did not recover")
		}
	})
}

func TestNumericDiagnosticNotificationAndRequests(t *testing.T) {
	large := "1" + strings.Repeat("0", 1<<18)
	mock := NewMockTransport()
	reports, notifications, requests := 0, 0, 0
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(_ string, err error) {
		reports++
		var typed *json.UnmarshalTypeError
		if !errors.As(err, &typed) || len(typed.Value) > 2048 || len(err.Error()) > 4096 {
			t.Error("handler reporter retained an oversized native error")
		}
	}))
	t.Cleanup(func() { _ = client.Close() })
	client.OnThreadTokenUsageUpdated(func(codex.ThreadTokenUsageUpdatedNotification) { notifications++ })
	client.SetApprovalHandlers(codex.ApprovalHandlers{
		OnDynamicToolCall: func(context.Context, codex.DynamicToolCallParams) (codex.DynamicToolCallResponse, error) {
			requests++
			return codex.DynamicToolCallResponse{Success: true, ContentItems: []codex.DynamicToolCallOutputContentItemWrapper{}}, nil
		},
		OnPermissionsRequestApproval: func(context.Context, codex.PermissionsRequestApprovalParams) (codex.PermissionsRequestApprovalResponse, error) {
			requests++
			return codex.PermissionsRequestApprovalResponse{}, nil
		},
	})
	usage := func(number string) json.RawMessage {
		breakdown := `{"cachedInputTokens":0,"inputTokens":` + number + `,"outputTokens":0,"reasoningOutputTokens":0,"totalTokens":0}`
		return json.RawMessage(`{"threadId":"t","turnId":"u","tokenUsage":{"last":` + breakdown + `,"total":` + breakdown + `}}`)
	}
	mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/tokenUsage/updated", Params: usage(large)})
	if reports != 1 || notifications != 0 {
		t.Fatal("invalid token usage notification was admitted")
	}
	mock.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/tokenUsage/updated", Params: usage("1")})
	if reports != 1 || notifications != 1 {
		t.Fatal("typed notification did not recover")
	}
	for _, test := range []struct {
		method, prefix, suffix string
		target                 reflect.Type
	}{
		{"item/tool/call", `{"arguments":{"n":`, `},"callId":"c","threadId":"t","tool":"tool","turnId":"u"}`, reflect.TypeFor[float64]()},
		{"item/permissions/requestApproval", `{"cwd":"/tmp","itemId":"i","permissions":{},"startedAtMs":`, `,"threadId":"t","turnId":"u"}`, reflect.TypeFor[int64]()},
	} {
		t.Run(test.method, func(t *testing.T) {
			before := requests
			response, err := mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: "r"}, Method: test.method, Params: json.RawMessage(test.prefix + large + test.suffix)})
			requireNumericDiagnostic(t, err, test.target)
			if !errors.Is(err, codex.ErrInvalidParams) || requests != before || response.Result != nil {
				t.Fatal("rejected request lost classification or published handler output")
			}
			_, err = mock.InjectServerRequest(context.Background(), codex.Request{ID: codex.RequestID{Value: "r"}, Method: test.method, Params: json.RawMessage(test.prefix + "1" + test.suffix)})
			if err != nil || requests != before+1 {
				t.Fatal("valid approval request did not recover")
			}
		})
	}
	if reports != 3 {
		t.Fatal("invalid request diagnostic was not reported once per rejection")
	}
}

func BenchmarkNumericDiagnosticConfigRead(b *testing.B) {
	for _, size := range []int{32, 1 << 20} {
		name := "small"
		if size > 32 {
			name = "1MiB"
		}
		b.Run(name, func(b *testing.B) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			b.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("config/read", codex.Response{Result: json.RawMessage(`{"config":{"model_context_window":1` + strings.Repeat("0", size) + `},"origins":{}}`)})
			_, _ = client.Config.Read(context.Background(), codex.ConfigReadParams{})
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := client.Config.Read(context.Background(), codex.ConfigReadParams{}); err == nil {
					b.Fatal("overflow was admitted")
				}
			}
		})
	}
}

func TestNumericDiagnosticPreservesValidLongNumber(t *testing.T) {
	input := []byte("1" + strings.Repeat("0", 1<<20))
	var value codex.OptionalNullable[json.Number]
	if err := json.Unmarshal(input, &value); err != nil || !value.Present || value.Value == nil || string(*value.Value) != string(input) {
		t.Fatal("accepted native Number data was truncated or rejected")
	}
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) != string(input) {
		t.Fatal("accepted long Number lost its serialization")
	}
}
