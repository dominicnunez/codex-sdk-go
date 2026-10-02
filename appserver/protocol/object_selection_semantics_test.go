package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

type selectionReferencePayload struct {
	ThreadID string     `json:"threadId"`
	Optional *string    `json:"optional"`
	Count    int        `json:"count"`
	Status   TurnStatus `json:"status"`
}

func TestInboundSelectionPreservesExistingPointerAliases(t *testing.T) {
	for _, data := range []string{
		`{"threadId":"a","optional":"first","optional":null,"optional":"last"}`,
		`{"threadId":"a","optional":"first","count":{},"optional":"last"}`,
	} {
		wantAlias, gotAlias := "initial", "initial"
		want := selectionReferencePayload{Optional: &wantAlias}
		got := selectionReferencePayload{Optional: &gotAlias}
		wantErr := referenceSelection([]byte(data), &want)
		err := unmarshalResponseObject([]byte(data), &got, []string{"threadId"}, []string{"threadId"})
		if selectionErrorKind(err) != selectionErrorKind(wantErr) || !reflect.DeepEqual(got, want) || gotAlias != wantAlias {
			t.Fatalf("alias differs: got=%+v/%q/%v want=%+v/%q/%v", got, gotAlias, err, want, wantAlias, wantErr)
		}
	}
}

// This independent Token/Decode model retains the prior decoder's wire-order
// validation, null rules, prefix mutation and first-error precedence.
func referenceSelection(data []byte, value *selectionReferencePayload) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	start, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrResultNotObject, err)
	}
	if start != json.Delim('{') {
		return ErrResultNotObject
	}
	seen := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: %w", ErrResultNotObject, err)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return fmt.Errorf("%w: %w", ErrResultNotObject, err)
		}
		switch key {
		case "threadId":
			seen = true
			if bytes.Equal(raw, []byte("null")) {
				return ErrNullResultField
			}
			err = json.Unmarshal(raw, &value.ThreadID)
		case "optional":
			err = json.Unmarshal(raw, &value.Optional)
		case "count":
			err = json.Unmarshal(raw, &value.Count)
		case "status":
			err = json.Unmarshal(raw, &value.Status)
		}
		if err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("%w: %w", ErrResultNotObject, err)
	}
	if !seen {
		return ErrMissingResultField
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func selectionErrorKind(err error) string {
	if err == nil {
		return "none"
	}
	for kind, target := range map[string]error{"null": ErrNullResultField, "missing": ErrMissingResultField, "object": ErrResultNotObject} {
		if errors.Is(err, target) {
			return kind
		}
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return "type"
	}
	return "syntax"
}

func FuzzInboundObjectSelection(f *testing.F) {
	for _, seed := range []string{
		`{"threadId":"a","threadId":null,"threadId":"b"}`,
		`{"threadId":99,"threadId":"b"}`,
		`{"threadId":"a","threadId":"b","ThreadID":"foreign"}`,
		`{"threadId":"a","optional":"x","optional":null,"count":2}`,
		`{"threadId":"a","optional":"x","optional":{},"optional":"y"}`,
		`{"threadId":"a","optional":"x","status":"bad","threadId":"b","optional":null}`,
		`{"threadId":"a","status":"completed","status":null,"optional":"x"}`,
		`{"threadId":null,"broken":`, `{"threadId":"a","count":2,"broken":`,
		`{} {}`, `{"threadId":"a"} {}`, `null`, `[]`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		wantInitial, gotInitial := "initial", "initial"
		want := selectionReferencePayload{ThreadID: "initial", Optional: &wantInitial, Count: 1}
		got := selectionReferencePayload{ThreadID: "initial", Optional: &gotInitial, Count: 1}
		wantErr := referenceSelection(data, &want)
		err := unmarshalResponseObject(data, &got, []string{"threadId"}, []string{"threadId"})
		if selectionErrorKind(err) != selectionErrorKind(wantErr) || !reflect.DeepEqual(got, want) {
			t.Fatalf("decode differs: got=%+v/%v want=%+v/%v data=%q", got, err, want, wantErr, data)
		}
	})
}
