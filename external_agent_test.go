package codex_test

import (
	"context"
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestExternalAgentConfigDetect(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		params       codex.ExternalAgentConfigDetectParams
		responseData map[string]interface{}
		wantItemsLen int
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
		},
		{
			name:   "no configs detected",
			params: codex.ExternalAgentConfigDetectParams{Cwds: ptr([]string{"/path/to/empty"})},
			responseData: map[string]interface{}{
				"items": []interface{}{},
			},
			wantItemsLen: 0,
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponseData("externalAgent/configDetect", tt.responseData)

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
		})
	}
}

func TestExternalAgentConfigImport(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		params codex.ExternalAgentConfigImportParams
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			mock.SetResponseData("externalAgent/configImport", map[string]interface{}{})

			client := codex.NewClient(mock)

			resp, err := client.ExternalAgent.ConfigImport(ctx, tt.params)
			if err != nil {
				t.Fatalf("ConfigImport() error = %v", err)
			}

			// Response is empty struct per spec
			_ = resp
		})
	}
}

func TestExternalAgentServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Compile-time verification that all methods exist with correct signatures
	_ = func(ctx context.Context, params codex.ExternalAgentConfigDetectParams) (codex.ExternalAgentConfigDetectResponse, error) {
		return client.ExternalAgent.ConfigDetect(ctx, params)
	}

	_ = func(ctx context.Context, params codex.ExternalAgentConfigImportParams) (codex.ExternalAgentConfigImportResponse, error) {
		return client.ExternalAgent.ConfigImport(ctx, params)
	}
}
