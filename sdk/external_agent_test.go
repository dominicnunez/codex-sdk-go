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

func TestExternalAgentConfigDetect(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		params       codex.ExternalAgentConfigDetectParams
		responseData map[string]interface{}
		wantItemsLen int
		wantJSON     map[string]interface{}
	}{
		{
			name:   "detected configs",
			params: codex.ExternalAgentConfigDetectParams{Cwds: ptr([]string{"/path/to/project"}), IncludeHome: ptr(true)},
			responseData: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"cwd":         "/path/to/project",
						"description": "Continue.dev configuration",
						"itemType":    "CONFIG",
					},
					map[string]interface{}{
						"cwd":         nil,
						"description": "Home MCP server config",
						"itemType":    "MCP_SERVER_CONFIG",
					},
				},
			},
			wantItemsLen: 2,
			wantJSON: map[string]interface{}{
				"cwds":        []interface{}{"/path/to/project"},
				"includeHome": true,
			},
		},
		{
			name:   "no configs detected",
			params: codex.ExternalAgentConfigDetectParams{Cwds: ptr([]string{"/path/to/empty"})},
			responseData: map[string]interface{}{
				"items": []interface{}{},
			},
			wantItemsLen: 0,
			wantJSON: map[string]interface{}{
				"cwds": []interface{}{"/path/to/empty"},
			},
		},
		{
			name:   "minimal params (no cwds, no includeHome)",
			params: codex.ExternalAgentConfigDetectParams{},
			responseData: map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"description": "Default skills",
						"itemType":    "SKILLS",
					},
				},
			},
			wantItemsLen: 1,
			wantJSON:     map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("externalAgentConfig/detect", tt.responseData)

			client := codex.NewClient(mock)

			resp, err := client.ExternalAgent.ConfigDetect(ctx, tt.params)
			if err != nil {
				t.Fatalf("ConfigDetect() error = %v", err)
			}

			if len(resp.Items) != tt.wantItemsLen {
				t.Errorf("ConfigDetect() Items length = %d, want %d", len(resp.Items), tt.wantItemsLen)
			}

			// Verify first item structure if items exist
			if tt.wantItemsLen > 0 && len(resp.Items) > 0 {
				item := resp.Items[0]
				if item.Description == "" {
					t.Error("ConfigDetect() first item Description is empty")
				}
				if item.ItemType == "" {
					t.Error("ConfigDetect() first item ItemType is empty")
				}
			}

			if len(mock.SentRequests) != 1 {
				t.Fatalf("expected 1 detect request, got %d", len(mock.SentRequests))
			}
			recordedReq := mock.SentRequests[0]
			if recordedReq.Method != "externalAgentConfig/detect" {
				t.Fatalf("method = %s; want externalAgentConfig/detect", recordedReq.Method)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(recordedReq.Params, &got); err != nil {
				t.Fatalf("request params decode failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantJSON) {
				t.Errorf("request params = %#v, want %#v", got, tt.wantJSON)
			}
		})
	}
}

func TestExternalAgentConfigImport(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		params   codex.ExternalAgentConfigImportParams
		wantJSON map[string]interface{}
	}{
		{
			name: "import single config item",
			params: codex.ExternalAgentConfigImportParams{
				MigrationItems: []codex.ExternalAgentConfigMigrationItem{
					{
						Cwd:         ptr("/path/to/project"),
						Description: "Continue.dev configuration",
						ItemType:    codex.MigrationItemTypeConfig,
					},
				},
			},
			wantJSON: map[string]interface{}{
				"migrationItems": []interface{}{
					map[string]interface{}{
						"cwd":         "/path/to/project",
						"description": "Continue.dev configuration",
						"itemType":    "CONFIG",
					},
				},
			},
		},
		{
			name: "import multiple items",
			params: codex.ExternalAgentConfigImportParams{
				MigrationItems: []codex.ExternalAgentConfigMigrationItem{
					{
						Description: "Home AGENTS.md",
						ItemType:    codex.MigrationItemTypeAgentsMd,
					},
					{
						Cwd:         ptr("/repo"),
						Description: "Project skills",
						ItemType:    codex.MigrationItemTypeSkills,
					},
					{
						Description: "MCP server configuration",
						ItemType:    codex.MigrationItemTypeMcpServerConfig,
					},
				},
			},
			wantJSON: map[string]interface{}{
				"migrationItems": []interface{}{
					map[string]interface{}{
						"description": "Home AGENTS.md",
						"itemType":    "AGENTS_MD",
					},
					map[string]interface{}{
						"cwd":         "/repo",
						"description": "Project skills",
						"itemType":    "SKILLS",
					},
					map[string]interface{}{
						"description": "MCP server configuration",
						"itemType":    "MCP_SERVER_CONFIG",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("externalAgentConfig/import", map[string]interface{}{})

			client := codex.NewClient(mock)

			resp, err := client.ExternalAgent.ConfigImport(ctx, tt.params)
			if err != nil {
				t.Fatalf("ConfigImport() error = %v", err)
			}

			// Response is empty struct per spec
			_ = resp

			if len(mock.SentRequests) != 1 {
				t.Fatalf("expected 1 import request, got %d", len(mock.SentRequests))
			}
			recordedReq := mock.SentRequests[0]
			if recordedReq.Method != "externalAgentConfig/import" {
				t.Fatalf("method = %s; want externalAgentConfig/import", recordedReq.Method)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(recordedReq.Params, &got); err != nil {
				t.Fatalf("request params decode failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantJSON) {
				t.Errorf("request params = %#v, want %#v", got, tt.wantJSON)
			}
		})
	}
}

func TestExternalAgentConfigDetect_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("externalAgentConfig/detect", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "detection failed",
		},
	})

	_, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
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

func TestExternalAgentConfigImport_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("externalAgentConfig/import", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "import failed",
		},
	})

	_, err := client.ExternalAgent.ConfigImport(context.Background(), codex.ExternalAgentConfigImportParams{MigrationItems: []codex.ExternalAgentConfigMigrationItem{{Description: "d", ItemType: codex.MigrationItemTypeConfig}}})
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

