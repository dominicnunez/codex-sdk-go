package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

// TestTurnStart tests turn/start method
func TestTurnStart(t *testing.T) {
	tests := []struct {
		name   string
		params codex.TurnStartParams
		want   map[string]interface{}
	}{
		{
			name: "minimal params",
			params: codex.TurnStartParams{
				ThreadID: "thread-123",
				Input: []codex.UserInput{
					&codex.TextUserInput{Text: "Hello"},
				},
			},
			want: map[string]interface{}{
				"turn": map[string]interface{}{
					"id":     "turn-456",
					"status": "inProgress",
					"items":  []interface{}{},
				},
			},
		},
		{
			name: "full params with overrides",
			params: codex.TurnStartParams{
				ThreadID: "thread-123",
				Input: []codex.UserInput{
					&codex.TextUserInput{Text: "Hello"},
					&codex.ImageUserInput{URL: "https://example.com/image.png"},
				},
				Model: strPtr("claude-opus-4"),
				Cwd:   strPtr("/workspace"),
			},
			want: map[string]interface{}{
				"turn": map[string]interface{}{
					"id":     "turn-456",
					"status": "completed",
					"items":  []interface{}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTransport := NewMockTransport()
			_ = mockTransport.SetResponseData("turn/start", tt.want)
			client := codex.NewClient(mockTransport)

			resp, err := client.Turn.Start(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Turn.Start() error = %v", err)
			}

			if resp.Turn.ID == "" {
				t.Error("Turn.Start() response missing turn.id")
			}

			req := mockTransport.GetSentRequest(0)
			if req == nil || req.Method != "turn/start" {
				t.Errorf("Turn.Start() sent method = %v, want turn/start", req.Method)
			}
		})
	}
}

// TestTurnInterrupt tests turn/interrupt method
func TestTurnInterrupt(t *testing.T) {
	mockTransport := NewMockTransport()
	_ = mockTransport.SetResponseData("turn/interrupt", map[string]interface{}{})
	client := codex.NewClient(mockTransport)

	params := codex.TurnInterruptParams{
		ThreadID: "thread-123",
		TurnID:   "turn-456",
	}

	resp, err := client.Turn.Interrupt(context.Background(), params)
	if err != nil {
		t.Fatalf("Turn.Interrupt() error = %v", err)
	}

	// Empty response, just verify no error
	_ = resp

	req := mockTransport.GetSentRequest(0)
	if req == nil || req.Method != "turn/interrupt" {
		t.Errorf("Turn.Interrupt() sent method = %v, want turn/interrupt", req.Method)
	}

	// Verify params serialization
	var params2 codex.TurnInterruptParams
	if err := json.Unmarshal(req.Params, &params2); err != nil {
		t.Fatalf("Turn.Interrupt() params unmarshal error = %v", err)
	}
	if params2.ThreadID != params.ThreadID {
		t.Errorf("Turn.Interrupt() params.threadId = %v, want %v", params2.ThreadID, params.ThreadID)
	}
	if params2.TurnID != params.TurnID {
		t.Errorf("Turn.Interrupt() params.turnId = %v, want %v", params2.TurnID, params.TurnID)
	}
}

// TestTurnSteer tests turn/steer method
func TestTurnSteer(t *testing.T) {
	mockTransport := NewMockTransport()
	_ = mockTransport.SetResponseData("turn/steer", map[string]interface{}{
		"turnId": "turn-789",
	})
	client := codex.NewClient(mockTransport)

	params := codex.TurnSteerParams{
		ThreadID:       "thread-123",
		ExpectedTurnID: "turn-456",
		Input: []codex.UserInput{
			&codex.TextUserInput{Text: "Actually, do this instead"},
		},
	}

	resp, err := client.Turn.Steer(context.Background(), params)
	if err != nil {
		t.Fatalf("Turn.Steer() error = %v", err)
	}

	if resp.TurnID != "turn-789" {
		t.Errorf("Turn.Steer() response.turnId = %v, want turn-789", resp.TurnID)
	}

	req := mockTransport.GetSentRequest(0)
	if req == nil || req.Method != "turn/steer" {
		t.Errorf("Turn.Steer() sent method = %v, want turn/steer", req.Method)
	}

	// Verify params serialization
	var params2 codex.TurnSteerParams
	if err := json.Unmarshal(req.Params, &params2); err != nil {
		t.Fatalf("Turn.Steer() params unmarshal error = %v", err)
	}
	if params2.ThreadID != params.ThreadID {
		t.Errorf("Turn.Steer() params.threadId = %v, want %v", params2.ThreadID, params.ThreadID)
	}
	if params2.ExpectedTurnID != params.ExpectedTurnID {
		t.Errorf("Turn.Steer() params.expectedTurnId = %v, want %v", params2.ExpectedTurnID, params.ExpectedTurnID)
	}
}

// TestTurnServiceMethodSignatures verifies TurnService has all required methods
func TestTurnServiceMethodSignatures(t *testing.T) {
	mockTransport := NewMockTransport()
	client := codex.NewClient(mockTransport)

	// Compile-time check that methods exist with correct signatures
	var _ = client.Turn.Start
	var _ = client.Turn.Interrupt
	var _ = client.Turn.Steer
}

// TestTurnStartedNotification tests TurnStartedNotification dispatch
func TestTurnStartedNotification(t *testing.T) {
	mockTransport := NewMockTransport()
	client := codex.NewClient(mockTransport)

	var gotNotif codex.TurnStartedNotification
	client.OnTurnStarted(func(notif codex.TurnStartedNotification) {
		gotNotif = notif
	})

	// Inject server notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/started",
		Params: json.RawMessage(`{
			"threadId": "thread-123",
			"turn": {
				"id": "turn-456",
				"status": "inProgress",
				"items": []
			}
		}`),
	}
	mockTransport.InjectServerNotification(context.Background(), notif)

	if gotNotif.ThreadID != "thread-123" {
		t.Errorf("OnTurnStarted() threadId = %v, want thread-123", gotNotif.ThreadID)
	}
	if gotNotif.Turn.ID != "turn-456" {
		t.Errorf("OnTurnStarted() turn.id = %v, want turn-456", gotNotif.Turn.ID)
	}
	if gotNotif.Turn.Status != "inProgress" {
		t.Errorf("OnTurnStarted() turn.status = %v, want inProgress", gotNotif.Turn.Status)
	}
}

