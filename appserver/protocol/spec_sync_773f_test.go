package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRateLimitsClientCapabilities(t *testing.T) {
	for _, tt := range []struct {
		name   string
		params *codex.GetAccountRateLimitsParams
		want   string
	}{
		{"nil", nil, ""},
		{"omitted", &codex.GetAccountRateLimitsParams{}, "{}"},
		{"false", &codex.GetAccountRateLimitsParams{ExcludeResetCreditDetails: ptr(false), SupportsLunaReserve: ptr(false)}, `{"excludeResetCreditDetails":false,"supportsLunaReserve":false}`},
		{"true", &codex.GetAccountRateLimitsParams{ExcludeResetCreditDetails: ptr(true), SupportsLunaReserve: ptr(true)}, `{"excludeResetCreditDetails":true,"supportsLunaReserve":true}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewMockTransport()
			client := codex.NewClient(transport)
			defer client.Close()
			if err := transport.SetResponseData("account/rateLimits/read", map[string]interface{}{"rateLimits": map[string]interface{}{}}); err != nil {
				t.Fatal(err)
			}
			if _, err := client.Account.GetRateLimitsWithParams(context.Background(), tt.params); err != nil {
				t.Fatal(err)
			}
			req := transport.GetSentRequest(0)
			if req.Method != "account/rateLimits/read" || string(req.Params) != tt.want {
				t.Fatalf("request = %s %s; want %s", req.Method, req.Params, tt.want)
			}
			if _, err := client.Account.GetRateLimits(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := string(transport.GetSentRequest(1).Params); got != "" {
				t.Fatalf("legacy params = %s", got)
			}
		})
	}
}

func TestRateLimitsOrdinaryUsagePermission(t *testing.T) {
	for _, value := range []string{"null", "false", "true"} {
		t.Run(value, func(t *testing.T) {
			var resp codex.GetAccountRateLimitsResponse
			data := `{"ordinaryUsageAllowed":` + value + `,"rateLimits":{"normalModelSlug":"model-a"},"rateLimitsByLimitId":{"alias":{"normalModelSlug":"model-b"}}}`
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				t.Fatal(err)
			}
			if value == "null" {
				if resp.OrdinaryUsageAllowed != nil {
					t.Fatal("unavailable permission became a decision")
				}
			} else if resp.OrdinaryUsageAllowed == nil || *resp.OrdinaryUsageAllowed != (value == "true") {
				t.Fatal("permission was lost")
			}
			if resp.RateLimits.NormalModelSlug == nil || *resp.RateLimits.NormalModelSlug != "model-a" {
				t.Fatal("normal model slug was lost")
			}
			if got := resp.RateLimitsByLimitId["alias"].NormalModelSlug; got == nil || *got != "model-b" {
				t.Fatal("alias model slug was lost")
			}
			if err := json.Unmarshal([]byte(`{"rateLimits":{}}`), &resp); err != nil {
				t.Fatal(err)
			}
			if resp.OrdinaryUsageAllowed != nil {
				t.Fatal("omitted permission retained a stale decision")
			}
		})
	}
}

func TestThreadOriginatorRoundTrip(t *testing.T) {
	fixture := validThreadPayload("thread-origin")
	fixture["originator"] = "desktop"
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var thread codex.Thread
	if err := json.Unmarshal(data, &thread); err != nil {
		t.Fatal(err)
	}
	if thread.Originator == nil || *thread.Originator != "desktop" {
		t.Fatal("originator was lost")
	}
	data, err = json.Marshal(thread)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip map[string]interface{}
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip["originator"] != "desktop" {
		t.Fatal("originator was not serialized")
	}
	delete(fixture, "originator")
	data, _ = json.Marshal(fixture)
	if err := json.Unmarshal(data, &thread); err != nil {
		t.Fatal(err)
	}
	if thread.Originator != nil {
		t.Fatal("stale originator survived decode")
	}
}

func TestRateLimitSpendControlSnapshot(t *testing.T) {
	var snapshot codex.RateLimitSnapshot
	data := `{"individualLimit":{"limit":"100.00","used":"100.00","remainingPercent":0,"resetsAt":123},"spendControlReached":false,"rateLimitReachedType":"workspace_member_usage_limit_reached"}`
	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.IndividualLimit == nil || snapshot.IndividualLimit.Limit != "100.00" || snapshot.IndividualLimit.RemainingPercent != 0 {
		t.Fatal("individual limit was lost")
	}
	if snapshot.SpendControlReached == nil || *snapshot.SpendControlReached {
		t.Fatal("explicit spend control state was lost")
	}
	if snapshot.RateLimitReachedType == nil || *snapshot.RateLimitReachedType != codex.RateLimitReachedTypeWorkspaceMemberUsageLimitReached {
		t.Fatal("limit reason was lost")
	}
	if err := json.Unmarshal([]byte(`{"individualLimit":{"limit":"100.00"}}`), &snapshot); err == nil {
		t.Fatal("accepted incomplete spend limit")
	}
}

func TestThreadOriginatorFilterRequest(t *testing.T) {
	transport := NewMockTransport()
	client := codex.NewClient(transport)
	defer client.Close()
	if err := transport.SetResponseData("thread/list", map[string]interface{}{"data": []interface{}{}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"desktop", "cli"}
	if _, err := client.Thread.List(context.Background(), codex.ThreadListParams{Originators: want}); err != nil {
		t.Fatal(err)
	}
	var params codex.ThreadListParams
	if err := json.Unmarshal(transport.GetSentRequest(0).Params, &params); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(params.Originators, want) {
		t.Fatalf("originators = %v", params.Originators)
	}
}

func TestDiscoveryErrorAndWebmcpPolicy(t *testing.T) {
	var status codex.McpServerStatus
	if err := json.Unmarshal([]byte(`{"authStatus":"unsupported","name":"server","resourceTemplates":[],"resources":[],"tools":{},"toolsError":"discovery failed"}`), &status); err != nil {
		t.Fatal(err)
	}
	if status.ToolsError == nil || *status.ToolsError != "discovery failed" {
		t.Fatal("discovery error was lost")
	}
	var policy codex.BrowserUseRequirements
	if err := json.Unmarshal([]byte(`{"allowWebmcp":false}`), &policy); err != nil {
		t.Fatal(err)
	}
	if policy.AllowWebmcp == nil || *policy.AllowWebmcp {
		t.Fatal("explicit denial was lost")
	}
}
