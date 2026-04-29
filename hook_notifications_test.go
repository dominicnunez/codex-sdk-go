package codex_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestHookNotificationTypesRejectMissingRequiredNestedFields(t *testing.T) {
	t.Run("hook output entry requires kind", func(t *testing.T) {
		var entry codex.HookOutputEntry
		err := json.Unmarshal([]byte(`{"text":"missing kind"}`), &entry)
		if err == nil || !strings.Contains(err.Error(), "missing required field") {
			t.Fatalf("json.Unmarshal error = %v; want missing required field failure", err)
		}
	})

	t.Run("hook run summary requires status", func(t *testing.T) {
		var run codex.HookRunSummary
		err := json.Unmarshal([]byte(`{
			"displayOrder": 1,
			"entries": [{"kind":"warning","text":"be careful"}],
			"eventName": "sessionStart",
			"executionMode": "sync",
			"handlerType": "command",
			"id": "run-1",
			"scope": "thread",
			"sourcePath": "/tmp/hook",
			"startedAt": 123
		}`), &run)
		if err == nil || !strings.Contains(err.Error(), "missing required field") {
			t.Fatalf("json.Unmarshal error = %v; want missing required field failure", err)
		}
	})

	t.Run("guardian review requires status", func(t *testing.T) {
		var review codex.GuardianApprovalReview
		err := json.Unmarshal([]byte(`{}`), &review)
		if err == nil || !strings.Contains(err.Error(), "missing required field") {
			t.Fatalf("json.Unmarshal error = %v; want missing required field failure", err)
		}
	})
}

func TestHookNotificationTypesRejectInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		target  func([]byte) error
		wantErr string
	}{
		{
			name:    "output entry kind",
			payload: `{"kind":"bogus","text":"bad"}`,
			target: func(data []byte) error {
				var entry codex.HookOutputEntry
				return json.Unmarshal(data, &entry)
			},
			wantErr: `invalid hook.output.kind "bogus"`,
		},
		{
			name: "run summary event name",
			payload: `{
				"displayOrder": 1,
				"entries": [{"kind":"warning","text":"be careful"}],
				"eventName": "bogus",
				"executionMode": "sync",
				"handlerType": "command",
				"id": "run-1",
				"scope": "thread",
				"sourcePath": "/tmp/hook",
				"startedAt": 123,
				"status": "running"
			}`,
			target: func(data []byte) error {
				var run codex.HookRunSummary
				return json.Unmarshal(data, &run)
			},
			wantErr: `invalid hook.eventName "bogus"`,
		},
		{
			name: "run summary execution mode",
			payload: `{
				"displayOrder": 1,
				"entries": [{"kind":"warning","text":"be careful"}],
				"eventName": "sessionStart",
				"executionMode": "bogus",
				"handlerType": "command",
				"id": "run-1",
				"scope": "thread",
				"sourcePath": "/tmp/hook",
				"startedAt": 123,
				"status": "running"
			}`,
			target: func(data []byte) error {
				var run codex.HookRunSummary
				return json.Unmarshal(data, &run)
			},
			wantErr: `invalid hook.executionMode "bogus"`,
		},
		{
			name: "run summary handler type",
			payload: `{
				"displayOrder": 1,
				"entries": [{"kind":"warning","text":"be careful"}],
				"eventName": "sessionStart",
				"executionMode": "sync",
				"handlerType": "bogus",
				"id": "run-1",
				"scope": "thread",
				"sourcePath": "/tmp/hook",
				"startedAt": 123,
				"status": "running"
			}`,
			target: func(data []byte) error {
				var run codex.HookRunSummary
				return json.Unmarshal(data, &run)
			},
			wantErr: `invalid hook.handlerType "bogus"`,
		},
		{
			name: "run summary scope",
			payload: `{
				"displayOrder": 1,
				"entries": [{"kind":"warning","text":"be careful"}],
				"eventName": "sessionStart",
				"executionMode": "sync",
				"handlerType": "command",
				"id": "run-1",
				"scope": "bogus",
				"sourcePath": "/tmp/hook",
				"startedAt": 123,
				"status": "running"
			}`,
			target: func(data []byte) error {
				var run codex.HookRunSummary
				return json.Unmarshal(data, &run)
			},
			wantErr: `invalid hook.scope "bogus"`,
		},
		{
			name: "run summary status",
			payload: `{
				"displayOrder": 1,
				"entries": [{"kind":"warning","text":"be careful"}],
				"eventName": "sessionStart",
				"executionMode": "sync",
				"handlerType": "command",
				"id": "run-1",
				"scope": "thread",
				"sourcePath": "/tmp/hook",
				"startedAt": 123,
				"status": "bogus"
			}`,
			target: func(data []byte) error {
				var run codex.HookRunSummary
				return json.Unmarshal(data, &run)
			},
			wantErr: `invalid hook.run.status "bogus"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target([]byte(tt.payload))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("json.Unmarshal error = %v; want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestGuardianApprovalReviewRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name:    "invalid status",
			payload: `{"status":"queued"}`,
			wantErr: `invalid guardian.review.status "queued"`,
		},
		{
			name:    "invalid risk level",
			payload: `{"status":"approved","riskLevel":"severe"}`,
			wantErr: `invalid guardian.review.riskLevel "severe"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var review codex.GuardianApprovalReview
			err := json.Unmarshal([]byte(tt.payload), &review)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("json.Unmarshal error = %v; want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestHookNotificationsReportHandlerErrorsForInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		params  string
		wantErr string
	}{
		{
			name:   "invalid nested output kind",
			method: "hook/started",
			params: `{
				"run": {
					"displayOrder": 1,
					"entries": [{"kind":"bogus","text":"be careful"}],
					"eventName": "sessionStart",
					"executionMode": "sync",
					"handlerType": "command",
					"id": "run-1",
					"scope": "thread",
					"sourcePath": "/tmp/hook",
					"startedAt": 123,
					"status": "running"
				},
				"threadId": "thread-1"
			}`,
			wantErr: `invalid hook.output.kind "bogus"`,
		},
		{
			name:   "invalid run status",
			method: "hook/completed",
			params: `{
				"run": {
					"displayOrder": 1,
					"entries": [{"kind":"warning","text":"be careful"}],
					"eventName": "sessionStart",
					"executionMode": "sync",
					"handlerType": "command",
					"id": "run-1",
					"scope": "thread",
					"sourcePath": "/tmp/hook",
					"startedAt": 123,
					"status": "mystery"
				},
				"threadId": "thread-1"
			}`,
			wantErr: `invalid hook.run.status "mystery"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()

			var (
				gotMethod string
				gotErr    error
				called    bool
			)
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
				gotMethod = method
				gotErr = err
			}))

			client.OnHookStarted(func(codex.HookStartedNotification) {
				called = true
			})
			client.OnHookCompleted(func(codex.HookCompletedNotification) {
				called = true
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  tt.method,
				Params:  json.RawMessage(tt.params),
			})

			if called {
				t.Fatal("handler should not be called for invalid hook enums")
			}
			if gotMethod != tt.method {
				t.Fatalf("handler error method = %q; want %q", gotMethod, tt.method)
			}
			if gotErr == nil || !strings.Contains(gotErr.Error(), tt.wantErr) {
				t.Fatalf("handler error = %v; want substring %q", gotErr, tt.wantErr)
			}
		})
	}
}

func TestHookAndGuardianNotificationsReportHandlerErrorsForMalformedPayloads(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		params   string
		register func(*codex.Client, *bool)
	}{
		{
			name:   "hook started",
			method: "hook/started",
			params: `{
				"run": {
					"displayOrder": 1,
					"entries": [{"kind":"warning","text":"be careful"}],
					"eventName": "sessionStart",
					"executionMode": "sync",
					"handlerType": "command",
					"id": "run-1",
					"scope": "thread",
					"sourcePath": "/tmp/hook",
					"startedAt": 123
				},
				"threadId": "thread-1"
			}`,
			register: func(client *codex.Client, called *bool) {
				client.OnHookStarted(func(codex.HookStartedNotification) {
					*called = true
				})
			},
		},
		{
			name:   "hook completed",
			method: "hook/completed",
			params: `{
				"run": {
					"displayOrder": 1,
					"entries": [{"kind":"warning","text":"be careful"}],
					"eventName": "sessionStart",
					"executionMode": "sync",
					"handlerType": "command",
					"scope": "thread",
					"sourcePath": "/tmp/hook",
					"startedAt": 123,
					"status": "completed"
				},
				"threadId": "thread-1"
			}`,
			register: func(client *codex.Client, called *bool) {
				client.OnHookCompleted(func(codex.HookCompletedNotification) {
					*called = true
				})
			},
		},
		{
			name:   "guardian review started",
			method: "item/autoApprovalReview/started",
			params: `{
				"review": {},
				"targetItemId": "item-1",
				"threadId": "thread-1",
				"turnId": "turn-1"
			}`,
			register: func(client *codex.Client, called *bool) {
				client.OnItemGuardianApprovalReviewStarted(func(codex.ItemGuardianApprovalReviewStartedNotification) {
					*called = true
				})
			},
		},
		{
			name:   "guardian review completed",
			method: "item/autoApprovalReview/completed",
			params: `{
				"review": {},
				"targetItemId": "item-1",
				"threadId": "thread-1",
				"turnId": "turn-1"
			}`,
			register: func(client *codex.Client, called *bool) {
				client.OnItemGuardianApprovalReviewCompleted(func(codex.ItemGuardianApprovalReviewCompletedNotification) {
					*called = true
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()

			var (
				gotMethod string
				gotErr    error
				called    bool
			)
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
				gotMethod = method
				gotErr = err
			}))
			tt.register(client, &called)

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  tt.method,
				Params:  json.RawMessage(tt.params),
			})

			if called {
				t.Fatal("handler should not be called for malformed payload")
			}
			if gotMethod != tt.method {
				t.Fatalf("handler error method = %q; want %q", gotMethod, tt.method)
			}
			if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
				t.Fatalf("handler error = %v; want missing required field failure", gotErr)
			}
		})
	}
}

func TestGuardianNotificationsReportHandlerErrorsForInvalidReviewEnums(t *testing.T) {
	tests := []struct {
		name   string
		method string
		params string
	}{
		{
			name:   "invalid review status",
			method: "item/autoApprovalReview/started",
			params: `{
				"action": {},
				"review": {"status":"queued"},
				"reviewId": "review-1",
				"targetItemId": "item-1",
				"threadId": "thread-1",
				"turnId": "turn-1"
			}`,
		},
		{
			name:   "invalid review risk level",
			method: "item/autoApprovalReview/completed",
			params: `{
				"action": {},
				"decisionSource": "agent",
				"review": {"status":"approved","riskLevel":"severe"},
				"reviewId": "review-1",
				"targetItemId": "item-1",
				"threadId": "thread-1",
				"turnId": "turn-1"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()

			var (
				gotMethod string
				gotErr    error
				called    bool
			)
			client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
				gotMethod = method
				gotErr = err
			}))

			client.OnItemGuardianApprovalReviewStarted(func(codex.ItemGuardianApprovalReviewStartedNotification) {
				called = true
			})
			client.OnItemGuardianApprovalReviewCompleted(func(codex.ItemGuardianApprovalReviewCompletedNotification) {
				called = true
			})

			mock.InjectServerNotification(context.Background(), codex.Notification{
				JSONRPC: "2.0",
				Method:  tt.method,
				Params:  json.RawMessage(tt.params),
			})

			if called {
				t.Fatal("handler should not be called for invalid guardian review enums")
			}
			if gotMethod != tt.method {
				t.Fatalf("handler error method = %q; want %q", gotMethod, tt.method)
			}
			if gotErr == nil || (!strings.Contains(gotErr.Error(), "invalid guardian.review.status") && !strings.Contains(gotErr.Error(), "invalid guardian.review.riskLevel")) {
				t.Fatalf("handler error = %v; want invalid guardian enum failure", gotErr)
			}
		})
	}
}
