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

// TestFuzzyFileSearchServiceSearch tests that FuzzyFileSearchService.Search sends the correct
// request method and deserializes the response.
func TestFuzzyFileSearchServiceSearch(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.FuzzyFileSearchParams
		response map[string]interface{}
		wantLen  int
	}{
		{
			name: "single result",
			params: codex.FuzzyFileSearchParams{
				Query: "main.go",
				Roots: []string{"/home/user/project"},
			},
			response: map[string]interface{}{
				"files": []interface{}{
					map[string]interface{}{
						"path":      "/home/user/project/main.go",
						"file_name": "main.go",
						"root":      "/home/user/project",
						"score":     float64(100),
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "multiple results with indices",
			params: codex.FuzzyFileSearchParams{
				Query:             "test",
				Roots:             []string{"/project"},
				CancellationToken: ptr("cancel-tok-1"),
			},
			response: map[string]interface{}{
				"files": []interface{}{
					map[string]interface{}{
						"path":      "/project/test_a.go",
						"file_name": "test_a.go",
						"root":      "/project",
						"score":     float64(95),
						"indices":   []interface{}{float64(0), float64(1), float64(2), float64(3)},
					},
					map[string]interface{}{
						"path":      "/project/test_b.go",
						"file_name": "test_b.go",
						"root":      "/project",
						"score":     float64(80),
					},
				},
			},
			wantLen: 2,
		},
		{
			name: "empty results",
			params: codex.FuzzyFileSearchParams{
				Query: "nonexistent",
				Roots: []string{"/project"},
			},
			response: map[string]interface{}{
				"files": []interface{}{},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			ctx := context.Background()

			_ = mock.SetResponseData("fuzzyFileSearch", tt.response)

			resp, err := client.FuzzyFileSearch.Search(ctx, tt.params)
			if err != nil {
				t.Fatalf("FuzzyFileSearch.Search returned error: %v", err)
			}

			if len(resp.Files) != tt.wantLen {
				t.Fatalf("Files length = %d, want %d", len(resp.Files), tt.wantLen)
			}

			// Verify response content for non-empty results
			if tt.wantLen > 0 {
				filesSlice := tt.response["files"].([]interface{})
				firstFile := filesSlice[0].(map[string]interface{})

				if resp.Files[0].Path != firstFile["path"].(string) {
					t.Errorf("Files[0].Path = %q, want %q", resp.Files[0].Path, firstFile["path"])
				}
				if resp.Files[0].FileName != firstFile["file_name"].(string) {
					t.Errorf("Files[0].FileName = %q, want %q", resp.Files[0].FileName, firstFile["file_name"])
				}
				if resp.Files[0].Root != firstFile["root"].(string) {
					t.Errorf("Files[0].Root = %q, want %q", resp.Files[0].Root, firstFile["root"])
				}
				if resp.Files[0].Score != uint32(firstFile["score"].(float64)) {
					t.Errorf("Files[0].Score = %d, want %d", resp.Files[0].Score, uint32(firstFile["score"].(float64)))
				}
			}

			// Verify the sent request method
			req := mock.GetSentRequest(0)
			if req == nil {
				t.Fatal("No request was sent")
			}

			if req.Method != "fuzzyFileSearch" {
				t.Errorf("Method = %q, want %q", req.Method, "fuzzyFileSearch")
			}
		})
	}
}
