package protocol_test

import (
	"context"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestMcpResourceReadRequiresTextOrBlob(t *testing.T) {
	tests := []struct {
		name     string
		content  map[string]interface{}
		wantErr  bool
		wantText *string
		wantBlob *string
	}{
		{
			name:    "missing both content representations",
			content: map[string]interface{}{"uri": "resource://item"},
			wantErr: true,
		},
		{
			name:     "text content",
			content:  map[string]interface{}{"uri": "resource://item", "text": "hello"},
			wantText: stringPointer("hello"),
		},
		{
			name:     "blob content",
			content:  map[string]interface{}{"uri": "resource://item", "blob": "AQID"},
			wantBlob: stringPointer("AQID"),
		},
		{
			name:     "both content representations",
			content:  map[string]interface{}{"uri": "resource://item", "text": "hello", "blob": "AQID"},
			wantText: stringPointer("hello"),
			wantBlob: stringPointer("AQID"),
		},
		{
			name:     "nullable opposite branch field",
			content:  map[string]interface{}{"uri": "resource://item", "text": "hello", "blob": nil},
			wantText: stringPointer("hello"),
		},
		{
			name:     "incompatible opposite blob extra",
			content:  map[string]interface{}{"uri": "resource://item", "text": "hello", "blob": 7},
			wantText: stringPointer("hello"),
		},
		{
			name:     "incompatible opposite text extra",
			content:  map[string]interface{}{"uri": "resource://item", "blob": "AQID", "text": map[string]interface{}{}},
			wantBlob: stringPointer("AQID"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			client := codex.NewClient(mock)
			if err := mock.SetResponseData("mcpServer/resource/read", map[string]interface{}{
				"contents": []interface{}{tt.content},
			}); err != nil {
				t.Fatalf("SetResponseData: %v", err)
			}

			resp, err := client.Mcp.ResourceRead(context.Background(), codex.McpResourceReadParams{
				Server: "server-1",
				URI:    "resource://item",
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("ResourceRead accepted content with neither text nor blob")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResourceRead: %v", err)
			}
			if len(resp.Contents) != 1 {
				t.Fatalf("got %d content items, want 1", len(resp.Contents))
			}
			if !sameOptionalString(resp.Contents[0].Text, tt.wantText) {
				t.Errorf("text = %v, want %v", resp.Contents[0].Text, tt.wantText)
			}
			if !sameOptionalString(resp.Contents[0].Blob, tt.wantBlob) {
				t.Errorf("blob = %v, want %v", resp.Contents[0].Blob, tt.wantBlob)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }

func sameOptionalString(got, want *string) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	return *got == *want
}
