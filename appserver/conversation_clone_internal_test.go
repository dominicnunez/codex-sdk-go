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

func TestCloneImageDetailOwnership(t *testing.T) {
	for _, input := range []UserInput{
		&ImageUserInput{FileID: Ptr("file"), Detail: Ptr(protocol.ImageDetailLow)},
		&LocalImageUserInput{Path: "relative", Detail: Ptr(protocol.ImageDetailLow)},
	} {
		getDetail := func(value UserInput) *protocol.ImageDetail {
			switch v := value.(type) {
			case *ImageUserInput:
				return v.Detail
			case *LocalImageUserInput:
				return v.Detail
			default:
				panic("not image input")
			}
		}
		cloned := cloneUserInput(input)
		*getDetail(cloned) = protocol.ImageDetailHigh
		if *getDetail(input) != protocol.ImageDetailLow {
			t.Fatal("clone detail mutation reached source")
		}
		*getDetail(input) = protocol.ImageDetailOriginal
		if *getDetail(cloned) != protocol.ImageDetailHigh {
			t.Fatal("source detail mutation reached clone")
		}
		item := ThreadItemWrapper{Value: &UserMessageThreadItem{Content: []UserInput{input}}}
		copiedItem := cloneThreadItemWrapper(item).Value.(*UserMessageThreadItem)
		*getDetail(copiedItem.Content[0]) = protocol.ImageDetailAuto
		if *getDetail(input) != protocol.ImageDetailOriginal {
			t.Fatal("item copy did not own nested image detail")
		}
	}
}
