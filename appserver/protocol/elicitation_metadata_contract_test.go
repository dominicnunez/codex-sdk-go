package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

var elicitationMetadataModes = []p.McpServerElicitationMode{
	p.McpServerElicitationModeForm, p.McpServerElicitationModeURL,
	p.McpServerElicitationModeOpenAIForm, p.McpServerElicitationModeOpenAIFormLegacy,
}

func TestElicitationMetadataSelection(t *testing.T) {
	for _, mode := range elicitationMetadataModes {
		for _, tc := range []struct{ members, want string }{
			{"", "null"},
			{`,"_meta":null`, "null"},
			{`,"_META":42`, "null"},
			{`,"_m\u0065ta":9007199254740993`, "9007199254740993"},
			{`,"_meta":{"old":1},"_meta":{"new":2}`, `{"new":2}`},
			{`,"_meta":[1],"_meta":null`, "null"},
			{`,"_meta":null,"_meta":[-0,1e400]`, "[-0,1e400]"},
			{`,"_meta":{"n":1,"n":2}`, `{"n":2}`},
		} {
			t.Run(string(mode)+tc.members, func(t *testing.T) {
				var params p.McpServerElicitationRequestParams
				seed := elicitationModeBody(fmt.Sprintf(`"mode":%q,"_meta":{"old":[1]}`, mode))
				if err := json.Unmarshal([]byte(seed), &params); err != nil {
					t.Fatal(err)
				}
				body := []byte(elicitationModeBody(fmt.Sprintf(`"mode":%q`, mode) + tc.members))
				if err := json.Unmarshal(body, &params); err != nil {
					t.Fatal(err)
				}
				for i := range body {
					body[i] = ' '
				}
				got, err := json.Marshal(params.Meta)
				if err != nil || string(got) != tc.want {
					t.Fatalf("selection or ownership changed: %s / %s: %v", got, tc.want, err)
				}
			})
		}
	}
}

func TestElicitationMetadataAdmission(t *testing.T) {
	for _, mode := range elicitationMetadataModes {
		t.Run(string(mode), func(t *testing.T) {
			seed := elicitationModeBody(fmt.Sprintf(`"mode":%q,"_meta":{"n":[9007199254740993]}`, mode))
			var params p.McpServerElicitationRequestParams
			if err := json.Unmarshal([]byte(seed), &params); err != nil {
				t.Fatal(err)
			}
			before := params
			bad := elicitationModeBody(fmt.Sprintf(`"mode":"future","mode":%q,"_meta":{"replacement":1e400}`, mode))
			if err := json.Unmarshal([]byte(bad), &params); err == nil {
				t.Fatal("invalid mode accepted")
			}
			if !reflect.DeepEqual(params, before) || reflect.ValueOf(params.Meta).Pointer() != reflect.ValueOf(before.Meta).Pointer() {
				t.Fatal("failed admission changed retained metadata")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			defer client.Close()
			calls := 0
			client.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, got p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
				calls++
				if !reflect.DeepEqual(got.Meta, before.Meta) {
					t.Fatal("recovery changed exact metadata")
				}
				return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel, Meta: got.Meta}, nil
			}})
			request := p.Request{ID: p.RequestID{Value: "r"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(bad)}
			resp, err := mock.InjectServerRequest(context.Background(), request)
			if !errors.Is(err, p.ErrInvalidParams) || calls != 0 || !reflect.DeepEqual(resp, p.Response{}) {
				t.Fatalf("invalid request published: %+v %v calls=%d", resp, err, calls)
			}
			request.Params = json.RawMessage(seed)
			resp, err = mock.InjectServerRequest(context.Background(), request)
			if err != nil || calls != 1 || resp.Error != nil {
				t.Fatalf("recovery failed: %v", err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(resp.Result, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["_meta"]) != `{"n":[9007199254740993]}` {
				t.Fatal("reply changed exact metadata")
			}
		})
	}
}

func FuzzElicitationMetadata(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(int64(9007199254740993), int16(400), "metadata", mode, uint8(0))
	}
	f.Fuzz(func(t *testing.T, n int64, exp int16, text string, mode, kind uint8) {
		if len(text) > 4096 {
			return
		}
		// Construct the expected graph independently, with every JSON kind and
		// numbers that ordinary float64 decoding cannot preserve.
		text = strings.ToValidUTF8(text, "\ufffd")
		integer := json.Number(strconv.FormatInt(n, 10))
		exponent := json.Number("-1.000000000000000001e" + strconv.Itoa(int(exp)))
		var want any
		switch kind % 6 {
		case 0:
			want = map[string]any{"integer": integer, "nested": []any{exponent, text, nil, true}}
		case 1:
			want = integer
		case 2:
			want = []any{integer, exponent}
		case 3:
			want = text
		case 4:
			want = false
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		body := []byte(elicitationModeBody(fmt.Sprintf(`"mode":%q,"_meta":%s`, elicitationMetadataModes[mode%4], raw)))
		var params p.McpServerElicitationRequestParams
		if err := json.Unmarshal(body, &params); err != nil {
			t.Fatal(err)
		}
		for i := range body {
			body[i] = ' '
		}
		if !reflect.DeepEqual(params.Meta, want) {
			t.Fatalf("metadata changed: %#v / %#v", params.Meta, want)
		}
		encoded, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		var round p.McpServerElicitationRequestParams
		if err := json.Unmarshal(encoded, &round); err != nil || !reflect.DeepEqual(params, round) {
			t.Fatalf("roundtrip changed request: %v", err)
		}
	})
}
