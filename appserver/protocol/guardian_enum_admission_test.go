package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestGuardianReviewRejectsUnknownAuthorization(t *testing.T) {
	var review p.GuardianApprovalReview
	if err := json.Unmarshal([]byte(`{"status":"approved","userAuthorization":"administrator"}`), &review); err == nil {
		t.Fatal("unknown user authorization admitted")
	}
}

func guardianEnumJSON(completed bool, authorization, source string) []byte {
	extra := ""
	if authorization != "" {
		authorization = `,"userAuthorization":` + authorization
	}
	if completed {
		extra = `,"completedAtMs":2,"decisionSource":` + source
	}
	return []byte(`{"action":{"type":"networkAccess","host":"example.test","port":443,"protocol":"https","target":"example.test"},"review":{"status":"approved"` + authorization + `},"reviewId":"r","startedAtMs":1,"threadId":"t","turnId":"u"` + extra + `}`)
}

func TestGuardianReviewEnumAdmission(t *testing.T) {
	for _, completed := range []bool{false, true} {
		for _, auth := range []string{"", `null`, `"unknown"`, `"low"`, `"medium"`, `"high"`, `"administrator"`, `""`, `17`, `false`, `{}`, `[]`, `"administrator","userAuthorization":"high"`} {
			t.Run(fmt.Sprintf("completed=%v/auth=%s", completed, auth), func(t *testing.T) {
				valid := auth == "" || auth == "null" || auth == `"unknown"` || auth == `"low"` || auth == `"medium"` || auth == `"high"`
				checkGuardianEnumDelivery(t, completed, auth, `"agent"`, valid)
			})
		}
	}
	for _, source := range []string{`"agent"`, `"user"`, `""`, `null`, `17`, `false`, `{}`, `[]`, `"user","decisionSource":"agent"`} {
		t.Run("source="+source, func(t *testing.T) {
			checkGuardianEnumDelivery(t, true, `"high"`, source, source == `"agent"`)
		})
	}
}

func checkGuardianEnumDelivery(t *testing.T, completed bool, auth, source string, valid bool) {
	t.Helper()
	mock := NewMockTransport()
	calls, failures := 0, 0
	client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { failures++ }))
	receive := func(review p.GuardianApprovalReview) {
		t.Helper()
		calls++
		if auth == "" || auth == "null" {
			if review.UserAuthorization != nil {
				t.Error("invented user authorization")
			}
		} else {
			var want p.GuardianUserAuthorization
			if err := json.Unmarshal([]byte(auth), &want); err != nil {
				t.Fatal(err)
			}
			if review.UserAuthorization == nil || *review.UserAuthorization != want {
				t.Error("authorization lost during delivery")
			}
		}
	}
	method := "item/autoApprovalReview/started"
	var remove func()
	if completed {
		method = "item/autoApprovalReview/completed"
		handler := func(n p.ItemGuardianApprovalReviewCompletedNotification) {
			receive(n.Review)
			if n.DecisionSource != p.AutoReviewDecisionSourceAgent {
				t.Error("decision source lost during delivery")
			}
		}
		client.OnItemGuardianApprovalReviewCompleted(handler)
		remove = client.AddItemGuardianApprovalReviewCompletedListener(handler)
	} else {
		handler := func(n p.ItemGuardianApprovalReviewStartedNotification) { receive(n.Review) }
		client.OnItemGuardianApprovalReviewStarted(handler)
		remove = client.AddItemGuardianApprovalReviewStartedListener(handler)
	}
	defer remove()
	mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: guardianEnumJSON(completed, auth, source)})
	if valid {
		if calls != 2 || failures != 0 {
			t.Fatalf("valid delivery: calls=%d failures=%d", calls, failures)
		}
		return
	}
	if calls != 0 || failures == 0 {
		t.Fatalf("invalid delivery: calls=%d failures=%d", calls, failures)
	}
	// A rejected notification must not prevent later valid delivery.
	auth = `"high"`
	mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: guardianEnumJSON(completed, auth, `"agent"`)})
	if calls != 2 {
		t.Fatalf("recovery delivery: calls=%d", calls)
	}
}

