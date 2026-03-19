package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestConfigRead(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.ConfigReadParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.ConfigReadResponse)
	}{
		{
			name:   "minimal read",
			params: codex.ConfigReadParams{},
			mockResponse: map[string]interface{}{
				"config":  map[string]interface{}{},
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
					"model":           "claude-4.5",
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
			_ = mock.SetResponseData("config/read", tt.mockResponse)
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

func TestConfigReadRejectsMalformedApprovalPolicy(t *testing.T) {
	tests := []struct {
		name        string
		approval    interface{}
		wantErr     error
		wantContain string
	}{
		{
			name:        "approval policy object requires discriminator",
			approval:    map[string]interface{}{},
			wantContain: "approval policy: missing discriminator",
		},
		{
			name: "granular approval policy requires mandatory fields",
			approval: map[string]interface{}{
				"granular": map[string]interface{}{},
			},
			wantErr: codex.ErrMissingResultField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("config/read", map[string]interface{}{
				"config": map[string]interface{}{
					"approval_policy": tt.approval,
				},
				"origins": map[string]interface{}{},
			})
			client := codex.NewClient(mock)

			_, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantContain != "" && !strings.Contains(err.Error(), tt.wantContain) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantContain)
			}
		})
	}
}

func TestConfigReadRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr string
	}{
		{
			name: "forced login method",
			config: map[string]interface{}{
				"forced_login_method": "totally-invalid",
			},
			wantErr: `invalid forcedLoginMethod "totally-invalid"`,
		},
		{
			name: "model verbosity",
			config: map[string]interface{}{
				"model_verbosity": "totally-invalid",
			},
			wantErr: `invalid verbosity "totally-invalid"`,
		},
		{
			name: "sandbox mode",
			config: map[string]interface{}{
				"sandbox_mode": "totally-invalid",
			},
			wantErr: `invalid sandboxMode "totally-invalid"`,
		},
		{
			name: "web search mode",
			config: map[string]interface{}{
				"web_search": "totally-invalid",
			},
			wantErr: `invalid webSearchMode "totally-invalid"`,
		},
		{
			name: "nested profile verbosity",
			config: map[string]interface{}{
				"profiles": map[string]interface{}{
					"default": map[string]interface{}{
						"model_verbosity": "totally-invalid",
					},
				},
			},
			wantErr: `invalid verbosity "totally-invalid"`,
		},
		{
			name: "nested profile web search",
			config: map[string]interface{}{
				"profiles": map[string]interface{}{
					"default": map[string]interface{}{
						"web_search": "totally-invalid",
					},
				},
			},
			wantErr: `invalid webSearchMode "totally-invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("config/read", map[string]interface{}{
				"config":  tt.config,
				"origins": map[string]interface{}{},
			})
			client := codex.NewClient(mock)

			_, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
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
					"featureRequirements": map[string]interface{}{
						"threads": true,
					},
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
				if !resp.Requirements.FeatureRequirements["threads"] {
					t.Errorf("expected featureRequirements[threads] = true, got %v", resp.Requirements.FeatureRequirements["threads"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("configRequirements/read", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Config.ReadRequirements(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "configRequirements/read" {
				t.Errorf("expected method configRequirements/read, got %v", req)
			}
		})
	}
}

func TestConfigReadRequirementsRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name         string
		requirements map[string]interface{}
		wantErr      string
	}{
		{
			name: "sandbox mode",
			requirements: map[string]interface{}{
				"allowedSandboxModes": []interface{}{"totally-invalid"},
			},
			wantErr: `invalid sandboxMode "totally-invalid"`,
		},
		{
			name: "web search mode",
			requirements: map[string]interface{}{
				"allowedWebSearchModes": []interface{}{"totally-invalid"},
			},
			wantErr: `invalid webSearchMode "totally-invalid"`,
		},
		{
			name: "residency requirement",
			requirements: map[string]interface{}{
				"enforceResidency": "totally-invalid",
			},
			wantErr: `invalid residencyRequirement "totally-invalid"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("configRequirements/read", map[string]interface{}{
				"requirements": tt.requirements,
			})
			client := codex.NewClient(mock)

			_, err := client.Config.ReadRequirements(context.Background())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
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
					"effectiveValue": "on-request",
					"message":        "Value overridden by system policy",
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
			_ = mock.SetResponseData("config/value/write", tt.mockResponse)
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
			_ = mock.SetResponseData("config/batchWrite", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Config.BatchWrite(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify method name
			req := mock.GetSentRequest(0)
			if req == nil || req.Method != "config/batchWrite" {
				t.Errorf("expected method config/batchWrite, got %v", req)
			}
		})
	}
}

