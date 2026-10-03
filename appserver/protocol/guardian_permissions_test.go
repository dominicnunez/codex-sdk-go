package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func guardianScopeJSON(completed bool, scope string) []byte {
	extra := ""
	if completed {
		extra = `,"completedAtMs":2,"decisionSource":"agent"`
	}
	return []byte(`{"action":{"type":"requestPermissions","permissions":{"fileSystem":` + scope + `}},"review":{"status":"inProgress"},"reviewId":"review","startedAtMs":1,"threadId":"thread","turnId":"turn"` + extra + `}`)
}

func TestGuardianPermissionScopePreservation(t *testing.T) {
	for _, completed := range []bool{false, true} {
		for _, additive := range []bool{false, true} {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			var action any
			method := "item/autoApprovalReview/started"
			if completed {
				method = "item/autoApprovalReview/completed"
				handler := func(n codex.ItemGuardianApprovalReviewCompletedNotification) { action = n.Action }
				if additive {
					client.AddItemGuardianApprovalReviewCompletedListener(handler)
				} else {
					client.OnItemGuardianApprovalReviewCompleted(handler)
				}
			} else {
				handler := func(n codex.ItemGuardianApprovalReviewStartedNotification) { action = n.Action }
				if additive {
					client.AddItemGuardianApprovalReviewStartedListener(handler)
				} else {
					client.OnItemGuardianApprovalReviewStarted(handler)
				}
			}
			for _, depth := range []string{"3", "9007199254740993", "18446744073709551615"} {
				action = nil
				scope := `{"entries":[],"globScanMaxDepth":` + depth + `,"read":["../raw"]}`
				mock.InjectServerNotification(context.Background(), codex.Notification{Method: method, Params: guardianScopeJSON(completed, scope)})
				if action == nil {
					t.Fatal("valid guardian scope did not reach typed listener")
				}
				encoded, err := json.Marshal(action)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(encoded), `"globScanMaxDepth":`+depth) || !strings.Contains(string(encoded), `"entries":[]`) {
					t.Fatalf("guardian scope changed: %s", encoded)
				}
				if _, ok := action.(map[string]interface{}); !ok {
					t.Fatalf("generic action shape changed: %T", action)
				}
			}
		}
	}
}

func TestGuardianRejectsMalformedScopeBeforeTypedListeners(t *testing.T) {
	for _, completed := range []bool{false, true} {
		for _, scope := range []string{`{"entries":[null]}`, `{"globScanMaxDepth":0}`, `{"entries":[{"access":"future","path":{"type":"path","path":"x"}}]}`, `{"globScanMaxDepth":0,"globScanMaxDepth":3}`} {
			mock := NewMockTransport()
			called, reported := 0, 0
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(string, error) { reported++ }))
			method := "item/autoApprovalReview/started"
			if completed {
				method = "item/autoApprovalReview/completed"
				client.OnItemGuardianApprovalReviewCompleted(func(codex.ItemGuardianApprovalReviewCompletedNotification) { called++ })
				client.AddItemGuardianApprovalReviewCompletedListener(func(codex.ItemGuardianApprovalReviewCompletedNotification) { called++ })
			} else {
				client.OnItemGuardianApprovalReviewStarted(func(codex.ItemGuardianApprovalReviewStartedNotification) { called++ })
				client.AddItemGuardianApprovalReviewStartedListener(func(codex.ItemGuardianApprovalReviewStartedNotification) { called++ })
			}
			mock.InjectServerNotification(context.Background(), codex.Notification{Method: method, Params: guardianScopeJSON(completed, scope)})
			if called != 0 || reported == 0 {
				t.Fatalf("malformed guardian scope admitted: calls=%d reports=%d", called, reported)
			}
		}
	}
}

func TestGuardianOtherActionsRemainGeneric(t *testing.T) {
	data := []byte(`{"action":{"type":"command","command":"echo hi","cwd":"relative","source":"agent","sequence":3},"review":{"status":"inProgress"},"reviewId":"review","startedAtMs":1,"threadId":"thread","turnId":"turn"}`)
	var notification codex.ItemGuardianApprovalReviewStartedNotification
	if err := json.Unmarshal(data, &notification); err != nil {
		t.Fatal(err)
	}
	object := notification.Action.(map[string]interface{})
	if object["sequence"] != float64(3) || object["cwd"] != "relative" {
		t.Fatalf("unrelated generic action changed: %+v", object)
	}
}

func TestGuardianDirectScopeAndDuplicateValidation(t *testing.T) {
	scope := `{"entries":[{"access":"deny","path":{"type":"glob_pattern","pattern":"**/raw"}},{"access":"write","path":{"type":"special","value":{"kind":"unknown","path":"","subpath":"../raw"}}}],"globScanMaxDepth":9007199254740993}`
	for _, completed := range []bool{false, true} {
		decode := func(data []byte) (any, error) {
			if completed {
				var n codex.ItemGuardianApprovalReviewCompletedNotification
				err := json.Unmarshal(data, &n)
				return n.Action, err
			}
			var n codex.ItemGuardianApprovalReviewStartedNotification
			err := json.Unmarshal(data, &n)
			return n.Action, err
		}
		action, err := decode(guardianScopeJSON(completed, scope))
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(action)
		if err != nil {
			t.Fatal(err)
		}
		assertPermissionJSON(t, encoded, []byte(`{"type":"requestPermissions","permissions":{"fileSystem":`+scope+`}}`))
		for _, invalid := range []string{
			`{"type":"requestPermissions"}`,
			`{"type":"requestPermissions","permissions":null}`,
			`{"type":"requestPermissions","permissions":{"extra":true}}`,
			`{"type":"requestPermissions","permissions":{"fileSystem":{"entries":[null]}},"permissions":{"fileSystem":{}}}`,
			`{"type":"requestPermissions","permissions":{"fileSystem":{"globScanMaxDepth":0},"fileSystem":{}}}`,
		} {
			valid := string(guardianScopeJSON(completed, `{}`))
			// Prefix a malformed action before a valid duplicate. Every recognized
			// custom semantic occurrence must be checked before admission.
			data := []byte(`{"action":` + invalid + `,` + valid[1:])
			if _, err := decode(data); err == nil {
				t.Fatalf("completed=%v admitted earlier invalid action: %s", completed, data)
			}
		}
	}
}
