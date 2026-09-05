package protocol

import (
	"errors"
	"testing"
)

// A named string without methods takes the original RawMessage fallback while
// having the same JSON value semantics as string.
type baselineValidationString string

func compareStringValidation(t *testing.T, data []byte, nonNull bool) {
	t.Helper()
	fast := struct {
		Text string `json:"text"`
	}{Text: "initial"}
	slow := struct {
		Text baselineValidationString `json:"text"`
	}{Text: "initial"}
	var nonNullFields []string
	if nonNull {
		nonNullFields = []string{"text"}
	}
	fastErr := unmarshalResponseObject(data, &fast, []string{"text"}, nonNullFields)
	slowErr := unmarshalResponseObject(data, &slow, []string{"text"}, nonNullFields)
	if (fastErr == nil) != (slowErr == nil) {
		t.Fatalf("acceptance differs: direct=%v fallback=%v", fastErr, slowErr)
	}
	if fast.Text != string(slow.Text) {
		t.Fatalf("value differs: direct=%q fallback=%q", fast.Text, slow.Text)
	}
	for _, sentinel := range []error{ErrResultNotObject, ErrMissingResultField, ErrNullResultField} {
		if errors.Is(fastErr, sentinel) != errors.Is(slowErr, sentinel) {
			t.Fatalf("error classification differs: direct=%v fallback=%v", fastErr, slowErr)
		}
	}
}

func TestDirectStringValidationMatchesFallback(t *testing.T) {
	for _, input := range []string{
		`{"text":"hello"}`, `{"text":""}`, `{"text":"a\n\t\"b"}`, `{"text":"\ud800"}`,
		`{}`, `null`, `[]`, `{"text":null}`, `{"text":42}`, `{"text":true}`, `{"text":{}}`, `{"text":[]}`,
		`{"text":"first","text":"last"}`, `{"text":"first","text":null}`, `{"text":null,"text":"last"}`,
		`{"TEXT":"case"}`, `{"te\u0078t":"escaped key"}`, `{"unknown":{"nested":[1,2]},"text":"ok"}`,
		`{"text":"ok"} true`, `{"text":"unterminated}`, `{"text":tru}`, `{"text":`,
	} {
		t.Run(input, func(t *testing.T) {
			compareStringValidation(t, []byte(input), false)
			compareStringValidation(t, []byte(input), true)
		})
	}
}

func FuzzDirectStringValidationMatchesFallback(f *testing.F) {
	for _, seed := range []string{`{"text":"hello"}`, `{"text":null}`, `{"text":"first","text":null}`, `{"text":{}}`, `{"text":`, `{"text":"\ud800"}`} {
		f.Add([]byte(seed), true)
		f.Add([]byte(seed), false)
	}
	f.Fuzz(func(t *testing.T, data []byte, nonNull bool) { compareStringValidation(t, data, nonNull) })
}