func TestConfigWriteRejectsInvalidStatus(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{name: "value write", method: "config/value/write"},
		{name: "batch write", method: "config/batchWrite"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData(tt.method, map[string]interface{}{
				"filePath": "/home/user/.claude/config.toml",
				"status":   "stale",
				"version":  "v1",
			})

			client := codex.NewClient(mock)

			var err error
			switch tt.method {
			case "config/value/write":
				_, err = client.Config.Write(context.Background(), codex.ConfigValueWriteParams{
					KeyPath:       "model",
					MergeStrategy: "replace",
					Value:         json.RawMessage(`"o3"`),
				})
			case "config/batchWrite":
				_, err = client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{
					Edits: []codex.ConfigEdit{{
						KeyPath:       "model",
						MergeStrategy: "replace",
						Value:         json.RawMessage(`"o3"`),
					}},
				})
			}

			if err == nil {
				t.Fatal("expected invalid status error")
			}
			if !strings.Contains(err.Error(), `invalid status "stale"`) {
				t.Fatalf("error = %v; want invalid status", err)
			}
		})
	}
}

func TestConfigWrite_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("config/value/write", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "write failed",
		},
	})

	_, err := client.Config.Write(context.Background(), codex.ConfigValueWriteParams{KeyPath: "k", MergeStrategy: "replace", Value: json.RawMessage(`"v"`)})
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

func TestConfigBatchWrite_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("config/batchWrite", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "batch write failed",
		},
	})

	_, err := client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{Edits: []codex.ConfigEdit{{KeyPath: "k", MergeStrategy: "replace", Value: json.RawMessage(`"v"`)}}})
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

func TestConfigBatchWriteRejectsNilEditsBeforeSending(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	_, err := client.Config.BatchWrite(context.Background(), codex.ConfigBatchWriteParams{})
	if err == nil {
		t.Fatal("expected invalid params error")
	}
	if !strings.Contains(err.Error(), "invalid params") {
		t.Fatalf("error = %v, want invalid params context", err)
	}
	if !strings.Contains(err.Error(), "edits must not be null") {
		t.Fatalf("error = %v, want edits validation error", err)
	}
	if got := mock.CallCount(); got != 0 {
		t.Fatalf("transport recorded %d requests, want 0", got)
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
		Method:  "configWarning",
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

func TestConfigWarningNotificationRejectsMissingRequiredFields(t *testing.T) {
	t.Run("missing summary", func(t *testing.T) {
		var notif codex.ConfigWarningNotification
		err := json.Unmarshal([]byte(`{"details":"oops"}`), &notif)
		if err == nil || !strings.Contains(err.Error(), "missing required field") {
			t.Fatalf("json.Unmarshal error = %v; want missing required field failure", err)
		}
	})

	t.Run("range requires end position", func(t *testing.T) {
		var notif codex.ConfigWarningNotification
		err := json.Unmarshal([]byte(`{
			"summary":"bad config",
			"range":{"start":{"line":1,"column":2}}
		}`), &notif)
		if err == nil || !strings.Contains(err.Error(), "missing required field") {
			t.Fatalf("json.Unmarshal error = %v; want missing required field failure", err)
		}
	})
}

func TestConfigWarningMissingSummaryReportsHandlerError(t *testing.T) {
	mock := NewMockTransport()

	var (
		gotMethod string
		gotErr    error
	)
	client := codex.NewClient(mock, codex.WithHandlerErrorCallback(func(method string, err error) {
		gotMethod = method
		gotErr = err
	}))

	var called bool
	client.OnConfigWarning(func(codex.ConfigWarningNotification) {
		called = true
	})

	mock.InjectServerNotification(context.Background(), codex.Notification{
		JSONRPC: "2.0",
		Method:  "configWarning",
		Params:  json.RawMessage(`{"details":"oops"}`),
	})

	if called {
		t.Fatal("handler should not be called for malformed payload")
	}
	if gotMethod != "configWarning" {
		t.Fatalf("handler error method = %q; want %q", gotMethod, "configWarning")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "missing required field") {
		t.Fatalf("handler error = %v; want missing required field failure", gotErr)
	}
}