func TestGuardianReviewEnumReceiverReuse(t *testing.T) {
	review := p.GuardianApprovalReview{Status: p.GuardianApprovalReviewStatusApproved, UserAuthorization: p.Ptr(p.GuardianUserAuthorizationHigh)}
	for _, auth := range []string{`"administrator"`, `"administrator","userAuthorization":"high"`, `"administrator","user\u0041uthorization":"high"`, `false`} {
		old := review.UserAuthorization
		if err := json.Unmarshal([]byte(`{"status":"denied","userAuthorization":`+auth+`}`), &review); err == nil {
			t.Fatal("invalid authorization admitted")
		}
		if review.Status != p.GuardianApprovalReviewStatusApproved || review.UserAuthorization != old || *old != p.GuardianUserAuthorizationHigh {
			t.Fatal("failed decode changed review receiver")
		}
	}
	for _, wire := range []string{`{"status":"approved"}`, `{"status":"approved","userAuthorization":null}`, `{"status":"approved","userAuthorization":"high","userAuthorization":null}`} {
		review.UserAuthorization = p.Ptr(p.GuardianUserAuthorizationHigh)
		if err := json.Unmarshal([]byte(wire), &review); err != nil || review.UserAuthorization != nil {
			t.Fatalf("absent/null authorization did not reset: %#v, %v", review, err)
		}
	}
	var notification p.ItemGuardianApprovalReviewCompletedNotification
	if err := json.Unmarshal(guardianEnumJSON(true, `"high"`, `"agent"`), &notification); err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(notification)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{`"user"`, `null`, `"user","decisionSource":"agent"`, `"user","decision\u0053ource":"agent"`} {
		if err := json.Unmarshal(guardianEnumJSON(true, `"low"`, source), &notification); err == nil {
			t.Fatal("invalid decision source admitted")
		}
		after, err := json.Marshal(notification)
		if err != nil || string(after) != string(before) {
			t.Fatalf("failed decode changed notification: %s, %v", after, err)
		}
	}
}

func TestGuardianReviewScalarEnvelopes(t *testing.T) {
	var auth struct {
		p.GuardianUserAuthorization `json:"userAuthorization"`
		Note                        string `json:"note"`
	}
	if err := json.Unmarshal([]byte(`{"userAuthorization":"high","note":"kept"}`), &auth); err != nil || auth.Note != "kept" || auth.GuardianUserAuthorization != p.GuardianUserAuthorizationHigh {
		t.Fatalf("authorization envelope changed: %#v, %v", auth, err)
	}
	var source struct {
		p.AutoReviewDecisionSource `json:"decisionSource"`
		Note                       string `json:"note"`
	}
	if err := json.Unmarshal([]byte(`{"decisionSource":"agent","note":"kept"}`), &source); err != nil || source.Note != "kept" || source.AutoReviewDecisionSource != p.AutoReviewDecisionSourceAgent {
		t.Fatalf("decision source envelope changed: %#v, %v", source, err)
	}
}

func TestGuardianReviewKeepsExistingErrorsFirst(t *testing.T) {
	var review p.GuardianApprovalReview
	err := json.Unmarshal([]byte(`{"userAuthorization":"administrator","status":"queued"}`), &review)
	if err == nil || !strings.Contains(err.Error(), `invalid guardian.review.status "queued"`) {
		t.Fatalf("existing status error masked: %v", err)
	}
	var notification p.ItemGuardianApprovalReviewCompletedNotification
	err = json.Unmarshal([]byte(`{"decisionSource":"user","review":{"status":"queued"}}`), &notification)
	if err == nil || !strings.Contains(err.Error(), `invalid guardian.review.status "queued"`) {
		t.Fatalf("existing nested review error masked: %v", err)
	}
}

func TestGuardianReviewRejectsUnknownDecisionSource(t *testing.T) {
	var notification p.ItemGuardianApprovalReviewCompletedNotification
	wire := `{"action":{"type":"networkAccess","host":"example.test","port":443,"protocol":"https","target":"example.test"},"review":{"status":"approved"},"reviewId":"r","startedAtMs":1,"completedAtMs":2,"decisionSource":"user","threadId":"t","turnId":"u"}`
	if err := json.Unmarshal([]byte(wire), &notification); err == nil {
		t.Fatal("unknown decision source admitted")
	}
}
