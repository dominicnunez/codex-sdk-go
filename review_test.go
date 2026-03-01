package codex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestReviewStart(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.ReviewStartParams
		response map[string]interface{}
	}{
		{
			name: "uncommitted changes review",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.UncommittedChangesReviewTarget{},
				},
			},
			response: map[string]interface{}{
				"reviewThreadId": "thread-123",
				"turn": map[string]interface{}{
					"id":     "turn-456",
					"status": "inProgress",
					"items":  []interface{}{},
				},
			},
		},
		{
			name: "base branch review",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.BaseBranchReviewTarget{
						Branch: "main",
					},
				},
			},
			response: map[string]interface{}{
				"reviewThreadId": "thread-123",
				"turn": map[string]interface{}{
					"id":     "turn-789",
					"status": "completed",
					"items":  []interface{}{},
				},
			},
		},
		{
			name: "commit review",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.CommitReviewTarget{
						SHA:   "abc123def456",
						Title: ptr("feat: add new feature"),
					},
				},
			},
			response: map[string]interface{}{
				"reviewThreadId": "thread-123",
				"turn": map[string]interface{}{
					"id":     "turn-101",
					"status": "completed",
					"items":  []interface{}{},
				},
			},
		},
		{
			name: "custom review",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.CustomReviewTarget{
						Instructions: "Review for security vulnerabilities",
					},
				},
			},
			response: map[string]interface{}{
				"reviewThreadId": "thread-123",
				"turn": map[string]interface{}{
					"id":     "turn-202",
					"status": "inProgress",
					"items":  []interface{}{},
				},
			},
		},
		{
			name: "detached review",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.UncommittedChangesReviewTarget{},
				},
				Delivery: ptr(codex.ReviewDeliveryDetached),
			},
			response: map[string]interface{}{
				"reviewThreadId": "review-thread-999",
				"turn": map[string]interface{}{
					"id":     "turn-303",
					"status": "inProgress",
					"items":  []interface{}{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			ctx := context.Background()

			_ = mock.SetResponseData("review/start", tt.response)

			resp, err := client.Review.Start(ctx, tt.params)
			if err != nil {
				t.Fatalf("Review.Start returned error: %v", err)
			}

			if resp.ReviewThreadID != tt.response["reviewThreadId"].(string) {
				t.Errorf("ReviewThreadID = %v, want %v", resp.ReviewThreadID, tt.response["reviewThreadId"])
			}

			turnMap := tt.response["turn"].(map[string]interface{})
			if resp.Turn.ID != turnMap["id"].(string) {
				t.Errorf("Turn.ID = %v, want %v", resp.Turn.ID, turnMap["id"])
			}

			req := mock.GetSentRequest(0)
			if req == nil {
				t.Fatal("No request was sent")
			}

			if req.Method != "review/start" {
				t.Errorf("Method = %v, want review/start", req.Method)
			}
		})
	}
}

func TestReviewServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Verify ReviewService exists and has Start method
	_ = client.Review.Start
}

func TestReviewStart_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("review/start", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "review engine unavailable",
		},
	})

	_, err := client.Review.Start(context.Background(), codex.ReviewStartParams{
		ThreadID: "thread-err",
		Target: codex.ReviewTargetWrapper{
			Value: &codex.UncommittedChangesReviewTarget{},
		},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected error to unwrap to *RPCError, got %T", err)
	}
	if rpcErr.RPCError().Code != codex.ErrCodeInternalError {
		t.Errorf("expected error code %d, got %d", codex.ErrCodeInternalError, rpcErr.RPCError().Code)
	}
}
