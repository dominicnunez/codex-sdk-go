package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// This reference is the prior single-record decoder, including its named wire
// target. Pointer URLs preserve the corrected representation without changing
// native root target names or field-error selection.
func imageWireReference(tool bool) any {
	if tool {
		type wire struct {
			Detail   *ImageDetail `json:"detail,omitempty"`
			ImageURL *string      `json:"image_url,omitempty"`
			FileID   *string      `json:"file_id,omitempty"`
		}
		return &wire{}
	}
	type wire struct {
		URL    *string      `json:"url,omitempty"`
		FileID *string      `json:"fileId,omitempty"`
		Detail *ImageDetail `json:"detail,omitempty"`
	}
	return &wire{}
}

func TestImageNativeErrorContract(t *testing.T) {
	for _, tool := range []bool{false, true} {
		for _, raw := range []string{`[]`, `false`, `17`, `"text"`, `{"url":17,"fileId":false,"detail":17}`, `{"detail":17,"url":17,"fileId":false}`, `{"url":"valid","detail":17}`, `{"url":{},"fileId":false}`} {
			if tool {
				raw = strings.NewReplacer(`"url"`, `"image_url"`, `"fileId"`, `"file_id"`).Replace(raw)
			}
			want := json.Unmarshal([]byte(raw), imageWireReference(tool))
			var got error
			if tool {
				var value InputImageFunctionCallOutputContentItem
				got = value.UnmarshalJSON([]byte(raw))
			} else {
				var value ImageUserInput
				got = value.UnmarshalJSON([]byte(raw))
			}
			var nativeWant, nativeGot *json.UnmarshalTypeError
			if !errors.As(want, &nativeWant) || !errors.As(got, &nativeGot) {
				t.Errorf("native cause lost for %s: %v / %v", raw, want, got)
				continue
			}
			// Separate local wire declarations have distinct Go identities, but
			// expose the same native target name and kind in diagnostics.
			a, b := *nativeWant, *nativeGot
			if a.Type.String() != b.Type.String() || a.Type.Kind() != b.Type.Kind() {
				t.Errorf("native target changed for %s: %v / %v", raw, a.Type, b.Type)
				continue
			}
			a.Type, b.Type = nil, nil
			if !reflect.DeepEqual(a, b) || want.Error() != got.Error() {
				t.Errorf("native contract changed for %s: %#v / %#v; %v / %v", raw, a, b, want, got)
			}
		}
	}
}
