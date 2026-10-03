package protocol

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

type syncedDecodeFixture struct {
	Name     string          `json:"name"`
	Title    *string         `json:"title,omitempty"`
	Enabled  bool            `json:"enabled"`
	Values   []int64         `json:"values"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// Compare pooled scratch state against the existing decoder, including direct
// malformed input and errors. The fixture does not implement either helper.
func FuzzSyncedObjectDecoder(f *testing.F) {
	for _, seed := range []string{
		`{"name":"","enabled":false,"values":[]}`,
		`{"name":"old","name":"new","enabled":false,"values":[]}`,
		`{"name":null,"name":"","enabled":false,"values":[]}`,
		`{"Name":"alias","enabled":false,"values":[]}`,
		`{"name":3,"name":"","enabled":false,"values":[]}`,
		`{"name":"","enabled":true,"values":[1,-1],"title":null,"metadata":[1]}`,
		`{"name":"prefix","title":"changed","enabled":true,`,
		`null`, `[]`, `{}`,
	} {
		f.Add([]byte(seed), false)
	}
	f.Fuzz(func(t *testing.T, data []byte, response bool) {
		oldTitle, newTitle := "initial", "initial"
		old := syncedDecodeFixture{Name: "initial", Title: &oldTitle, Values: []int64{9}}
		got := syncedDecodeFixture{Name: "initial", Title: &newTitle, Values: []int64{9}}
		required := []string{"name", "enabled", "values"}
		validation := inboundObjectValidationErrors()
		if response {
			validation = responseObjectValidationErrors()
		}
		oldErr := decodeObjectWithValidation(data, &old, required, required, validation)
		gotErr := unmarshalSyncedObject(data, &got, required, required, validation)
		if (oldErr == nil) != (gotErr == nil) {
			t.Fatalf("errors reference=%v pooled=%v", oldErr, gotErr)
		}
		if oldErr != nil && (reflect.TypeOf(oldErr) != reflect.TypeOf(gotErr) || oldErr.Error() != gotErr.Error()) {
			t.Fatalf("error contract reference=%T %v pooled=%T %v", oldErr, oldErr, gotErr, gotErr)
		}
		for _, sentinel := range []error{ErrResultNotObject, ErrMissingResultField, ErrNullResultField} {
			if errors.Is(oldErr, sentinel) != errors.Is(gotErr, sentinel) {
				t.Fatalf("error classification differs: %v", sentinel)
			}
		}
		if !reflect.DeepEqual(old, got) || oldTitle != newTitle {
			t.Fatalf("receiver reference=%+v pooled=%+v; prior titles %q %q", old, got, oldTitle, newTitle)
		}
	})
}

func TestSyncedObjectConcurrentRecovery(t *testing.T) {
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Go(func() {
			for j := 0; j < 32; j++ {
				var v ConnectorMetadata
				if err := json.Unmarshal([]byte(`{"id":"partial","name":null}`), &v); err == nil {
					t.Error("invalid record admitted")
				}
				if err := json.Unmarshal([]byte(`{"id":"","name":""}`), &v); err != nil || v.ID != "" || v.Name != "" {
					t.Errorf("state leaked: %+v %v", v, err)
				}
			}
		})
	}
	workers.Wait()
}