// TestTurnCompletedNotification tests TurnCompletedNotification dispatch
func TestTurnCompletedNotification(t *testing.T) {
	mockTransport := NewMockTransport()
	client := codex.NewClient(mockTransport)

	var gotNotif codex.TurnCompletedNotification
	client.OnTurnCompleted(func(notif codex.TurnCompletedNotification) {
		gotNotif = notif
	})

	// Inject server notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params: json.RawMessage(`{
			"threadId": "thread-123",
			"turn": {
				"id": "turn-456",
				"status": "completed",
				"items": []
			}
		}`),
	}
	mockTransport.InjectServerNotification(context.Background(), notif)

	if gotNotif.ThreadID != "thread-123" {
		t.Errorf("OnTurnCompleted() threadId = %v, want thread-123", gotNotif.ThreadID)
	}
	if gotNotif.Turn.ID != "turn-456" {
		t.Errorf("OnTurnCompleted() turn.id = %v, want turn-456", gotNotif.Turn.ID)
	}
	if gotNotif.Turn.Status != "completed" {
		t.Errorf("OnTurnCompleted() turn.status = %v, want completed", gotNotif.Turn.Status)
	}
}

// TestTurnPlanUpdatedNotification tests TurnPlanUpdatedNotification dispatch
func TestTurnPlanUpdatedNotification(t *testing.T) {
	mockTransport := NewMockTransport()
	client := codex.NewClient(mockTransport)

	var gotNotif codex.TurnPlanUpdatedNotification
	client.OnTurnPlanUpdated(func(notif codex.TurnPlanUpdatedNotification) {
		gotNotif = notif
	})

	// Inject server notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/plan/updated",
		Params: json.RawMessage(`{
			"threadId": "thread-123",
			"turnId": "turn-456",
			"plan": [
				{"step": "Read the file", "status": "completed"},
				{"step": "Write the code", "status": "inProgress"}
			],
			"explanation": "Here's my plan"
		}`),
	}
	mockTransport.InjectServerNotification(context.Background(), notif)

	if gotNotif.ThreadID != "thread-123" {
		t.Errorf("OnTurnPlanUpdated() threadId = %v, want thread-123", gotNotif.ThreadID)
	}
	if gotNotif.TurnID != "turn-456" {
		t.Errorf("OnTurnPlanUpdated() turnId = %v, want turn-456", gotNotif.TurnID)
	}
	if len(gotNotif.Plan) != 2 {
		t.Errorf("OnTurnPlanUpdated() len(plan) = %v, want 2", len(gotNotif.Plan))
	}
	if gotNotif.Plan[0].Step != "Read the file" {
		t.Errorf("OnTurnPlanUpdated() plan[0].step = %v, want 'Read the file'", gotNotif.Plan[0].Step)
	}
	if gotNotif.Plan[0].Status != "completed" {
		t.Errorf("OnTurnPlanUpdated() plan[0].status = %v, want completed", gotNotif.Plan[0].Status)
	}
	if gotNotif.Explanation == nil || *gotNotif.Explanation != "Here's my plan" {
		t.Errorf("OnTurnPlanUpdated() explanation = %v, want 'Here's my plan'", gotNotif.Explanation)
	}
}

// TestTurnDiffUpdatedNotification tests TurnDiffUpdatedNotification dispatch
func TestTurnDiffUpdatedNotification(t *testing.T) {
	mockTransport := NewMockTransport()
	client := codex.NewClient(mockTransport)

	var gotNotif codex.TurnDiffUpdatedNotification
	client.OnTurnDiffUpdated(func(notif codex.TurnDiffUpdatedNotification) {
		gotNotif = notif
	})

	// Inject server notification
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/diff/updated",
		Params: json.RawMessage(`{
			"threadId": "thread-123",
			"turnId": "turn-456",
			"diff": "--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n+// New line\n package main\n"
		}`),
	}
	mockTransport.InjectServerNotification(context.Background(), notif)

	if gotNotif.ThreadID != "thread-123" {
		t.Errorf("OnTurnDiffUpdated() threadId = %v, want thread-123", gotNotif.ThreadID)
	}
	if gotNotif.TurnID != "turn-456" {
		t.Errorf("OnTurnDiffUpdated() turnId = %v, want turn-456", gotNotif.TurnID)
	}
	if gotNotif.Diff == "" {
		t.Error("OnTurnDiffUpdated() diff is empty")
	}
}

// TestTurnStartParamsApprovalPolicyMarshal verifies that TurnStartParams with
// each AskForApproval variant marshals correctly for the wire protocol.
func TestTurnStartParamsApprovalPolicyMarshal(t *testing.T) {
	tests := []struct {
		name   string
		policy codex.AskForApproval
		want   string
	}{
		{
			name:   "never",
			policy: codex.ApprovalPolicyNever,
			want:   `"never"`,
		},
		{
			name:   "untrusted",
			policy: codex.ApprovalPolicyUntrusted,
			want:   `"untrusted"`,
		},
		{
			name:   "on-failure",
			policy: codex.ApprovalPolicyOnFailure,
			want:   `"on-failure"`,
		},
		{
			name:   "on-request",
			policy: codex.ApprovalPolicyOnRequest,
			want:   `"on-request"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := codex.TurnStartParams{
				ThreadID:       "thread-1",
				Input:          []codex.UserInput{&codex.TextUserInput{Text: "hi"}},
				ApprovalPolicy: &tt.policy,
			}
			data, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}
			got := string(raw["approvalPolicy"])
			if got != tt.want {
				t.Errorf("approvalPolicy = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestTurnUserInputTypes tests various UserInput type serialization
func TestTurnUserInputTypes(t *testing.T) {
	tests := []struct {
		name  string
		input codex.UserInput
		want  string
	}{
		{
			name:  "text input",
			input: &codex.TextUserInput{Text: "Hello"},
			want:  `{"type":"text","text":"Hello"}`,
		},
		{
			name:  "image input",
			input: &codex.ImageUserInput{URL: "https://example.com/image.png"},
			want:  `{"type":"image","url":"https://example.com/image.png"}`,
		},
		{
			name:  "local image input",
			input: &codex.LocalImageUserInput{Path: "/path/to/image.png"},
			want:  `{"type":"localImage","path":"/path/to/image.png"}`,
		},
		{
			name:  "skill input",
			input: &codex.SkillUserInput{Name: "skill-name", Path: "/path/to/skill"},
			want:  `{"type":"skill","name":"skill-name","path":"/path/to/skill"}`,
		},
		{
			name:  "mention input",
			input: &codex.MentionUserInput{Name: "mention-name", Path: "/path/to/mention"},
			want:  `{"type":"mention","name":"mention-name","path":"/path/to/mention"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			if !jsonEqual(string(data), tt.want) {
				t.Errorf("json.Marshal() = %s, want %s", data, tt.want)
			}
		})
	}
}
