package jsonencode_test

import (
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

func TestMarshalMatchesStandardAdmission(t *testing.T) {
	for _, literal := range []string{"", "0", "-0", "1e400", "01", "+1", "NaN", "1.", strings.Repeat("9", 1<<16)} {
		t.Run(literal[:min(len(literal), 20)], func(t *testing.T) {
			want, wantErr := json.Marshal(json.Number(literal))
			got, gotErr := jsonencode.Marshal(json.Number(literal))
			if string(got) != string(want) || (gotErr == nil) != (wantErr == nil) {
				t.Fatal("standard numeric admission or representation changed")
			}
			if wantErr != nil && gotErr.Error() != wantErr.Error() {
				t.Fatal("short standard diagnostic changed")
			}
		})
	}
}

func TestDefaultNumberMarshalDiagnostic(t *testing.T) {
	for _, value := range []any{json.Number(strings.Repeat("x", 1<<16)), map[string]any{"number": json.Number(strings.Repeat("x", 1<<16))}} {
		_, original := json.Marshal(value)
		_, bounded := jsonencode.Marshal(value)
		if original == nil || bounded == nil || len(bounded.Error()) > 4096 || !strings.Contains(bounded.Error(), "bytes omitted") {
			t.Fatal("native Number error not bounded")
		}
		for e := bounded; e != nil; e = errors.Unwrap(e) {
			if len(e.Error()) > 4096 {
				t.Fatal("unbounded error cause retained")
			}
		}
		var got, want *json.MarshalerError
		if !errors.As(bounded, &got) || !errors.As(original, &want) || got.Type != want.Type || !errors.Is(bounded, strconv.ErrSyntax) {
			t.Fatal("native Number error metadata/cause lost")
		}
	}
}

type defaultApplicationMarshaler struct{ err error }

func (v defaultApplicationMarshaler) MarshalJSONTo(*jsontext.Encoder) error { return v.err }

func TestDefaultApplicationMarshalOwnership(t *testing.T) {
	_, native := json.Marshal(json.Number(strings.Repeat("x", 1<<16)))
	for _, value := range []any{defaultApplicationMarshaler{native}, struct{ Value defaultApplicationMarshaler }{defaultApplicationMarshaler{native}}} {
		_, err := jsonencode.Marshal(value)
		var wrapper *json.MarshalerError
		if !errors.As(err, &wrapper) || wrapper.Err != native || !errors.Is(err, native) {
			t.Fatal("application streaming marshaler error changed")
		}
	}
}
