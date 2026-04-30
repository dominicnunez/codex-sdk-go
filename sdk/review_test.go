package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/sdk"
)

func TestReviewStart(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.ReviewStartParams
		response map[string]interface{}
		wantJSON map[string]interface{}
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
			wantJSON: map[string]interface{}{
				"threadId": "thread-123",
				"target": map[string]interface{}{
					"type": "uncommittedChanges",
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
			wantJSON: map[string]interface{}{
				"threadId": "thread-123",
				"target": map[string]interface{}{
					"type":   "baseBranch",
					"branch": "main",
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
			wantJSON: map[string]interface{}{
				"threadId": "thread-123",
				"target": map[string]interface{}{
					"type":  "commit",
					"sha":   "abc123def456",
					"title": "feat: add new feature",
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
			wantJSON: map[string]interface{}{
				"threadId": "thread-123",
				"target": map[string]interface{}{
					"type":         "custom",
					"instructions": "Review for security vulnerabilities",
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
			wantJSON: map[string]interface{}{
				"threadId": "thread-123",
				"delivery": "detached",
				"target": map[string]interface{}{
					"type": "uncommittedChanges",
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
				return
			}

			if req.Method != "review/start" {
				t.Errorf("Method = %v, want review/start", req.Method)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(req.Params, &got); err != nil {
				t.Fatalf("request params decode failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantJSON) {
				t.Errorf("request params = %#v, want %#v", got, tt.wantJSON)
			}
		})
	}
}

func TestReviewTargetWrapperUnmarshalRejectsMissingVariantFields(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "base branch missing branch",
			data: `{"type":"baseBranch"}`,
		},
		{
			name: "commit missing sha",
			data: `{"type":"commit"}`,
		},
		{
			name: "custom missing instructions",
			data: `{"type":"custom"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wrapper codex.ReviewTargetWrapper
			err := json.Unmarshal([]byte(tt.data), &wrapper)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, codex.ErrMissingResultField) {
				t.Fatalf("error = %v; want ErrMissingResultField", err)
			}
		})
	}
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

func TestReviewStartRejectsInvalidParamsBeforeSending(t *testing.T) {
	tests := []struct {
		name    string
		params  codex.ReviewStartParams
		wantErr string
	}{
		{
			name: "empty thread id",
			params: codex.ReviewStartParams{
				ThreadID: "",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.UncommittedChangesReviewTarget{},
				},
			},
			wantErr: "threadId must not be empty",
		},
		{
			name: "nil target value",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
			},
			wantErr: "target must not be null",
		},
		{
			name: "typed nil target value",
			params: func() codex.ReviewStartParams {
				var target *codex.UncommittedChangesReviewTarget
				return codex.ReviewStartParams{
					ThreadID: "thread-123",
					Target: codex.ReviewTargetWrapper{
						Value: target,
					},
				}
			}(),
			wantErr: "target must not be null",
		},
		{
			name: "empty base branch",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.BaseBranchReviewTarget{},
				},
			},
			wantErr: "target.branch must not be empty",
		},
		{
			name: "empty commit sha",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.CommitReviewTarget{},
				},
			},
			wantErr: "target.sha must not be empty",
		},
		{
			name: "empty custom instructions",
			params: codex.ReviewStartParams{
				ThreadID: "thread-123",
				Target: codex.ReviewTargetWrapper{
					Value: &codex.CustomReviewTarget{},
				},
			},
			wantErr: "target.instructions must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			_, err := client.Review.Start(context.Background(), tt.params)
			if err == nil {
				t.Fatal("expected invalid params error")
			}
			if !strings.Contains(err.Error(), "invalid params") {
				t.Fatalf("error = %v, want invalid params context", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
			if got := mock.CallCount(); got != 0 {
				t.Fatalf("transport recorded %d requests, want 0", got)
			}
		})
	}
}
