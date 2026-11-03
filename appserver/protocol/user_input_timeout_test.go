package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func userInputTimeoutBody(fields string) json.RawMessage {
	return json.RawMessage(`{"isBlocking":true,"itemId":"item","threadId":"thread","turnId":"turn","questions":[]` + fields + `}`)
}

func TestUserInputTimeoutAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, fields, want string
		valid              bool
	}{
		{"absent", "", "", true},
		{"null", `,"autoResolutionMs":null`, "", true},
		{"zero", `,"autoResolutionMs":0`, "0", true},
		{"positive", `,"autoResolutionMs":1`, "1", true},
		{"old maximum", `,"autoResolutionMs":9223372036854775807`, "9223372036854775807", true},
		{"negative", `,"autoResolutionMs":-1`, "", false},
		{"negative then valid", `,"autoResolutionMs":-1,"autoResolutionMs":0`, "", false},
		{"negative then null", `,"autoResolutionMs":-1,"autoResolutionMs":null`, "", false},
		{"escaped negative", `,"autoReso\u006cutionMs":-1,"autoResolutionMs":0`, "", false},
		{"differently cased extra", `,"AUTORESOLUTIONMS":-1,"autoResolutionMs":0`, "0", true},
		{"overflow", `,"autoResolutionMs":18446744073709551616`, "", false},
		{"fraction", `,"autoResolutionMs":0.5`, "", false},
		{"wrong kind", `,"autoResolutionMs":"1"`, "", false},
		{"valid duplicates", `,"autoResolutionMs":0,"autoResolutionMs":18446744073709551615`, "18446744073709551615", true},
		{"valid then null", `,"autoResolutionMs":18446744073709551615,"autoResolutionMs":null`, "", true},
		{"escaped maximum", `,"autoReso\u006cutionMs":18446744073709551615`, "18446744073709551615", true},
		{"differently cased extra alone", `,"AUTORESOLUTIONMS":18446744073709551615`, "", true},
		{"upper half", `,"autoResolutionMs":9223372036854775808`, "9223372036854775808", true},
		{"maximum", `,"autoResolutionMs":18446744073709551615`, "18446744073709551615", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := userInputTimeoutBody(tc.fields)
			var direct p.ToolRequestUserInputParams
			err := json.Unmarshal(body, &direct)
			if (err == nil) != tc.valid {
				t.Errorf("direct timeout admission valid=%v err=%v", tc.valid, err)
			}
			if tc.valid {
				encoded, marshalErr := json.Marshal(direct)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil || string(fields["autoResolutionMs"]) != tc.want {
					t.Fatalf("timeout lost during encoding: %s %v", encoded, err)
				}
				var roundtrip p.ToolRequestUserInputParams
				if err := json.Unmarshal(encoded, &roundtrip); err != nil || !reflect.DeepEqual(direct, roundtrip) {
					t.Fatalf("timeout roundtrip changed params: %v", err)
				}
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			calls := 0
			want := tc.want
			client.SetApprovalHandlers(p.ApprovalHandlers{OnToolRequestUserInput: func(_ context.Context, got p.ToolRequestUserInputParams) (p.ToolRequestUserInputResponse, error) {
				calls++
				if (want == "" && got.AutoResolutionMs != nil) || (want != "" && (got.AutoResolutionMs == nil || fmt.Sprint(*got.AutoResolutionMs) != want)) {
					t.Errorf("timeout not preserved: %v", got.AutoResolutionMs)
				}
				if got.ItemID != "item" || got.ThreadID != "thread" || got.TurnID != "turn" || !got.IsBlocking {
					t.Error("request identity changed")
				}
				return p.ToolRequestUserInputResponse{Answers: map[string]p.ToolRequestUserInputAnswer{}}, nil
			}})
			response, err := mock.InjectServerRequest(context.Background(), p.Request{JSONRPC: "2.0", ID: (p.RequestID{Value: "timeout"}), Method: "item/tool/requestUserInput", Params: body})
			if tc.valid {
				if err != nil || calls != 1 || response.Error != nil || response.ID != (p.RequestID{Value: "timeout"}) {
					t.Fatalf("valid request failed: %v calls=%d response=%+v", err, calls, response)
				}
			} else if err == nil || calls != 0 || !reflect.DeepEqual(response, p.Response{}) {
				t.Fatalf("invalid timeout reached callback: err=%v calls=%d", err, calls)
			}
			if !tc.valid {
				want = "0"
				recovered, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: 7}, Method: "item/tool/requestUserInput", Params: userInputTimeoutBody(`,"autoResolutionMs":0`)})
				if err != nil || calls != 1 || recovered.ID != (p.RequestID{Value: 7}) || recovered.Error != nil {
					t.Fatalf("same-client recovery failed: %v calls=%d response=%+v", err, calls, recovered)
				}
			}
		})
	}
}

func TestUserInputTimeoutFailureContracts(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		native           bool
	}{
		{"negative", string(userInputTimeoutBody(`,"autoResolutionMs":-1`)), "", true},
		{"overflow", string(userInputTimeoutBody(`,"autoResolutionMs":18446744073709551616`)), "", true},
		{"old native first", `{"isBlocking":"bad","autoResolutionMs":-1}`, "bool", true},
		{"old null first", `{"itemId":null,"autoResolutionMs":-1}`, "itemId", false},
		{"old required", `{"autoResolutionMs":0}`, "isBlocking", false},
		{"old identity", `{"isBlocking":true,"itemId":"","threadId":"thread","turnId":"turn","questions":[],"autoResolutionMs":18446744073709551615}`, "itemId", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := p.ToolRequestUserInputParams{AutoResolutionMs: p.Ptr(uint64(17)), ItemID: "old", Questions: []p.ToolRequestUserInputQuestion{{ID: "kept"}}}
			dst := original
			err := json.Unmarshal([]byte(tc.body), &dst)
			if err == nil || !reflect.DeepEqual(dst, original) || dst.AutoResolutionMs != original.AutoResolutionMs || &dst.Questions[0] != &original.Questions[0] || *original.AutoResolutionMs != 17 || original.Questions[0].ID != "kept" {
				t.Fatalf("failure changed receiver/references: %v", err)
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("prior error lost: %v", err)
			}
			if tc.native {
				var native *json.UnmarshalTypeError
				if !errors.As(err, &native) {
					t.Fatalf("native cause lost: %v", err)
				}
			}
		})
	}
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	response, err := mock.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "no-handler"}, Method: "item/tool/requestUserInput", Params: userInputTimeoutBody(`,"autoResolutionMs":-1`)})
	if err != nil || response.Error == nil || response.Error.Code != -32601 {
		t.Fatalf("nil handler priority changed: %+v %v", response, err)
	}
}
