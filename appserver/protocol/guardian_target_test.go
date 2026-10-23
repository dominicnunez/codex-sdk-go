package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func guardianTargetJSON(completed bool, target string) []byte {
	extra := ""
	if completed {
		extra = `,"completedAtMs":2,"decisionSource":"agent"`
	}
	if target != "" {
		extra += `,"targetItemId":` + target
	}
	return []byte(`{"action":{"type":"networkAccess","host":"example.test","port":443,"protocol":"https","target":"example.test"},"review":{"status":"inProgress"},"reviewId":"review","startedAtMs":1,"threadId":"thread","turnId":"turn"` + extra + `}`)
}

func guardianTargetValue(completed bool) (any, func() *string) {
	if completed {
		value := &p.ItemGuardianApprovalReviewCompletedNotification{}
		return value, func() *string { return value.TargetItemID }
	}
	value := &p.ItemGuardianApprovalReviewStartedNotification{}
	return value, func() *string { return value.TargetItemID }
}

func TestGuardianTargetMetadataStates(t *testing.T) {
	for _, completed := range []bool{false, true} {
		for _, prior := range []string{"", `null`, `""`, `"item"`} {
			for _, next := range []string{"", `null`, `""`, `"item"`} {
				t.Run(fmt.Sprintf("completed=%v/%s/%s", completed, prior, next), func(t *testing.T) {
					value, target := guardianTargetValue(completed)
					if err := json.Unmarshal(guardianTargetJSON(completed, prior), value); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(guardianTargetJSON(completed, next), value); err != nil {
						t.Fatal(err)
					}
					want := next
					if next == "" || next == "null" {
						want = "null"
						if target() != nil {
							t.Fatal("absent/null target retained a value")
						}
					} else if target() == nil {
						t.Fatal("present target became nil")
					}
					encoded, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(encoded, &fields); err != nil {
						t.Fatal(err)
					}
					if string(fields["targetItemId"]) != want {
						t.Errorf("target became %s; want %s", fields["targetItemId"], want)
					}
				})
			}
		}
	}
}

func TestGuardianTargetDeliveryAndOwnership(t *testing.T) {
	for _, completed := range []bool{false, true} {
		for _, raw := range []string{"", `null`, `""`, `"item"`, `17`, `false`, `{}`, `[]`} {
			t.Run(fmt.Sprintf("completed=%v/%s", completed, raw), func(t *testing.T) {
				mock := NewMockTransport()
				failures := 0
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { failures++ }))
				var targets []*string
				receive := func(target *string) { targets = append(targets, target) }
				method := "item/autoApprovalReview/started"
				var remove func()
				if completed {
					method = "item/autoApprovalReview/completed"
					client.OnItemGuardianApprovalReviewCompleted(func(n p.ItemGuardianApprovalReviewCompletedNotification) { receive(n.TargetItemID) })
					remove = client.AddItemGuardianApprovalReviewCompletedListener(func(n p.ItemGuardianApprovalReviewCompletedNotification) { receive(n.TargetItemID) })
				} else {
					client.OnItemGuardianApprovalReviewStarted(func(n p.ItemGuardianApprovalReviewStartedNotification) { receive(n.TargetItemID) })
					remove = client.AddItemGuardianApprovalReviewStartedListener(func(n p.ItemGuardianApprovalReviewStartedNotification) { receive(n.TargetItemID) })
				}
				defer remove()
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: guardianTargetJSON(completed, raw)})
				valid := raw == "" || raw == "null" || raw[0] == '"'
				if !valid {
					if len(targets) != 0 || failures == 0 {
						t.Fatalf("invalid target delivery: calls=%d failures=%d", len(targets), failures)
					}
					return
				}
				if len(targets) != 2 || failures != 0 {
					t.Fatalf("valid target delivery: calls=%d failures=%d", len(targets), failures)
				}
				if raw == "" || raw == "null" {
					if targets[0] != nil || targets[1] != nil {
						t.Fatal("invented target identity")
					}
					return
				}
				var want string
				if err := json.Unmarshal([]byte(raw), &want); err != nil {
					t.Fatal(err)
				}
				if targets[0] == nil || targets[1] == nil || *targets[0] != want || *targets[1] != want {
					t.Fatal("target lost during typed delivery")
				}
				*targets[0] = "first mutation"
				if *targets[1] != want {
					t.Fatal("first recipient mutated second recipient")
				}
				*targets[1] = "second mutation"
				if *targets[0] != "first mutation" {
					t.Fatal("second recipient mutated first recipient")
				}
			})
		}
	}
}

func TestGuardianTargetMalformedDecodePreservesReceiver(t *testing.T) {
	for _, completed := range []bool{false, true} {
		for _, raw := range []string{`17`, `false`, `{}`, `[]`} {
			value, target := guardianTargetValue(completed)
			if err := json.Unmarshal(guardianTargetJSON(completed, `"old"`), value); err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			old := target()
			if err := json.Unmarshal(guardianTargetJSON(completed, raw), value); err == nil {
				t.Fatalf("completed=%v accepted malformed target %s", completed, raw)
			}
			after, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || target() != old {
				t.Fatal("failed decode changed receiver or references")
			}
		}
	}
}
