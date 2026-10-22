package protocol

import (
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

// ImageGenerationInputImage identifies a local image read for this generation.
type ImageGenerationInputImage struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func (i *ImageGenerationInputImage) UnmarshalJSON(data []byte) error {
	type wire ImageGenerationInputImage
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"path", "sha256"}, []string{"path", "sha256"}); err != nil {
		return err
	}
	path, err := validateInboundAbsolutePathField("inputImages.path", decoded.Path)
	if err != nil {
		return err
	}
	decoded.Path = path
	*i = ImageGenerationInputImage(decoded)
	return nil
}

// ImageGenerationThreadItem contains inline image data or a durable file reference.
// FileID is authoritative when a rollout contains both FileID and Result.
type ImageGenerationThreadItem struct {
	ID                    string                         `json:"id"`
	Status                string                         `json:"status"`
	FileID                *string                        `json:"fileId,omitempty"`
	Result                *string                        `json:"result,omitempty"`
	InputImages           *[]ImageGenerationInputImage   `json:"inputImages,omitempty"`
	Failure               *ImageGenerationFailureWrapper `json:"failure,omitempty"`
	RevisedPrompt         *string                        `json:"revisedPrompt,omitempty"`
	SavedPath             *string                        `json:"savedPath,omitempty"`
	TransparentBackground *bool                          `json:"transparentBackground,omitempty"`
}

// ImageGenerationFailure describes a known image-generation failure.
type ImageGenerationFailure interface{ imageGenerationFailure() }

// UsageLimitExceededImageGenerationFailure reports a usage limit and reset time.
type UsageLimitExceededImageGenerationFailure struct {
	LimitID  string `json:"limitId"`
	ResetsAt *int64 `json:"resetsAt,omitempty"`
}

func (UsageLimitExceededImageGenerationFailure) imageGenerationFailure() {}

// ImageGenerationFailureWrapper wraps the image-generation failure union.
type ImageGenerationFailureWrapper struct{ Value ImageGenerationFailure }

func (w *ImageGenerationFailureWrapper) UnmarshalJSON(data []byte) error {
	tag, err := decodeRequiredObjectTypeField(data, "image generation failure")
	if err != nil {
		return err
	}
	if tag != "usageLimitExceeded" {
		return errors.New("unknown image generation failure type")
	}
	var decoded UsageLimitExceededImageGenerationFailure
	if err := validateRequiredTaggedObjectFields(data, "limitId"); err != nil {
		return err
	}
	if err := jsondecode.Unmarshal(data, &decoded); err != nil {
		return err
	}
	w.Value = decoded
	return nil
}

func (w ImageGenerationFailureWrapper) MarshalJSON() ([]byte, error) {
	switch v := w.Value.(type) {
	case UsageLimitExceededImageGenerationFailure:
		return marshalImageUsageLimit(v)
	case *UsageLimitExceededImageGenerationFailure:
		if v != nil {
			return marshalImageUsageLimit(*v)
		}
	}
	return nil, errors.New("missing or unsupported image generation failure")
}

func marshalImageUsageLimit(v UsageLimitExceededImageGenerationFailure) ([]byte, error) {
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		UsageLimitExceededImageGenerationFailure
	}{Type: "usageLimitExceeded", UsageLimitExceededImageGenerationFailure: v})
}

func (ImageGenerationThreadItem) threadItem() {}

func (i *ImageGenerationThreadItem) MarshalJSON() ([]byte, error) {
	if i.FileID == nil && i.Result == nil {
		return nil, errors.New("imageGeneration requires fileId or result")
	}
	if i.InputImages != nil && *i.InputImages == nil {
		return nil, errors.New("inputImages must not be null")
	}
	type wire ImageGenerationThreadItem
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		*wire
	}{Type: "imageGeneration", wire: (*wire)(i)})
}

func decodeImageGenerationThreadItem(data []byte) (ThreadItem, error) {
	var decoded ImageGenerationThreadItem
	if err := validateRequiredTaggedObjectFields(data, "id", "status"); err != nil {
		return nil, err
	}
	// Decode common fields independently: each anyOf branch constrains only
	// its own alternative, so the other property may have any JSON shape.
	type wire ImageGenerationThreadItem
	common := struct {
		*wire
		FileID discardedImageAlternative `json:"fileId"`
		Result discardedImageAlternative `json:"result"`
	}{wire: (*wire)(&decoded)}
	if err := jsondecode.Unmarshal(data, &common); err != nil {
		return nil, err
	}
	var file struct {
		FileID *string `json:"fileId"`
	}
	if err := jsondecode.Unmarshal(data, &file); err == nil {
		decoded.FileID = file.FileID
	}
	var inline struct {
		Result *string `json:"result"`
	}
	if err := jsondecode.Unmarshal(data, &inline); err == nil {
		decoded.Result = inline.Result
	}
	// Prefer FileID when both alternatives succeed. Preserve a valid legacy
	// Result alongside it, but an invalid extra must never reject the item.
	if decoded.FileID == nil && decoded.Result == nil {
		return nil, errors.New("imageGeneration requires fileId or result")
	}
	if err := validateOptionalArrays(data, func(raw []byte) error {
		return validateArrayElements(raw, func(_ int, element []byte) error {
			if isNullJSONValue(element) {
				return errors.New("inputImages element must not be null")
			}
			return nil
		})
	}, "inputImages"); err != nil {
		return nil, err
	}
	if decoded.SavedPath != nil {
		path, err := validateInboundAbsolutePathField("savedPath", *decoded.SavedPath)
		if err != nil {
			return nil, err
		}
		decoded.SavedPath = &path
	}
	return &decoded, nil
}

type discardedImageAlternative struct{}

func (*discardedImageAlternative) UnmarshalJSON([]byte) error { return nil }
