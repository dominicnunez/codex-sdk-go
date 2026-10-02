package jsonobject

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestTypeFieldErrorContext(t *testing.T) {
	for _, input := range []string{`{"type":1}`, ` {"metadata":1,"TYPE":false}`, `{"type":{},"type":"later"}`, `{"type":"earlier","type":[]}`, `{"type":null,"\u0074ype":99}`, `{"metadata":"x","type":[1,2],"Type":false}`} {
		var reference struct {
			Type string `json:"type"`
		}
		want := json.Unmarshal([]byte(input), &reference)
		_, err := TypeField([]byte(input))
		var gotType, wantType *json.UnmarshalTypeError
		if !errors.As(err, &gotType) || !errors.As(want, &wantType) || !reflect.DeepEqual(gotType, wantType) {
			t.Fatalf("input=%s error context got=%+v want=%+v", input, gotType, wantType)
		}
	}
}

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
		if !reflect.DeepEqual(err, wantErr) || got != want.Type {
			t.Fatalf("string selection differs: got=%q/%v want=%q/%v", got, err, want.Type, wantErr)
		}
	})
}

func FuzzIDField(f *testing.F) {
	for _, seed := range []string{`{"id":"a","ID":"b"}`, `{"id":"a","ID":null}`, `{"id":99,"id":"b"}`, `{"\u0069d":"a"}`, `null`, `[]`, `{`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		var reference struct {
			ID string `json:"id"`
		}
		wantErr := json.Unmarshal(data, &reference)
		got, err := IDField(data)
		if !reflect.DeepEqual(err, wantErr) || got != reference.ID {
			t.Fatalf("ID selection=%q/%v reference=%q/%v data=%q", got, err, reference.ID, wantErr, data)
		}
	})
}
