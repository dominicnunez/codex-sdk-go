package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAccountGetRateLimitsRejectsNullBucket(t *testing.T) {
	for _, withParams := range []bool{false, true} {
		name := "GetRateLimits"
		if withParams {
			name = "GetRateLimitsWithParams"
		}
		t.Run(name, func(t *testing.T) {
			transport := NewMockTransport()
			transport.SetResponse("account/rateLimits/read", codex.Response{
				JSONRPC: "2.0",
				Result:  json.RawMessage(`{"rateLimits":{},"rateLimitsByLimitId":{"bad":null}}`),
			})
			client := codex.NewClient(transport)
			var got codex.GetAccountRateLimitsResponse
			var err error
			if withParams {
				got, err = client.Account.GetRateLimitsWithParams(context.Background(), &codex.GetAccountRateLimitsParams{})
			} else {
				got, err = client.Account.GetRateLimits(context.Background())
			}
			if err == nil {
				t.Fatal("GetRateLimits returned a successful response containing a null bucket")
			}
			if got.RateLimitsByLimitId != nil || got.RateLimits != (codex.RateLimitSnapshot{}) {
				t.Fatalf("response on error = %#v; want zero response", got)
			}
		})
	}
}

func TestAccountRateLimitsBucketMapShapeAndEncoding(t *testing.T) {
	tests := []struct {
		name string
		json string
		ok   bool
	}{
		{name: "absent", json: `{"rateLimits":{}}`, ok: true},
		{name: "null outer map", json: `{"rateLimits":{},"rateLimitsByLimitId":null}`, ok: true},
		{name: "empty outer map", json: `{"rateLimits":{},"rateLimitsByLimitId":{}}`, ok: true},
		{name: "empty bucket", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":{}}}`, ok: true},
		{name: "null bucket", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":null}}`},
		{name: "bad then valid duplicate map", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":null},"rateLimitsByLimitId":{"a":{}}}`},
		{name: "null reset after bad map", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":null},"rateLimitsByLimitId":null}`},
		{name: "folded escaped duplicate", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":null},"RATE\u004cIMITSBYLIMITID":{"a":{}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.GetAccountRateLimitsResponse
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.ok && err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("Unmarshal() accepted invalid null bucket")
			}
			if !tt.ok {
				return
			}
			if tt.name == "empty outer map" {
				encoded, err := json.Marshal(got)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				var members map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &members); err != nil {
					t.Fatalf("Unmarshal(encoded) error = %v", err)
				}
				if string(members["rateLimitsByLimitId"]) != `{}` {
					t.Fatalf("encoded member = %s; want present empty object", members["rateLimitsByLimitId"])
				}
			}
		})
	}
}

func TestAccountRateLimitsDuplicateMapMergesAndNullReset(t *testing.T) {
	tests := []struct {
		name string
		json string
		want map[string]*codex.RateLimitSnapshot
	}{
		{
			name: "valid duplicate maps merge",
			json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":{"limitId":"a"}},"rateLimitsByLimitId":{"b":{"limitId":"b"}}}`,
			want: map[string]*codex.RateLimitSnapshot{"a": {LimitId: ptr("a")}, "b": {LimitId: ptr("b")}},
		},
		{
			name: "null resets before valid map",
			json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":{}},"rateLimitsByLimitId":null,"rateLimitsByLimitId":{"b":{}}}`,
			want: map[string]*codex.RateLimitSnapshot{"b": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.GetAccountRateLimitsResponse
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if len(got.RateLimitsByLimitId) != len(tt.want) {
				t.Fatalf("bucket keys = %#v; want %#v", got.RateLimitsByLimitId, tt.want)
			}
			for key, want := range tt.want {
				if got.RateLimitsByLimitId[key] == nil || got.RateLimitsByLimitId[key].LimitId == nil && want.LimitId != nil {
					t.Fatalf("bucket %q = %#v; want %#v", key, got.RateLimitsByLimitId[key], want)
				}
				if want.LimitId != nil && *got.RateLimitsByLimitId[key].LimitId != *want.LimitId {
					t.Fatalf("bucket %q limitId = %q; want %q", key, *got.RateLimitsByLimitId[key].LimitId, *want.LimitId)
				}
			}
		})
	}
}

func TestAccountRateLimitsEmptyMapEncodingKeepsEnvelopeFields(t *testing.T) {
	type envelope struct {
		codex.GetAccountRateLimitsResponse
		Marker string `json:"marker"`
	}
	got, err := json.Marshal(envelope{
		GetAccountRateLimitsResponse: codex.GetAccountRateLimitsResponse{RateLimitsByLimitId: map[string]*codex.RateLimitSnapshot{}},
		Marker:                       "kept",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got, &fields); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if string(fields["rateLimitsByLimitId"]) != `{}` || string(fields["marker"]) != `"kept"` {
		t.Fatalf("encoded envelope = %s; want empty map and marker", got)
	}
}

func TestAccountRateLimitsNativeTypeErrorPrecedesNullBucketValidation(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "native type", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":null,"b":4}}`, want: "cannot unmarshal number"},
		{name: "enum", json: `{"rateLimits":{},"rateLimitsByLimitId":{"a":null,"b":{"planType":"bogus"}}}`, want: `invalid rateLimits.planType "bogus"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got codex.GetAccountRateLimitsResponse
			err := json.Unmarshal([]byte(tt.json), &got)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Unmarshal() error = %v; want existing decoder error %q", err, tt.want)
			}
		})
	}
}

func TestAccountRateLimitsDirectDecoderErrorLeavesReusedReceiver(t *testing.T) {
	prior := codex.RateLimitSnapshot{LimitId: ptr("prior")}
	got := codex.GetAccountRateLimitsResponse{
		RateLimits:          prior,
		RateLimitsByLimitId: map[string]*codex.RateLimitSnapshot{"prior": &prior},
	}
	err := json.Unmarshal([]byte(`{"rateLimits":{},"rateLimitsByLimitId":{"bad":null}}`), &got)
	if err == nil {
		t.Fatal("Unmarshal() accepted null bucket")
	}
	if got.RateLimits.LimitId == nil || *got.RateLimits.LimitId != "prior" || got.RateLimitsByLimitId["prior"] != &prior || len(got.RateLimitsByLimitId) != 1 {
		t.Fatalf("receiver changed on error: %#v", got)
	}
}

func TestAccountRateLimitsNamedWrapperPreservesDecoderBoundary(t *testing.T) {
	type wrapper struct {
		Before   int                                `json:"before"`
		Response codex.GetAccountRateLimitsResponse `json:"response"`
		After    int                                `json:"after"`
	}
	got := wrapper{After: 13}
	err := json.Unmarshal([]byte(`{"before":"wrong","response":{"rateLimits":{},"rateLimitsByLimitId":{"bad":null}},"after":9}`), &got)
	if err == nil || !strings.Contains(err.Error(), "rateLimitsByLimitId") || strings.Contains(err.Error(), "cannot unmarshal string") {
		t.Fatalf("Unmarshal() error = %v; want wrapped account bucket validation error", err)
	}
	if got.After != 13 {
		t.Fatalf("later sibling decoded after response error: got %d, want preserved sentinel 13", got.After)
	}
}

func TestAccountRateLimitsMalformedDirectCallKeepsExistingEOFError(t *testing.T) {
	input := []byte(`{"rateLimits":{},"rateLimitsByLimitId":{"bad":null},`)
	var got codex.GetAccountRateLimitsResponse
	err := got.UnmarshalJSON(input)
	if err == nil || err.Error() != "server returned non-object result: EOF" {
		t.Fatalf("direct UnmarshalJSON() error = %v; want the existing EOF response-shape error", err)
	}
}

func TestAccountRateLimitsNullBucketDiagnosticBoundsLongKey(t *testing.T) {
	key := strings.Repeat("control\n\t\\\"/", 1000)
	encodedKey, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("Marshal(key) error = %v", err)
	}
	input := fmt.Sprintf(`{"rateLimits":{},"rateLimitsByLimitId":{%s:null}}`, encodedKey)
	transport := NewMockTransport()
	transport.SetResponse("account/rateLimits/read", codex.Response{JSONRPC: "2.0", Result: json.RawMessage(input)})
	client := codex.NewClient(transport)
	_, err = client.Account.GetRateLimits(context.Background())
	if err == nil {
		t.Fatal("GetRateLimits() accepted null bucket")
	}
	if len(err.Error()) > 1200 || !strings.Contains(err.Error(), "bytes omitted") || !strings.Contains(err.Error(), "account/rateLimits/read") {
		t.Fatalf("public diagnostic length = %d; expected bounded method-wrapped diagnostic: %v", len(err.Error()), err)
	}
}
