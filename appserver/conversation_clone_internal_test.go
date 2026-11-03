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

func TestCloneImageURLPresence(t *testing.T) {
	for _, url := range []*string{nil, Ptr(""), Ptr("opaque")} {
		input := &ImageUserInput{URL: url, FileID: Ptr("file")}
		copied := cloneUserInput(input).(*ImageUserInput)
		if url == nil {
			if copied.URL != nil {
				t.Fatal("nil URL changed")
			}
		} else {
			original := *url
			if copied.URL == nil || *copied.URL != original || copied.URL == url {
				t.Fatal("image URL not independently copied")
			}
			*copied.URL = "copy"
			if *url != original {
				t.Fatal("copy mutation reached source")
			}
			*url = "source"
			if *copied.URL != "copy" {
				t.Fatal("source mutation reached copy")
			}
		}
		output := &protocol.InputImageFunctionCallOutputContentItem{ImageURL: url, FileID: Ptr("file")}
		item := ThreadItemWrapper{Value: &protocol.FunctionCallOutputThreadItem{Output: protocol.FunctionCallOutputBody{Content: []protocol.FunctionCallOutputContentItemWrapper{{Value: output}}}}}
		cloned := cloneThreadItemWrapper(item).Value.(*protocol.FunctionCallOutputThreadItem).Output.Content[0].Value.(*protocol.InputImageFunctionCallOutputContentItem)
		if url == nil {
			if cloned.ImageURL != nil {
				t.Fatal("nil tool URL changed")
			}
		} else {
			if cloned.ImageURL == nil || cloned.ImageURL == url || *cloned.ImageURL != *url {
				t.Fatal("tool URL not copied")
			}
			old := *url
			*cloned.ImageURL = "tool-copy"
			if *url != old {
				t.Fatal("tool URL copy aliases source")
			}
			*url = "tool-source"
			if *cloned.ImageURL != "tool-copy" {
				t.Fatal("tool source aliases copy")
			}
		}
	}
}
