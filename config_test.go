package codex_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestConfigRead(t *testing.T) {
	tests := []struct {
		name           string
		params         codex.ConfigReadParams
		mockResponse   map[string]interface{}
		checkResponse  func(t *testing.T, resp codex.ConfigReadResponse)
	}{
		{
			name: "minimal read",
			params: codex.ConfigReadParams{},
			mockResponse: map[string]interface{}{
				"config": map[string]interface{}{},
				"origins": map[string]interface{}{},
			},
			checkResponse: func(t *testing.T, resp codex.ConfigReadResponse) {
				if resp.Config == nil {
					t.Error("expected Config to be non-nil")
				}
				if resp.Origins == nil {
					t.Error("expected Origins to be non-nil")
				}
			},
		},
		{
			name: "read with cwd and layers",
			params: codex.ConfigReadParams{
				Cwd:           ptr("/home/user/project"),
				IncludeLayers: ptr(true),
			},
			mockResponse: map[string]interface{}{
				"config": map[string]interface{}{
					"model": "claude-4.5",
					"approval_policy": "on-request",
				},
				"layers": []interface{}{
					map[string]interface{}{
						"name":    map[string]interface{}{"type": "user", "file": "/home/user/.claude/config.toml"},
						"version": "v1",
						"config":  map[string]interface{}{},
					},
				},
				"origins": map[string]interface{}{
					"user": map[string]interface{}{
						"name":    map[string]interface{}{"type": "user", "file": "/home/user/.claude/config.toml"},
						"version": "v1",
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.ConfigReadResponse) {
				if resp.Config == nil {
					t.Fatal("expected Config to be non-nil")
				}
				if resp.Config.Model == nil || *resp.Config.Model != "claude-4.5" {
					t.Errorf("expected Model = claude-4.5, got %v", resp.Config.Model)
				}
				if resp.Layers == nil || len(*resp.Layers) != 1 {
					t.Errorf("expected 1 layer, got %v", resp.Layers)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponseData("config/read", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Config.Read(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "config/read" {
				t.Errorf("expected method config/read, got %v", req)
			}
		})
	}
}

func TestConfigReadRequirements(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.ConfigRequirementsReadResponse)
	}{
		{
			name: "no requirements",
			mockResponse: map[string]interface{}{
				"requirements": nil,
			},
			checkResponse: func(t *testing.T, resp codex.ConfigRequirementsReadResponse) {
				if resp.Requirements != nil {
					t.Error("expected Requirements to be nil")
				}
			},
		},
		{
			name: "with requirements",
			mockResponse: map[string]interface{}{
				"requirements": map[string]interface{}{
					"allowedApprovalPolicies": []interface{}{"on-request", "never"},
					"allowedSandboxModes":     []interface{}{"read-only", "workspace-write"},
					"allowedWebSearchModes":   []interface{}{"cached"},
					"enforceResidency":        "us",
				},
			},
			checkResponse: func(t *testing.T, resp codex.ConfigRequirementsReadResponse) {
				if resp.Requirements == nil {
					t.Fatal("expected Requirements to be non-nil")
				}
				if resp.Requirements.AllowedApprovalPolicies == nil || len(*resp.Requirements.AllowedApprovalPolicies) != 2 {
					t.Errorf("expected 2 approval policies, got %v", resp.Requirements.AllowedApprovalPolicies)
				}
				if resp.Requirements.EnforceResidency == nil || *resp.Requirements.EnforceResidency != "us" {
					t.Errorf("expected enforceResidency = us, got %v", resp.Requirements.EnforceResidency)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponseData("config/requirements/read", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Config.ReadRequirements(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "config/requirements/read" {
				t.Errorf("expected method config/requirements/read, got %v", req)
			}
		})
	}
}

func TestConfigWrite(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.ConfigValueWriteParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.ConfigWriteResponse)
	}{
		{
			name: "simple write",
			params: codex.ConfigValueWriteParams{
				KeyPath:       "model",
				MergeStrategy: "replace",
				Value:         json.RawMessage(`"claude-4.5"`),
			},
			mockResponse: map[string]interface{}{
				"filePath": "/home/user/.claude/config.toml",
				"status":   "ok",
				"version":  "v2",
			},
			checkResponse: func(t *testing.T, resp codex.ConfigWriteResponse) {
				if resp.FilePath != "/home/user/.claude/config.toml" {
					t.Errorf("expected FilePath = /home/user/.claude/config.toml, got %s", resp.FilePath)
				}
				if resp.Status != "ok" {
					t.Errorf("expected Status = ok, got %s", resp.Status)
				}
				if resp.Version != "v2" {
					t.Errorf("expected Version = v2, got %s", resp.Version)
				}
			},
		},
		{
			name: "write with override",
			params: codex.ConfigValueWriteParams{
				KeyPath:       "approval_policy",
				MergeStrategy: "replace",
				Value:         json.RawMessage(`"never"`),
				FilePath:      ptr("/home/user/project/.codex/config.toml"),
			},
			mockResponse: map[string]interface{}{
				"filePath": "/home/user/project/.codex/config.toml",
				"status":   "okOverridden",
				"version":  "v3",
				"overriddenMetadata": map[string]interface{}{
					"effectiveValue":   "on-request",
					"message":          "Value overridden by system policy",
					"overridingLayer": map[string]interface{}{
						"name":    map[string]interface{}{"type": "system", "file": "/etc/claude/config.toml"},
						"version": "v1",
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.ConfigWriteResponse) {
				if resp.Status != "okOverridden" {
					t.Errorf("expected Status = okOverridden, got %s", resp.Status)
				}
				if resp.OverriddenMetadata == nil {
					t.Fatal("expected OverriddenMetadata to be non-nil")
				}
				if resp.OverriddenMetadata.Message != "Value overridden by system policy" {
					t.Errorf("expected override message, got %s", resp.OverriddenMetadata.Message)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponseData("config/value/write", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Config.Write(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "config/value/write" {
				t.Errorf("expected method config/value/write, got %v", req)
			}
		})
	}
}

func TestConfigBatchWrite(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.ConfigBatchWriteParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.ConfigWriteResponse)
	}{
		{
			name: "batch write multiple keys",
			params: codex.ConfigBatchWriteParams{
				Edits: []codex.ConfigEdit{
					{
						KeyPath:       "model",
						MergeStrategy: "replace",
						Value:         json.RawMessage(`"claude-4.5"`),
					},
					{
						KeyPath:       "tools.web_search",
						MergeStrategy: "replace",
						Value:         json.RawMessage(`true`),
					},
				},
			},
			mockResponse: map[string]interface{}{
				"filePath": "/home/user/.claude/config.toml",
				"status":   "ok",
				"version":  "v4",
			},
			checkResponse: func(t *testing.T, resp codex.ConfigWriteResponse) {
				if resp.Status != "ok" {
					t.Errorf("expected Status = ok, got %s", resp.Status)
				}
				if resp.Version != "v4" {
					t.Errorf("expected Version = v4, got %s", resp.Version)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponseData("config/batch/write", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Config.BatchWrite(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "config/batch/write" {
				t.Errorf("expected method config/batch/write, got %v", req)
			}
		})
	}
}

func TestConfigWarningNotification(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Set up notification listener
	var receivedNotif *codex.ConfigWarningNotification
	client.OnConfigWarning(func(notif codex.ConfigWarningNotification) {
		receivedNotif = &notif
	})

	// Inject notification from server
	notif := codex.Notification{
		JSONRPC: "2.0",
		Method:  "notification/config/warning",
		Params:  json.RawMessage(`{"summary": "Invalid config value", "details": "The value 'foo' is not valid for key 'model'", "path": "/home/user/.claude/config.toml"}`),
	}

	ctx := context.Background()
	mock.InjectServerNotification(ctx, notif)

	// Verify notification was received
	if receivedNotif == nil {
		t.Fatal("expected notification to be received")
	}
	if receivedNotif.Summary != "Invalid config value" {
		t.Errorf("expected Summary = 'Invalid config value', got %s", receivedNotif.Summary)
	}
	if receivedNotif.Details == nil || *receivedNotif.Details != "The value 'foo' is not valid for key 'model'" {
		t.Errorf("expected Details with error message, got %v", receivedNotif.Details)
	}
	if receivedNotif.Path == nil || *receivedNotif.Path != "/home/user/.claude/config.toml" {
		t.Errorf("expected Path to config file, got %v", receivedNotif.Path)
	}
}

func TestConfigServiceMethodSignatures(t *testing.T) {
	// Compile-time verification that ConfigService has all required methods
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	var _ interface {
		Read(context.Context, codex.ConfigReadParams) (codex.ConfigReadResponse, error)
		ReadRequirements(context.Context) (codex.ConfigRequirementsReadResponse, error)
		Write(context.Context, codex.ConfigValueWriteParams) (codex.ConfigWriteResponse, error)
		BatchWrite(context.Context, codex.ConfigBatchWriteParams) (codex.ConfigWriteResponse, error)
	} = client.Config
}
