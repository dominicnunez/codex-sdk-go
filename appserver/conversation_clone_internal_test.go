package appserver

import (
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestCloneNewSchemaPointerFields(t *testing.T) {
	fileID := "file-1"
	image := &ImageUserInput{FileID: &fileID}
	clonedImage := cloneUserInput(image).(*ImageUserInput)
	*clonedImage.FileID = "changed"
	if *image.FileID != "file-1" {
		t.Fatalf("image file ID mutation leaked: %q", *image.FileID)
	}

	mcp := &McpToolCallThreadItem{McpAppUI: &protocol.McpAppUI{ResourceURI: "ui://original"}}
	clonedMCP := cloneThreadItemWrapper(ThreadItemWrapper{Value: mcp}).Value.(*McpToolCallThreadItem)
	clonedMCP.McpAppUI.ResourceURI = "ui://changed"
	if mcp.McpAppUI.ResourceURI != "ui://original" {
		t.Fatalf("MCP app UI mutation leaked: %q", mcp.McpAppUI.ResourceURI)
	}
}
