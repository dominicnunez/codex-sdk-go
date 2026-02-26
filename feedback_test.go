package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestFeedbackUpload(t *testing.T) {
	tests := []struct {
		name   string
		params codex.FeedbackUploadParams
		want   map[string]interface{}
	}{
		{
			name: "minimal params",
			params: codex.FeedbackUploadParams{
				Classification: "bug",
				IncludeLogs:    true,
			},
			want: map[string]interface{}{
				"classification": "bug",
				"includeLogs":    true,
			},
		},
		{
			name: "full params",
			params: codex.FeedbackUploadParams{
				Classification: "feature-request",
				IncludeLogs:    false,
				Reason:         ptr("The search is too slow"),
				ThreadID:       ptr("thread-123"),
				ExtraLogFiles:  &[]string{"/var/log/app.log", "/var/log/error.log"},
			},
			want: map[string]interface{}{
				"classification": "feature-request",
				"includeLogs":    false,
				"reason":         "The search is too slow",
				"threadId":       "thread-123",
				"extraLogFiles":  []interface{}{"/var/log/app.log", "/var/log/error.log"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)

			mock.SetResponseData("feedback/upload", map[string]interface{}{
				"threadId": "thread-456",
			})

			resp, err := client.Feedback.Upload(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.ThreadID != "thread-456" {
				t.Errorf("got threadId %q, want %q", resp.ThreadID, "thread-456")
			}

			// Verify params were serialized correctly
			req := mock.GetSentRequest(0)
			if req == nil {
				t.Fatal("no request sent")
			}
			if req.Method != "feedback/upload" {
				t.Errorf("got method %q, want %q", req.Method, "feedback/upload")
			}

			var gotParams map[string]interface{}
			if err := json.Unmarshal(req.Params, &gotParams); err != nil {
				t.Fatalf("failed to unmarshal params: %v", err)
			}

			wantJSON, _ := json.Marshal(tt.want)
			gotJSON, _ := json.Marshal(gotParams)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("params mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestFeedbackServiceMethodSignatures(t *testing.T) {
	// Compile-time verification that FeedbackService has the expected methods
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var _ interface {
		Upload(context.Context, codex.FeedbackUploadParams) (codex.FeedbackUploadResponse, error)
	} = client.Feedback
}
