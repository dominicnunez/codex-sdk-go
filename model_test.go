package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestModelList(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.ModelListParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.ModelListResponse)
	}{
		{
			name:   "minimal list",
			params: codex.ModelListParams{},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"id":                         "claude-opus-4-6",
						"model":                      "claude-opus-4-6",
						"displayName":                "Claude Opus 4.6",
						"description":                "Most capable Claude model",
						"hidden":                     false,
						"isDefault":                  true,
						"defaultReasoningEffort":     "medium",
						"supportedReasoningEfforts":  []interface{}{},
						"inputModalities":            []interface{}{"text", "image"},
						"supportsPersonality":        false,
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.ModelListResponse) {
				if len(resp.Data) != 1 {
					t.Errorf("expected 1 model, got %d", len(resp.Data))
				}
				if resp.Data[0].ID != "claude-opus-4-6" {
					t.Errorf("expected ID = claude-opus-4-6, got %s", resp.Data[0].ID)
				}
				if resp.Data[0].DisplayName != "Claude Opus 4.6" {
					t.Errorf("expected DisplayName = Claude Opus 4.6, got %s", resp.Data[0].DisplayName)
				}
				if resp.Data[0].IsDefault != true {
					t.Errorf("expected IsDefault = true, got %v", resp.Data[0].IsDefault)
				}
				if resp.Data[0].DefaultReasoningEffort != "medium" {
					t.Errorf("expected DefaultReasoningEffort = medium, got %s", resp.Data[0].DefaultReasoningEffort)
				}
			},
		},
		{
			name: "paginated list with hidden models",
			params: codex.ModelListParams{
				Cursor:        ptr("cursor123"),
				IncludeHidden: ptr(true),
				Limit:         ptr(uint32(10)),
			},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"id":                        "claude-sonnet-4-5",
						"model":                     "claude-sonnet-4-5",
						"displayName":               "Claude Sonnet 4.5",
						"description":               "Balanced performance and speed",
						"hidden":                    false,
						"isDefault":                 false,
						"defaultReasoningEffort":    "low",
						"supportedReasoningEfforts": []interface{}{
							map[string]interface{}{
								"reasoningEffort": "none",
								"description":     "No reasoning",
							},
							map[string]interface{}{
								"reasoningEffort": "low",
								"description":     "Minimal reasoning",
							},
						},
						"inputModalities":     []interface{}{"text"},
						"supportsPersonality": true,
						"upgrade":             "claude-opus-4-6",
					},
					map[string]interface{}{
						"id":                        "gpt-4",
						"model":                     "gpt-4",
						"displayName":               "GPT-4",
						"description":               "OpenAI GPT-4",
						"hidden":                    true,
						"isDefault":                 false,
						"defaultReasoningEffort":    "medium",
						"supportedReasoningEfforts": []interface{}{},
						"inputModalities":           []interface{}{"text", "image"},
					},
				},
				"nextCursor": "cursor456",
			},
			checkResponse: func(t *testing.T, resp codex.ModelListResponse) {
				if len(resp.Data) != 2 {
					t.Fatalf("expected 2 models, got %d", len(resp.Data))
				}

				// Check first model
				model1 := resp.Data[0]
				if model1.ID != "claude-sonnet-4-5" {
					t.Errorf("expected ID = claude-sonnet-4-5, got %s", model1.ID)
				}
				if len(model1.SupportedReasoningEfforts) != 2 {
					t.Errorf("expected 2 reasoning efforts, got %d", len(model1.SupportedReasoningEfforts))
				}
				if model1.SupportedReasoningEfforts[0].ReasoningEffort != "none" {
					t.Errorf("expected first effort = none, got %s", model1.SupportedReasoningEfforts[0].ReasoningEffort)
				}
				if model1.SupportsPersonality != true {
					t.Errorf("expected SupportsPersonality = true, got %v", model1.SupportsPersonality)
				}
				if model1.Upgrade == nil || *model1.Upgrade != "claude-opus-4-6" {
					t.Errorf("expected Upgrade = claude-opus-4-6, got %v", model1.Upgrade)
				}

				// Check second model (hidden)
				model2 := resp.Data[1]
				if model2.Hidden != true {
					t.Errorf("expected Hidden = true, got %v", model2.Hidden)
				}

				// Check pagination cursor
				if resp.NextCursor == nil || *resp.NextCursor != "cursor456" {
					t.Errorf("expected NextCursor = cursor456, got %v", resp.NextCursor)
				}
			},
		},
		{
			name:   "empty list",
			params: codex.ModelListParams{},
			mockResponse: map[string]interface{}{
				"data": []interface{}{},
			},
			checkResponse: func(t *testing.T, resp codex.ModelListResponse) {
				if len(resp.Data) != 0 {
					t.Errorf("expected 0 models, got %d", len(resp.Data))
				}
				if resp.NextCursor != nil {
					t.Errorf("expected no NextCursor, got %v", resp.NextCursor)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("model/list", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Model.List(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "model/list" {
				t.Errorf("expected method model/list, got %v", req)
			}
		})
	}
}

func TestModelReroutedNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Set up notification listener
	var receivedNotif *codex.ModelReroutedNotification
	client.OnModelRerouted(func(notif codex.ModelReroutedNotification) {
		receivedNotif = &notif
	})

	// Inject notification from server
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "notification/model/rerouted",
		Params: json.RawMessage(`{
			"threadId": "thread-123",
			"turnId": "turn-456",
			"fromModel": "claude-opus-4-6",
			"toModel": "claude-sonnet-4-5",
			"reason": "highRiskCyberActivity"
		}`),
	}

	ctx := context.Background()
	mock.InjectServerNotification(ctx, notif)

	// Verify notification was received
	if receivedNotif == nil {
		t.Fatal("expected notification to be received")
	}
	if receivedNotif.ThreadID != "thread-123" {
		t.Errorf("expected ThreadID = thread-123, got %s", receivedNotif.ThreadID)
	}
	if receivedNotif.TurnID != "turn-456" {
		t.Errorf("expected TurnID = turn-456, got %s", receivedNotif.TurnID)
	}
	if receivedNotif.FromModel != "claude-opus-4-6" {
		t.Errorf("expected FromModel = claude-opus-4-6, got %s", receivedNotif.FromModel)
	}
	if receivedNotif.ToModel != "claude-sonnet-4-5" {
		t.Errorf("expected ToModel = claude-sonnet-4-5, got %s", receivedNotif.ToModel)
	}
	if receivedNotif.Reason != "highRiskCyberActivity" {
		t.Errorf("expected Reason = highRiskCyberActivity, got %s", receivedNotif.Reason)
	}
}

func TestModelServiceMethodSignatures(t *testing.T) {
	// Compile-time verification that ModelService has all required methods
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var _ interface {
		List(context.Context, codex.ModelListParams) (codex.ModelListResponse, error)
	} = client.Model
}
