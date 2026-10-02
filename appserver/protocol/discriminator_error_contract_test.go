package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestPublicDiscriminatorErrorContext(t *testing.T) {
	for name, makeReceiver := range map[string]func() json.Unmarshaler{
		"account":              func() json.Unmarshaler { return &AccountWrapper{} },
		"command":              func() json.Unmarshaler { return &CommandActionWrapper{} },
		"parsed command":       func() json.Unmarshaler { return &ParsedCommandWrapper{} },
		"file change":          func() json.Unmarshaler { return &FileChangeWrapper{} },
		"patch":                func() json.Unmarshaler { return &PatchChangeKindWrapper{} },
		"search":               func() json.Unmarshaler { return &WebSearchActionWrapper{} },
		"review":               func() json.Unmarshaler { return &ReviewTargetWrapper{} },
		"common item selector": func() json.Unmarshaler { return &ThreadItemWrapper{} },
	} {
		t.Run(name, func(t *testing.T) {
			for _, input := range []string{`{"metadata":"before","type":1}`, `{"type":"future","TYPE":{},"type":"future"}`, `{"type":[],"\u0074ype":false}`} {
				var reference struct {
					Type string `json:"type"`
				}
				want := json.Unmarshal([]byte(input), &reference)
				err := makeReceiver().UnmarshalJSON([]byte(input))
				var gotType, wantType *json.UnmarshalTypeError
				if !errors.As(err, &gotType) || !errors.As(want, &wantType) || !reflect.DeepEqual(gotType, wantType) {
					t.Fatalf("input=%s error context got=%+v want=%+v", input, gotType, wantType)
				}
			}
		})
	}
}
