package jsonencode_test

import (
	"encoding/json"
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
