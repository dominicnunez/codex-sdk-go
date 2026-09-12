package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRateLimitReadRejectsUnknownReason(t *testing.T) {
	for _, payload := range []string{
		`{"rateLimits":{"rateLimitReachedType":"unknown"}}`,
		`{"rateLimits":{},"rateLimitsByLimitId":{"alias":{"rateLimitReachedType":"unknown"}}}`,
	} {
		transport := NewMockTransport()
		transport.SetResponse("account/rateLimits/read", codex.Response{Result: json.RawMessage(payload)})
		client := codex.NewClient(transport)
		if _, err := client.Account.GetRateLimits(context.Background()); err == nil {
			t.Errorf("accepted invalid reason: %s", payload)
		}
	}
}

func TestRateLimitReasonValidation(t *testing.T) {
	for _, value := range []string{
		`"rate_limit_reached"`, `"workspace_owner_credits_depleted"`,
		`"workspace_member_credits_depleted"`, `"workspace_owner_usage_limit_reached"`,
		`"workspace_member_usage_limit_reached"`, `null`,
	} {
		var snapshot codex.RateLimitSnapshot
		if err := json.Unmarshal([]byte(`{"rateLimitReachedType":`+value+`}`), &snapshot); err != nil {
			t.Fatalf("valid %s: %v", value, err)
		}
		if value == `null` {
			if snapshot.RateLimitReachedType != nil {
				t.Fatal("null became a block reason")
			}
		} else {
			encoded, err := json.Marshal(snapshot.RateLimitReachedType)
			if err != nil || string(encoded) != value {
				t.Fatalf("reason changed: %s, %v", encoded, err)
			}
		}
		if err := json.Unmarshal([]byte(`{}`), &snapshot); err != nil || snapshot.RateLimitReachedType != nil {
			t.Fatal("missing field retained a stale block reason")
		}
	}
	for _, value := range []string{`""`, `"unknown"`, `"RATE_LIMIT_REACHED"`, `42`, `true`, `{}`} {
		var snapshot codex.RateLimitSnapshot
		if err := json.Unmarshal([]byte(`{"rateLimitReachedType":`+value+`}`), &snapshot); err == nil {
			t.Errorf("accepted invalid reason %s", value)
		}
	}
}

func TestRateLimitNotificationRejectsInvalidReason(t *testing.T) {
	transport := NewMockTransport()
	var handlerErr error
	client := codex.NewClient(transport, codex.WithHandlerErrorCallback(func(_ string, err error) { handlerErr = err }))
	called := 0
	client.OnAccountRateLimitsUpdated(func(codex.AccountRateLimitsUpdatedNotification) { called++ })
	transport.InjectServerNotification(context.Background(), codex.Notification{Method: "account/rateLimits/updated", Params: json.RawMessage(`{"rateLimits":{"rateLimitReachedType":"unknown"}}`)})
	if called != 0 || handlerErr == nil {
		t.Fatal("invalid notification reached caller or was silently ignored")
	}
	transport.InjectServerNotification(context.Background(), codex.Notification{Method: "account/rateLimits/updated", Params: json.RawMessage(`{"rateLimits":{"rateLimitReachedType":"rate_limit_reached"}}`)})
	if called != 1 {
		t.Fatal("valid notification after invalid notification was rejected")
	}
}