func TestExternalAgentConfigDetectRejectsInvalidItemType(t *testing.T) {
	mock := NewMockTransport()
	_ = mock.SetResponseData("externalAgentConfig/detect", map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"description": "Unsupported migration target",
				"itemType":    "PROMPTS",
			},
		},
	})

	client := codex.NewClient(mock)

	_, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
	if err == nil || !strings.Contains(err.Error(), `invalid externalAgentConfig.itemType "PROMPTS"`) {
		t.Fatalf("ConfigDetect error = %v; want invalid item type failure", err)
	}
}

func TestExternalAgentConfigDetectAcceptsEmptyCwd(t *testing.T) {
	mock := NewMockTransport()
	_ = mock.SetResponseData("externalAgentConfig/detect", map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"cwd":         "",
				"description": "Home-scoped skills",
				"itemType":    "SKILLS",
			},
		},
	})

	client := codex.NewClient(mock)

	resp, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
	if err != nil {
		t.Fatalf("ConfigDetect() error = %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("ConfigDetect() items length = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].Cwd == nil {
		t.Fatal("ConfigDetect() item cwd = nil, want empty-string pointer")
	}
	if *resp.Items[0].Cwd != "" {
		t.Fatalf("ConfigDetect() item cwd = %q, want empty string", *resp.Items[0].Cwd)
	}
}

func TestExternalAgentConfigDetectRejectsInvalidCwd(t *testing.T) {
	tests := []struct {
		name         string
		cwd          string
		wantContains string
	}{
		{
			name:         "relative cwd",
			cwd:          "relative/path",
			wantContains: `externalAgentConfig.cwd: must be an absolute path`,
		},
		{
			name:         "non-normalized cwd",
			cwd:          "/repo/../project",
			wantContains: `externalAgentConfig.cwd: must be normalized`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("externalAgentConfig/detect", map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{
						"cwd":         tt.cwd,
						"description": "Project skills",
						"itemType":    "SKILLS",
					},
				},
			})

			client := codex.NewClient(mock)

			_, err := client.ExternalAgent.ConfigDetect(context.Background(), codex.ExternalAgentConfigDetectParams{})
			if err == nil || !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("ConfigDetect error = %v; want substring %q", err, tt.wantContains)
			}
		})
	}
}

func TestExternalAgentConfigImportPreparesRequestParams(t *testing.T) {
	tests := []struct {
		name    string
		params  codex.ExternalAgentConfigImportParams
		wantErr string
		wantCwd string
	}{
		{
			name:    "nil migration items",
			params:  codex.ExternalAgentConfigImportParams{},
			wantErr: "migrationItems must not be null",
		},
		{
			name: "invalid migration item type",
			params: codex.ExternalAgentConfigImportParams{
				MigrationItems: []codex.ExternalAgentConfigMigrationItem{{
					Description: "Unsupported migration target",
					ItemType:    codex.ExternalAgentConfigMigrationItemType("PROMPTS"),
				}},
			},
			wantErr: `invalid externalAgentConfig.itemType "PROMPTS"`,
		},
		{
			name: "normalizes repo cwd",
			params: codex.ExternalAgentConfigImportParams{
				MigrationItems: []codex.ExternalAgentConfigMigrationItem{{
					Cwd:         ptr("/repo/./subdir/.."),
					Description: "Project skills",
					ItemType:    codex.MigrationItemTypeSkills,
				}},
			},
			wantCwd: "/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("externalAgentConfig/import", map[string]interface{}{})
			client := codex.NewClient(mock)

			_, err := client.ExternalAgent.ConfigImport(context.Background(), tt.params)
			if tt.wantErr != "" {
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
				return
			}

			if err != nil {
				t.Fatalf("ConfigImport() error = %v", err)
			}

			req := mock.GetSentRequest(0)
			if req == nil {
				t.Fatal("no request sent")
				return
			}

			var got struct {
				MigrationItems []struct {
					Cwd string `json:"cwd"`
				} `json:"migrationItems"`
			}
			if err := json.Unmarshal(req.Params, &got); err != nil {
				t.Fatalf("request params decode failed: %v", err)
			}
			if len(got.MigrationItems) != 1 {
				t.Fatalf("migrationItems length = %d, want 1", len(got.MigrationItems))
			}
			if got.MigrationItems[0].Cwd != tt.wantCwd {
				t.Fatalf("migrationItems[0].cwd = %q, want %q", got.MigrationItems[0].Cwd, tt.wantCwd)
			}
		})
	}
}
