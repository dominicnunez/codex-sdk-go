package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestFuzzyFileSearchParamsResponse tests FuzzyFileSearch params/response round-trip.
// FuzzyFileSearch is a server→client request, so params/response are already defined in approval.go.
func TestFuzzyFileSearchParamsResponse(t *testing.T) {
	tests := []struct {
		name   string
		params codex.FuzzyFileSearchParams
		resp   codex.FuzzyFileSearchResponse
	}{
		{
			name: "minimal search",
			params: codex.FuzzyFileSearchParams{
				Query: "main.go",
				Roots: []string{"/home/user/project"},
			},
			resp: codex.FuzzyFileSearchResponse{
				Files: []codex.FuzzyFileSearchResult{
					{
						Path:     "/home/user/project/main.go",
						FileName: "main.go",
						Root:     "/home/user/project",
						Score:    100,
					},
				},
			},
		},
		{
			name: "search with indices",
			params: codex.FuzzyFileSearchParams{
				Query:             "test",
				Roots:             []string{"/home/user/project"},
				CancellationToken: ptr("token123"),
			},
			resp: codex.FuzzyFileSearchResponse{
				Files: []codex.FuzzyFileSearchResult{
					{
						Path:     "/home/user/project/test_file.go",
						FileName: "test_file.go",
						Root:     "/home/user/project",
						Score:    95,
						Indices:  &[]uint32{0, 1, 2, 3},
					},
				},
			},
		},
		{
			name: "empty results",
			params: codex.FuzzyFileSearchParams{
				Query: "nonexistent",
				Roots: []string{"/home/user/project"},
			},
			resp: codex.FuzzyFileSearchResponse{
				Files: []codex.FuzzyFileSearchResult{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test params serialization
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("failed to marshal params: %v", err)
			}

			var params2 codex.FuzzyFileSearchParams
			if err := json.Unmarshal(paramsJSON, &params2); err != nil {
				t.Fatalf("failed to unmarshal params: %v", err)
			}

			if params2.Query != tt.params.Query {
				t.Errorf("Query mismatch: got %q, want %q", params2.Query, tt.params.Query)
			}

			// Test response serialization
			respJSON, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("failed to marshal response: %v", err)
			}

			var resp2 codex.FuzzyFileSearchResponse
			if err := json.Unmarshal(respJSON, &resp2); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if len(resp2.Files) != len(tt.resp.Files) {
				t.Errorf("Files length mismatch: got %d, want %d", len(resp2.Files), len(tt.resp.Files))
			}
		})
	}
}

// TestFuzzyFileSearchSessionCompletedNotification tests the sessionCompleted notification dispatch.
func TestFuzzyFileSearchSessionCompletedNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	notificationReceived := false
	var receivedSessionID string

	client.OnFuzzyFileSearchSessionCompleted(func(notif codex.FuzzyFileSearchSessionCompletedNotification) {
		notificationReceived = true
		receivedSessionID = notif.SessionID
	})

	// Inject notification from server
	notifParams := map[string]interface{}{
		"sessionId": "session-123",
	}
	paramsJSON, _ := json.Marshal(notifParams)

	ctx := context.Background()
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "fuzzyFileSearch/sessionCompleted",
		Params:  paramsJSON,
	})

	if !notificationReceived {
		t.Error("notification listener was not called")
	}

	if receivedSessionID != "session-123" {
		t.Errorf("SessionID mismatch: got %q, want %q", receivedSessionID, "session-123")
	}
}

// TestFuzzyFileSearchSessionUpdatedNotification tests the sessionUpdated notification dispatch.
func TestFuzzyFileSearchSessionUpdatedNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	notificationReceived := false
	var receivedNotif codex.FuzzyFileSearchSessionUpdatedNotification

	client.OnFuzzyFileSearchSessionUpdated(func(notif codex.FuzzyFileSearchSessionUpdatedNotification) {
		notificationReceived = true
		receivedNotif = notif
	})

	// Inject notification from server
	notifParams := map[string]interface{}{
		"sessionId": "session-456",
		"query":     "main",
		"files": []interface{}{
			map[string]interface{}{
				"path":      "/project/main.go",
				"file_name": "main.go",
				"root":      "/project",
				"score":     float64(100), // JSON numbers are float64
			},
			map[string]interface{}{
				"path":      "/project/cmd/main.go",
				"file_name": "main.go",
				"root":      "/project",
				"score":     float64(85),
				"indices":   []interface{}{float64(0), float64(1), float64(2), float64(3)},
			},
		},
	}
	paramsJSON, _ := json.Marshal(notifParams)

	ctx := context.Background()
	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "fuzzyFileSearch/sessionUpdated",
		Params:  paramsJSON,
	})

	if !notificationReceived {
		t.Error("notification listener was not called")
	}

	if receivedNotif.SessionID != "session-456" {
		t.Errorf("SessionID mismatch: got %q, want %q", receivedNotif.SessionID, "session-456")
	}

	if receivedNotif.Query != "main" {
		t.Errorf("Query mismatch: got %q, want %q", receivedNotif.Query, "main")
	}

	if len(receivedNotif.Files) != 2 {
		t.Errorf("Files length mismatch: got %d, want 2", len(receivedNotif.Files))
	}

	if len(receivedNotif.Files) >= 1 {
		file := receivedNotif.Files[0]
		if file.Path != "/project/main.go" {
			t.Errorf("File[0].Path mismatch: got %q, want %q", file.Path, "/project/main.go")
		}
		if file.Score != 100 {
			t.Errorf("File[0].Score mismatch: got %d, want 100", file.Score)
		}
	}

	if len(receivedNotif.Files) >= 2 {
		file := receivedNotif.Files[1]
		if file.Indices == nil {
			t.Error("File[1].Indices should not be nil")
		} else if len(*file.Indices) != 4 {
			t.Errorf("File[1].Indices length mismatch: got %d, want 4", len(*file.Indices))
		}
	}
}
