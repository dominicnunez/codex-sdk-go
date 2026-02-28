package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

// TestInitializeParamsSerialization verifies that InitializeParams serializes correctly
// matching the specs/v1/InitializeParams.json schema.
func TestInitializeParamsSerialization(t *testing.T) {
	tests := []struct {
		name     string
		params   codex.InitializeParams
		expected string
	}{
		{
			name: "minimal params with required fields only",
			params: codex.InitializeParams{
				ClientInfo: codex.ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
				},
			},
			expected: `{"clientInfo":{"name":"test-client","version":"1.0.0"}}`,
		},
		{
			name: "params with optional title",
			params: codex.InitializeParams{
				ClientInfo: codex.ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
					Title:   strPtr("Test Client Title"),
				},
			},
			expected: `{"clientInfo":{"name":"test-client","version":"1.0.0","title":"Test Client Title"}}`,
		},
		{
			name: "params with capabilities",
			params: codex.InitializeParams{
				ClientInfo: codex.ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
				},
				Capabilities: &codex.InitializeCapabilities{
					ExperimentalAPI: true,
				},
			},
			expected: `{"clientInfo":{"name":"test-client","version":"1.0.0"},"capabilities":{"experimentalApi":true}}`,
		},
		{
			name: "params with optOutNotificationMethods",
			params: codex.InitializeParams{
				ClientInfo: codex.ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
				},
				Capabilities: &codex.InitializeCapabilities{
					OptOutNotificationMethods: []string{"codex/event/session_configured", "codex/event/test"},
				},
			},
			expected: `{"clientInfo":{"name":"test-client","version":"1.0.0"},"capabilities":{"experimentalApi":false,"optOutNotificationMethods":["codex/event/session_configured","codex/event/test"]}}`,
		},
		{
			name: "params with all fields",
			params: codex.InitializeParams{
				ClientInfo: codex.ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
					Title:   strPtr("Test Client Title"),
				},
				Capabilities: &codex.InitializeCapabilities{
					ExperimentalAPI:           true,
					OptOutNotificationMethods: []string{"codex/event/test"},
				},
			},
			expected: `{"clientInfo":{"name":"test-client","version":"1.0.0","title":"Test Client Title"},"capabilities":{"experimentalApi":true,"optOutNotificationMethods":["codex/event/test"]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Compare as JSON objects to ignore field order
			var got, want map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal result failed: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &want); err != nil {
				t.Fatalf("Unmarshal expected failed: %v", err)
			}

			if !jsonEqual(got, want) {
				t.Errorf("InitializeParams serialization mismatch:\ngot:  %s\nwant: %s", string(data), tt.expected)
			}
		})
	}
}

// TestInitializeResponseDeserialization verifies that InitializeResponse deserializes correctly
// matching the specs/v1/InitializeResponse.json schema.
func TestInitializeResponseDeserialization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected codex.InitializeResponse
		wantErr  bool
	}{
		{
			name:  "valid response with userAgent",
			input: `{"userAgent":"codex-server/1.0.0"}`,
			expected: codex.InitializeResponse{
				UserAgent: "codex-server/1.0.0",
			},
		},
		{
			name:    "missing required userAgent field",
			input:   `{}`,
			wantErr: false, // JSON unmarshal succeeds, just empty string
			expected: codex.InitializeResponse{
				UserAgent: "",
			},
		},
		{
			name:  "response with extra fields (forward compatibility)",
			input: `{"userAgent":"codex-server/1.0.0","extra":"field"}`,
			expected: codex.InitializeResponse{
				UserAgent: "codex-server/1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp codex.InitializeResponse
			err := json.Unmarshal([]byte(tt.input), &resp)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}

			if resp.UserAgent != tt.expected.UserAgent {
				t.Errorf("UserAgent = %q, want %q", resp.UserAgent, tt.expected.UserAgent)
			}
		})
	}
}

// TestClientInitialize verifies the Client.Initialize method round-trip using MockTransport.
func TestClientInitialize(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Set up expected response
	responseData := codex.InitializeResponse{
		UserAgent: "codex-server/1.0.0",
	}
	responseJSON, _ := json.Marshal(responseData)
	mock.SetResponse("initialize", codex.Response{
		JSONRPC: "2.0",
		Result:  json.RawMessage(responseJSON),
	})

	// Call Initialize
	params := codex.InitializeParams{
		ClientInfo: codex.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
			Title:   strPtr("Test Client"),
		},
		Capabilities: &codex.InitializeCapabilities{
			ExperimentalAPI: true,
		},
	}

	ctx := context.Background()
	resp, err := client.Initialize(ctx, params)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify response
	if resp.UserAgent != "codex-server/1.0.0" {
		t.Errorf("UserAgent = %q, want %q", resp.UserAgent, "codex-server/1.0.0")
	}

	// Verify the correct method name was sent
	sentReq := mock.GetSentRequest(0)
	if sentReq == nil {
		t.Fatal("no request was sent")
	}
	if sentReq.Method != "initialize" {
		t.Errorf("request method = %q, want %q", sentReq.Method, "initialize")
	}

	// Verify params were serialized correctly
	var sentParams codex.InitializeParams
	if err := json.Unmarshal(sentReq.Params, &sentParams); err != nil {
		t.Fatalf("failed to unmarshal sent params: %v", err)
	}
	if sentParams.ClientInfo.Name != "test-client" {
		t.Errorf("ClientInfo.Name = %q, want %q", sentParams.ClientInfo.Name, "test-client")
	}
	if sentParams.ClientInfo.Version != "1.0.0" {
		t.Errorf("ClientInfo.Version = %q, want %q", sentParams.ClientInfo.Version, "1.0.0")
	}
	if sentParams.ClientInfo.Title == nil || *sentParams.ClientInfo.Title != "Test Client" {
		t.Errorf("ClientInfo.Title = %v, want %q", sentParams.ClientInfo.Title, "Test Client")
	}
	if sentParams.Capabilities == nil || !sentParams.Capabilities.ExperimentalAPI {
		t.Error("Capabilities.ExperimentalAPI should be true")
	}
}

// TestClientInitializeError verifies error handling in Initialize.
func TestClientInitializeError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Set up error response
	mock.SetResponse("initialize", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    -32600,
			Message: "Invalid request",
		},
	})

	params := codex.InitializeParams{
		ClientInfo: codex.ClientInfo{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}

	ctx := context.Background()
	_, err := client.Initialize(ctx, params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify it's an RPCError
	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected RPCError, got %T", err)
	}
	if rpcErr.Code() != -32600 {
		t.Errorf("error code = %d, want %d", rpcErr.Code(), -32600)
	}
}

// Helper function to create string pointers for optional fields
func strPtr(s string) *string {
	return &s
}

// jsonEqual compares two JSON objects for deep equality
func jsonEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
