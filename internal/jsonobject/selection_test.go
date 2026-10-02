package jsonobject

import (
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzSelectedFields(f *testing.F) {
	for _, seed := range []string{`{"type":"a","type":null,"other":[]}`, `{"\u0074ype":"a","Type":"b"}`, `null`, `[]`, `{`, `{"type":{},"type":"b"}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		var all map[string]json.RawMessage
		wantErr := json.Unmarshal(data, &all)
		var want map[string]json.RawMessage
		if all != nil {
			want = make(map[string]json.RawMessage)
			for _, name := range []string{"type", "subAgent", "thread_spawn", "$schema"} {
				if value, ok := all[name]; ok {
					want[name] = value
				}
			}
		}
		got, err := SelectFields(data, "type", "subAgent", "thread_spawn", "$schema")
		if (err == nil) != (wantErr == nil) || (err == nil && !reflect.DeepEqual(got, want)) {
			t.Fatalf("selection differs: got=%v/%v want=%v/%v", got, err, want, wantErr)
		}
	})
}

func FuzzStringField(f *testing.F) {
	for _, seed := range []string{`{"type":"a","Type":null}`, `{"type":{},"TYPE":"b"}`, `{"\u0074ype":"a","Type":"b"}`, `{"type":"a","TYPE":99}`, `null`, `[]`, `{`, `{"type":"\ud800"}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		var want struct {
			Type string `json:"type"`
		}
		wantErr := json.Unmarshal(data, &want)
		got, err := TypeField(data)
		if (err == nil) != (wantErr == nil) || (err == nil && got != want.Type) {
			t.Fatalf("string selection differs: got=%q/%v want=%q/%v", got, err, want.Type, wantErr)
		}
	})
}
