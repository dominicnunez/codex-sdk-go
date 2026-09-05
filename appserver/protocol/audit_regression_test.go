package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestLoginCredentialValuesRedact(t *testing.T) {
	for _, v := range []any{
		codex.ApiKeyLoginAccountParams{ApiKey: "audit-secret"},
		codex.ChatgptAuthTokensLoginAccountParams{AccessToken: "audit-secret"},
		codex.AmazonBedrockLoginAccountParams{ApiKey: "audit-secret"},
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "audit-secret") {
			t.Errorf("%T leaks through JSON", v)
		}
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			if strings.Contains(fmt.Sprintf(format, v), "audit-secret") {
				t.Errorf("%T leaks through %s", v, format)
			}
		}
		b, err = json.Marshal(map[string]any{"login": v})
		if err != nil || strings.Contains(string(b), "audit-secret") {
			t.Errorf("%T map value redaction failed", v)
		}
	}
}

func TestProjectNotificationsUpdateThreadCache(t *testing.T) {
	m := NewMockTransport()
	c := codex.NewClient(m)
	old := "old-project"
	c.CacheThreadState(codex.Thread{ID: "thread", ProjectID: &old})
	m.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/project/updated", Params: json.RawMessage(`{"threadId":"thread","projectId":"new-project"}`)})
	got, ok := c.ThreadStateSnapshot("thread")
	if !ok || got.ProjectID == nil || *got.ProjectID != "new-project" {
		t.Fatal("project notification did not update cached snapshot")
	}
	*got.ProjectID = "caller mutation"
	got, _ = c.ThreadStateSnapshot("thread")
	if *got.ProjectID != "new-project" {
		t.Fatal("snapshot aliases cache")
	}
	m.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/project/updated", Params: json.RawMessage(`{"threadId":"thread"}`)})
	got, _ = c.ThreadStateSnapshot("thread")
	if got.ProjectID == nil {
		t.Fatal("malformed notification cleared project")
	}
	m.InjectServerNotification(context.Background(), codex.Notification{Method: "thread/project/updated", Params: json.RawMessage(`{"threadId":"thread","projectId":null}`)})
	got, _ = c.ThreadStateSnapshot("thread")
	if got.ProjectID != nil {
		t.Fatal("project removal not reflected")
	}
}

func TestResetCreditAcceptsValidOutcomes(t *testing.T) {
	for _, outcome := range []string{"reset", "nothingToReset", "noCredit", "alreadyRedeemed"} {
		m := NewMockTransport()
		m.SetResponse("account/rateLimitResetCredit/consume", codex.Response{Result: json.RawMessage(fmt.Sprintf(`{"outcome":%q}`, outcome))})
		c := codex.NewClient(m)
		got, err := c.Account.ConsumeRateLimitResetCredit(context.Background(), codex.ConsumeAccountRateLimitResetCreditParams{IdempotencyKey: "audit"})
		if err != nil || string(got.Outcome) != outcome {
			t.Fatalf("outcome %s: got %v, error %v", outcome, got, err)
		}
	}
}

func TestResetCreditRejectsInvalidOutcomes(t *testing.T) {
	for _, payload := range []string{`{}`, `{"outcome":null}`, `{"outcome":"invalid"}`, `{"outcome":""}`, `{"outcome":42}`} {
		m := NewMockTransport()
		m.SetResponse("account/rateLimitResetCredit/consume", codex.Response{Result: json.RawMessage(payload)})
		c := codex.NewClient(m)
		_, err := c.Account.ConsumeRateLimitResetCredit(context.Background(), codex.ConsumeAccountRateLimitResetCreditParams{IdempotencyKey: "audit"})
		if err == nil {
			t.Errorf("accepted invalid response %s", payload)
		}
	}
}
