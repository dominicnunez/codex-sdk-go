package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type numberLiteralApplicationCodec struct{ Err error }

func (v *numberLiteralApplicationCodec) UnmarshalJSON([]byte) error { return v.Err }

func TestMalformedNumberApplicationDecoderIdentity(t *testing.T) {
	shared := errors.New("json: invalid number literal, trying to unmarshal " + strings.Repeat("x", 1<<20) + " into Number")
	codec := numberLiteralApplicationCodec{Err: shared}
	value := protocol.OptionalNullable[numberLiteralApplicationCodec]{Present: true, Value: &codec}
	err := json.Unmarshal([]byte(`"1"`), &value)
	if !errors.Is(err, shared) || err != shared || value.Value != &codec {
		t.Fatal("application codec error or target identity changed")
	}
}

type numberLiteralApplicationTextDecoder struct{ Err error }

func (v *numberLiteralApplicationTextDecoder) UnmarshalText([]byte) error { return v.Err }

func TestMalformedNumberApplicationTextDecoderIdentity(t *testing.T) {
	var native json.Number
	input, err := json.Marshal(strings.Repeat("x", 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	shared := json.Unmarshal(input, &native)
	codec := numberLiteralApplicationTextDecoder{Err: shared}
	value := protocol.OptionalNullable[numberLiteralApplicationTextDecoder]{Present: true, Value: &codec}
	if err := json.Unmarshal(input, &value); err != shared || !errors.Is(err, shared) || value.Value != &codec {
		t.Fatal("application Text decoder ownership changed")
	}
}

func TestMalformedNumberReceiverCompatibility(t *testing.T) {
	quoted, err := json.Marshal(strings.Repeat("x", 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"good":9,"bad":` + string(quoted) + `,"after":7}`,
		`{"same":9,"same":` + string(quoted) + `}`,
		`{"reset":null,"bad":` + string(quoted) + `}`,
	} {
		want := map[string]json.Number{"seed": "5", "reset": "4"}
		got := map[string]json.Number{"seed": "5", "reset": "4"}
		wantErr := json.Unmarshal([]byte(input), &want)
		value := protocol.OptionalNullable[map[string]json.Number]{Present: true, Value: &got}
		gotErr := json.Unmarshal([]byte(input), &value)
		if wantErr == nil || gotErr == nil || len(gotErr.Error()) > 4096 || !reflect.DeepEqual(got, want) || value.Value != &got || !value.Present {
			t.Fatal("partial receiver, duplicate or null behavior changed")
		}
	}
	var want json.Number
	wantErr := json.Unmarshal([]byte(`"01"`), &want)
	var got protocol.OptionalNullable[json.Number]
	gotErr := json.Unmarshal([]byte(`"01"`), &got)
	if wantErr == nil || gotErr == nil || wantErr.Error() != gotErr.Error() || got.Present || got.Value != nil {
		t.Fatal("short error or failed absent receiver changed")
	}
}

func TestMalformedNumberOptionalNullableDiagnostic(t *testing.T) {
	invalid := strings.Repeat("x", 1<<20)
	input, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	seed := json.Number("7")
	value := protocol.OptionalNullable[json.Number]{Present: true, Value: &seed}
	err = json.Unmarshal(input, &value)
	if err == nil {
		t.Fatal("invalid numeric literal admitted")
	}
	if len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") {
		t.Fatalf("native number diagnostic not bounded: bytes=%d", len(err.Error()))
	}
	if value.Value != &seed || seed != "7" || !value.Present {
		t.Fatal("diagnostic policy changed the failed receiver")
	}
	for current := err; current != nil; {
		if len(current.Error()) > 4096 {
			t.Fatal("bounded display retains a full original diagnostic in its chain")
		}
		wrapped, ok := current.(interface{ Unwrap() error })
		if !ok {
			break
		}
		current = wrapped.Unwrap()
	}
	if err := json.Unmarshal([]byte(`"123"`), &value); err != nil || *value.Value != "123" {
		t.Fatalf("valid number failed after rejection: %v", err)
	}
}

func TestMalformedNumberApprovalResultDiagnostic(t *testing.T) {
	mock := NewMockTransport()
	var reported []int
	client := protocol.NewClient(mock, protocol.WithHandlerErrorCallback(func(_ string, err error) {
		reported = append(reported, len(err.Error()))
	}))
	defer client.Close()
	number := json.Number(strings.Repeat("x", 1<<20))
	calls := 0
	client.SetApprovalHandlers(protocol.ApprovalHandlers{
		OnMcpServerElicitationRequest: func(context.Context, protocol.McpServerElicitationRequestParams) (protocol.McpServerElicitationRequestResponse, error) {
			calls++
			return protocol.McpServerElicitationRequestResponse{Action: protocol.McpServerElicitationActionAccept, Content: map[string]interface{}{"n": number}}, nil
		},
	})
	request := protocol.Request{ID: protocol.RequestID{Value: "r"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"serverName":"s","threadId":"t","mode":"url","message":"m","elicitationId":"e","url":"https://example.test"}`)}
	response, err := mock.InjectServerRequest(context.Background(), request)
	if err == nil || len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") {
		length := 0
		if err != nil {
			length = len(err.Error())
		}
		t.Fatalf("result diagnostic not bounded: bytes=%d", length)
	}
	if response.Result != nil || calls != 1 || len(reported) != 1 || reported[0] > 4096 {
		t.Fatal("rejected result was published or oversized reporter data retained")
	}
	number = json.Number(strings.Repeat("9", 1<<16))
	response, err = mock.InjectServerRequest(context.Background(), request)
	if err != nil || calls != 2 || len(reported) != 1 {
		t.Fatalf("valid long numeric data did not recover: %v", err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	var content map[string]json.RawMessage
	if err := json.Unmarshal(result["content"], &content); err != nil {
		t.Fatal(err)
	}
	if string(content["n"]) != string(number) {
		t.Fatal("valid long numeric token was altered")
	}
}

func TestMalformedNumberRequestDiagnostic(t *testing.T) {
	mock := NewMockTransport()
	client := protocol.NewClient(mock)
	defer client.Close()
	params := protocol.TurnStartParams{ThreadID: "t", Input: []protocol.UserInput{}, OutputSchema: map[string]any{"n": json.Number(strings.Repeat("x", 1<<20))}}
	_, err := client.Turn.Start(context.Background(), params)
	if err == nil || len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") {
		t.Fatal("request diagnostic must be bounded")
	}
	if mock.GetSentRequest(0) != nil {
		t.Fatal("invalid request reached transport")
	}
	mock.SetResponse("turn/start", protocol.Response{Result: json.RawMessage(`{"turn":{"id":"u","items":[],"status":"completed","error":null}}`)})
	valid := json.Number(strings.Repeat("9", 1<<16))
	params.OutputSchema = map[string]any{"n": valid}
	if _, err := client.Turn.Start(context.Background(), params); err != nil {
		t.Fatalf("request did not recover: %v", err)
	}
	request := mock.GetSentRequest(0)
	if request == nil || !strings.Contains(string(request.Params), string(valid)) {
		t.Fatal("valid long numeric data did not reach transport intact")
	}
}

func TestMalformedNumberNestedSDKMarshalers(t *testing.T) {
	number := json.Number(strings.Repeat("x", 1<<20))
	object := map[string]any{"n": number}
	optional := protocol.OptionalNullable[map[string]any]{Present: true, Value: &object}
	cases := map[string]any{
		"optional":          optional,
		"optional envelope": struct{ Value any }{optional},
		"turn":              protocol.TurnStartParams{OutputSchema: object},
		"MCP arguments":     &protocol.McpToolCallThreadItem{Arguments: object},
		"MCP result":        &protocol.McpToolCallThreadItem{Result: &protocol.McpToolCallResult{StructuredContent: object}},
		"dynamic arguments": &protocol.DynamicToolCallThreadItem{Arguments: object},
		"thread union":      protocol.ThreadItemWrapper{Value: &protocol.DynamicToolCallThreadItem{Arguments: object}},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := json.Marshal(value)
			if err == nil || len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") {
				t.Fatal("nested native diagnostic must be bounded")
			}
			for current := err; current != nil; current = errors.Unwrap(current) {
				if len(current.Error()) > 4096 {
					t.Fatal("error chain retains original diagnostic")
				}
			}
		})
	}
}

type numberLiteralApplicationMarshaler struct{ Err error }

func (v numberLiteralApplicationMarshaler) MarshalJSON() ([]byte, error) { return nil, v.Err }

type numberLiteralApplicationTextMarshaler struct{ Err error }

func (v numberLiteralApplicationTextMarshaler) MarshalText() ([]byte, error) { return nil, v.Err }

func TestMalformedNumberApplicationEncoderIdentity(t *testing.T) {
	// Also return an actual native error from application code; a native-looking
	// message is insufficient to transfer ownership to the SDK.
	_, native := json.Marshal(json.Number(strings.Repeat("x", 1<<20)))
	for name, codec := range map[string]any{
		"JSON": numberLiteralApplicationMarshaler{native},
		"Text": numberLiteralApplicationTextMarshaler{native},
	} {
		t.Run(name, func(t *testing.T) {
			value := protocol.OptionalNullable[any]{Present: true, Value: &codec}
			_, err := json.Marshal(value)
			var marshaler *json.MarshalerError
			if !errors.Is(err, native) || !errors.As(err, &marshaler) {
				t.Fatal("application error chain changed")
			}
			if len(native.Error()) < 1<<20 {
				t.Fatal("shared application error was mutated")
			}
		})
	}
}

func BenchmarkMalformedNumberPublicOperations(b *testing.B) {
	for _, size := range []int{1 << 10, 1 << 20} {
		for _, escaped := range []bool{false, true} {
			name := fmt.Sprintf("bytes=%d/escaped=%t", size, escaped)
			literal := strings.Repeat("x", size)
			if escaped {
				literal = strings.Repeat("\x00", size)
			}
			number := json.Number(literal)
			b.Run("request/"+name, func(b *testing.B) {
				client := protocol.NewClient(NewMockTransport())
				defer client.Close()
				params := protocol.TurnStartParams{ThreadID: "t", Input: []protocol.UserInput{}, OutputSchema: map[string]any{"n": number}}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_, err := client.Turn.Start(context.Background(), params)
					if err == nil {
						b.Fatal("invalid request admitted")
					}
					_ = err.Error()
				}
			})
			b.Run("approval/"+name, func(b *testing.B) {
				mock := NewMockTransport()
				client := protocol.NewClient(mock, protocol.WithHandlerErrorCallback(func(_ string, err error) { _ = err.Error() }))
				defer client.Close()
				client.SetApprovalHandlers(protocol.ApprovalHandlers{OnMcpServerElicitationRequest: func(context.Context, protocol.McpServerElicitationRequestParams) (protocol.McpServerElicitationRequestResponse, error) {
					return protocol.McpServerElicitationRequestResponse{Action: protocol.McpServerElicitationActionAccept, Content: map[string]any{"n": number}}, nil
				}})
				request := protocol.Request{ID: protocol.RequestID{Value: "r"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"serverName":"s","threadId":"t","mode":"url","message":"m","elicitationId":"e","url":"https://example.test"}`)}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					_, err := mock.InjectServerRequest(context.Background(), request)
					if err == nil {
						b.Fatal("invalid result admitted")
					}
					_ = err.Error()
				}
			})
		}
	}
}
