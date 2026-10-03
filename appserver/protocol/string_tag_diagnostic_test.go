package protocol_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type stringTagRecord[T any] struct {
	Before int    `json:"before"`
	N      T      `json:"n,string"` //nolint:staticcheck // Tests instantiate T with Go's tag-supported scalars and pointers; SA5008 cannot classify the generic field.
	After  string `json:"after"`
}
type stringTagNamedInt[T any] int64

func TestStringTagNamedTargetSuffix(t *testing.T) {
	type target = stringTagNamedInt[struct {
		Label string `json:" into type"`
	}]
	quoted, err := json.Marshal(strings.Repeat("x into int64", 1<<16))
	if err != nil {
		t.Fatal(err)
	}
	var value protocol.OptionalNullable[stringTagRecord[target]]
	err = json.Unmarshal([]byte(`{"n":`+string(quoted)+`}`), &value)
	if err == nil || len(err.Error()) > 4096 || !strings.HasSuffix(err.Error(), " into "+reflect.TypeFor[target]().String()) {
		t.Fatal("quoted literal splitting changed application-defined target type")
	}
}

func TestStringTagNestedAndFreshReceivers(t *testing.T) {
	quoted, err := json.Marshal(strings.Repeat("x", 1<<18))
	if err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"before":7,"n":` + string(quoted) + `,"after":"new"}`)
	var fresh protocol.OptionalNullable[stringTagRecord[int64]]
	if err := json.Unmarshal(bad, &fresh); err == nil || len(err.Error()) > 4096 || fresh.Present || fresh.Value != nil {
		t.Fatal("failed fresh target published partial value")
	}
	if err := json.Unmarshal([]byte(`{"n":"4","after":"recovered"}`), &fresh); err != nil || !fresh.Present || fresh.Value == nil || fresh.Value.N != 4 {
		t.Fatal("fresh receiver failed to recover")
	}
	input := []byte(`{"a":[` + string(bad) + `],"b":[{"n":"9"}]}`)
	want := map[string][]stringTagRecord[int64]{"seed": {{N: 3}}}
	wantErr := json.Unmarshal(input, &want)
	got := map[string][]stringTagRecord[int64]{"seed": {{N: 3}}}
	value := protocol.OptionalNullable[map[string][]stringTagRecord[int64]]{Present: true, Value: &got}
	gotErr := json.Unmarshal(input, &value)
	if wantErr == nil || gotErr == nil || len(gotErr.Error()) > 4096 || !reflect.DeepEqual(got, want) {
		t.Fatal("nested map/slice owner or partial receiver changed")
	}
	for _, input := range [][]byte{bad, []byte(`{"n":null,"n":` + string(quoted) + `}`)} {
		wantNumber, gotNumber := int64(3), int64(3)
		want := stringTagRecord[*int64]{N: &wantNumber}
		wantErr := json.Unmarshal(input, &want)
		got := stringTagRecord[*int64]{N: &gotNumber}
		value := protocol.OptionalNullable[stringTagRecord[*int64]]{Present: true, Value: &got}
		gotErr := json.Unmarshal(input, &value)
		if wantErr == nil || gotErr == nil || len(gotErr.Error()) > 4096 || !reflect.DeepEqual(got, want) {
			t.Fatal("pointer/null target changed")
		}
	}
}

func checkStringTagDiagnostic[T any](t *testing.T, seed T) {
	t.Helper()
	for _, literal := range []string{
		strings.Repeat("x", 1<<20),
		"null" + strings.Repeat("x", 1<<20),
		"true" + strings.Repeat("x", 1<<20),
		"false" + strings.Repeat("x", 1<<20),
		`"` + strings.Repeat("x", 1<<20),
		strings.Repeat("\x00", 1<<18),
	} {
		quoted, err := json.Marshal(literal)
		if err != nil {
			t.Fatal(err)
		}
		input := []byte(`{"before":7,"n":` + string(quoted) + `,"after":"updated"}`)
		want := stringTagRecord[T]{N: seed, After: "seed"}
		wantErr := json.Unmarshal(input, &want)
		got := stringTagRecord[T]{N: seed, After: "seed"}
		value := protocol.OptionalNullable[stringTagRecord[T]]{Present: true, Value: &got}
		gotErr := json.Unmarshal(input, &value)
		if wantErr == nil || gotErr == nil {
			t.Fatal("invalid string-tag input admitted")
		}
		if len(gotErr.Error()) > 4096 || !strings.Contains(gotErr.Error(), "bytes omitted") {
			t.Fatalf("native string-tag diagnostic unbounded: bytes=%d", len(gotErr.Error()))
		}
		if !reflect.DeepEqual(got, want) || value.Value != &got || !value.Present {
			t.Fatal("native partial receiver behavior changed")
		}
		for current := gotErr; current != nil; current = errors.Unwrap(current) {
			if len(current.Error()) > 4096 {
				t.Fatal("full native diagnostic retained in error chain")
			}
		}
		if err := json.Unmarshal([]byte(`{"before":8,"n":null,"after":"recovered"}`), &value); err != nil || got.Before != 8 || got.After != "recovered" {
			t.Fatal("receiver failed to recover")
		}
	}
}

func TestStringTagNativeDiagnostic(t *testing.T) {
	t.Run("signed", func(t *testing.T) { checkStringTagDiagnostic(t, int64(3)) })
	t.Run("unsigned", func(t *testing.T) { checkStringTagDiagnostic(t, uint64(3)) })
	t.Run("float", func(t *testing.T) { checkStringTagDiagnostic(t, float64(3)) })
	t.Run("bool", func(t *testing.T) { checkStringTagDiagnostic(t, true) })
	t.Run("string", func(t *testing.T) { checkStringTagDiagnostic(t, "seed") })
	t.Run("Number", func(t *testing.T) { checkStringTagDiagnostic(t, json.Number("3")) })
}

func checkStringTagAdmission[T any](t *testing.T, seed T) {
	t.Helper()
	for _, literal := range []string{"", "null", "true", "false", "0", "-1", "01", "12x", "1e400", `"text"`, "x", strings.Repeat("9", 1<<16)} {
		quoted, err := json.Marshal(literal)
		if err != nil {
			t.Fatal(err)
		}
		input := []byte(`{"before":7,"n":` + string(quoted) + `,"after":"new"}`)
		want := stringTagRecord[T]{N: seed, After: "seed"}
		wantErr := json.Unmarshal(input, &want)
		got := stringTagRecord[T]{N: seed, After: "seed"}
		value := protocol.OptionalNullable[stringTagRecord[T]]{Present: true, Value: &got}
		gotErr := json.Unmarshal(input, &value)
		if (wantErr == nil) != (gotErr == nil) || !reflect.DeepEqual(got, want) || value.Value != &got || !value.Present {
			t.Fatal("string-tag admission or receiver changed")
		}
		if wantErr == nil {
			continue
		}
		if len(wantErr.Error()) <= 2048 && gotErr.Error() != wantErr.Error() {
			t.Fatal("short native string-tag diagnostic changed")
		}
		var wantTyped, gotTyped *json.UnmarshalTypeError
		if errors.As(wantErr, &wantTyped) {
			if !errors.As(gotErr, &gotTyped) || wantTyped.Type != gotTyped.Type || wantTyped.Offset != gotTyped.Offset || wantTyped.Struct != gotTyped.Struct || wantTyped.Field != gotTyped.Field {
				t.Fatal("structured conversion context changed")
			}
		}
	}
}

func TestStringTagStandardAdmission(t *testing.T) {
	t.Run("signed", func(t *testing.T) { checkStringTagAdmission(t, int64(3)) })
	t.Run("unsigned", func(t *testing.T) { checkStringTagAdmission(t, uint64(3)) })
	t.Run("float", func(t *testing.T) { checkStringTagAdmission(t, float64(3)) })
	t.Run("bool", func(t *testing.T) { checkStringTagAdmission(t, true) })
	t.Run("string", func(t *testing.T) { checkStringTagAdmission(t, "seed") })
	t.Run("Number", func(t *testing.T) { checkStringTagAdmission(t, json.Number("3")) })
}

func TestStringTagFailurePrecedence(t *testing.T) {
	for _, literal := range []string{strings.Repeat("x", 1<<18), "null" + strings.Repeat("x", 1<<18), "x into int64" + strings.Repeat("\x00", 1<<18)} {
		quoted, err := json.Marshal(literal)
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range []string{
			`{"before":"bad","n":` + string(quoted) + `,"after":"new"}`,
			`{"before":7,"n":` + string(quoted) + `,"n":"8","after":7}`,
			`{"before":7,"n":null,"n":` + string(quoted) + `,"after":"new"}`,
		} {
			want := stringTagRecord[int64]{N: 3, After: "seed"}
			wantErr := json.Unmarshal([]byte(input), &want)
			got := stringTagRecord[int64]{N: 3, After: "seed"}
			value := protocol.OptionalNullable[stringTagRecord[int64]]{Present: true, Value: &got}
			gotErr := json.Unmarshal([]byte(input), &value)
			if wantErr == nil || gotErr == nil || !reflect.DeepEqual(got, want) {
				t.Fatal("duplicate/null/mixed failure receiver changed")
			}
			if len(wantErr.Error()) <= 2048 {
				if gotErr.Error() != wantErr.Error() {
					t.Fatal("earlier native error lost precedence")
				}
			} else if len(gotErr.Error()) > 4096 || !strings.HasSuffix(gotErr.Error(), " into int64") {
				t.Fatal("later native formatter ownership or target changed")
			}
		}
	}
}

type stringTagApplicationJSON struct{ Err error }

func (v *stringTagApplicationJSON) UnmarshalJSON([]byte) error { return v.Err }

type stringTagApplicationText struct{ Err error }

func (v *stringTagApplicationText) UnmarshalText([]byte) error { return v.Err }

type stringTagWrappedError struct{ Cause error }

func (e *stringTagWrappedError) Error() string { return "application codec: " + e.Cause.Error() }
func (e *stringTagWrappedError) Unwrap() error { return e.Cause }

func TestStringTagApplicationErrorOwnership(t *testing.T) {
	quoted, err := json.Marshal(strings.Repeat("x", 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	var native stringTagRecord[int64]
	shared := json.Unmarshal([]byte(`{"n":`+string(quoted)+`}`), &native)
	jsonCodec := stringTagApplicationJSON{shared}
	jsonValue := protocol.OptionalNullable[stringTagApplicationJSON]{Present: true, Value: &jsonCodec}
	if err := json.Unmarshal([]byte(`{}`), &jsonValue); err != shared || !errors.Is(err, shared) {
		t.Fatal("application JSON error changed")
	}
	textCodec := stringTagApplicationText{shared}
	textValue := protocol.OptionalNullable[stringTagApplicationText]{Present: true, Value: &textCodec}
	if err := json.Unmarshal([]byte(`"ignored"`), &textValue); err != shared || !errors.Is(err, shared) {
		t.Fatal("application Text error changed")
	}
	if len(shared.Error()) < 1<<20 {
		t.Fatal("shared application error was mutated")
	}
	if jsonValue.Value != &jsonCodec || textValue.Value != &textCodec {
		t.Fatal("application receiver changed")
	}
	wrapped := &stringTagWrappedError{Cause: shared}
	promoted := struct{ *stringTagApplicationJSON }{&stringTagApplicationJSON{Err: wrapped}}
	promotedValue := protocol.OptionalNullable[struct{ *stringTagApplicationJSON }]{Present: true, Value: &promoted}
	err = json.Unmarshal([]byte(`{}`), &promotedValue)
	var matched *stringTagWrappedError
	if err != wrapped || !errors.Is(err, shared) || !errors.As(err, &matched) || matched != wrapped || promotedValue.Value != &promoted {
		t.Fatal("promoted application codec lost error chain or receiver")
	}
	dynamic := any(&stringTagRecord[int64]{N: 3})
	dynamicValue := protocol.OptionalNullable[any]{Present: true, Value: &dynamic}
	err = json.Unmarshal([]byte(`{"n":`+string(quoted)+`}`), &dynamicValue)
	if err == nil || len(err.Error()) < 1<<20 || dynamicValue.Value != &dynamic {
		t.Fatal("dynamic interface provenance exclusion changed")
	}
}

func checkActiveDashDecoder[T any](t *testing.T, target *T, shared error, input string) {
	t.Helper()
	value := protocol.OptionalNullable[T]{Present: true, Value: target}
	if err := json.Unmarshal([]byte(input), &value); err != shared || !errors.Is(err, shared) || value.Value != target {
		t.Fatal("active dash-named application decoder error ownership changed")
	}
}

func TestStringTagActiveDashFieldOwnership(t *testing.T) {
	quoted, err := json.Marshal(strings.Repeat("x", 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	var native stringTagRecord[int64]
	stringTagErr := json.Unmarshal([]byte(`{"n":`+string(quoted)+`}`), &native)
	var number json.Number
	numberErr := json.Unmarshal([]byte(`"`+strings.Repeat("x", 1<<20)+`"`), &number)
	var integer int64
	conversionErr := json.Unmarshal([]byte("1"+strings.Repeat("0", 1<<20)), &integer)
	for _, family := range []struct {
		name string
		err  error
	}{{"string tag", stringTagErr}, {"Number formatter", numberErr}, {"numeric conversion", conversionErr}} {
		if family.err == nil || len(family.err.Error()) <= 2048 {
			t.Fatal("native oversized error fixture missing")
		}
		t.Run(family.name, func(t *testing.T) {
			shared := family.err
			t.Run("JSON optional tag", func(t *testing.T) {
				target := struct {
					Codec stringTagApplicationJSON `json:"-,omitempty"` //nolint:staticcheck // Intentionally exercises Go's active dash-named field, not an ignored field.
				}{stringTagApplicationJSON{shared}}
				checkActiveDashDecoder(t, &target, shared, `{"-":{}}`)
			})
			t.Run("JSON string tag", func(t *testing.T) {
				target := struct {
					Codec stringTagApplicationJSON `json:"-,string"` //nolint:staticcheck // Intentionally exercises Go's active dash-named field, not an ignored field.
				}{stringTagApplicationJSON{shared}}
				checkActiveDashDecoder(t, &target, shared, `{"-":{}}`)
			})
			t.Run("JSON empty option", func(t *testing.T) {
				target := struct {
					Codec stringTagApplicationJSON `json:"-,"` //nolint:staticcheck // Intentionally exercises Go's active dash-named field, not an ignored field.
				}{stringTagApplicationJSON{shared}}
				checkActiveDashDecoder(t, &target, shared, `{"-":{}}`)
			})
			t.Run("Text optional tag", func(t *testing.T) {
				target := struct {
					Codec stringTagApplicationText `json:"-,omitempty"` //nolint:staticcheck // Intentionally exercises Go's active dash-named field, not an ignored field.
				}{stringTagApplicationText{shared}}
				checkActiveDashDecoder(t, &target, shared, `{"-":"ignored"}`)
			})
		})
	}
	t.Run("exact dash tag is ignored", func(t *testing.T) {
		target := struct {
			Codec stringTagApplicationJSON `json:"-"`
			N     int64                    `json:"n,string"`
		}{Codec: stringTagApplicationJSON{stringTagErr}}
		checkIgnoredDashDecoder(t, &target, []byte(`{"-":{},"n":`+string(quoted)+`}`))
	})
}

func checkIgnoredDashDecoder[T any](t *testing.T, target *T, input []byte) {
	t.Helper()
	value := protocol.OptionalNullable[T]{Present: true, Value: target}
	err := json.Unmarshal(input, &value)
	if err == nil || len(err.Error()) > 4096 || !strings.Contains(err.Error(), "bytes omitted") || value.Value != target {
		t.Fatal("ignored custom decoder prevented native diagnostic normalization")
	}
}

// Use the same standard custom-decoder entry boundary. encoding/json passes a
// codec its selected value without outer whitespace, so a direct struct decode
// is not an independent reference for the codec's relative error offsets.
type stringTagReference[T any] struct{ Record stringTagRecord[T] }

func (v *stringTagReference[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &v.Record)
}

func checkStringTagFuzz[T any](t *testing.T, input []byte) {
	t.Helper()
	reference := stringTagReference[T]{Record: stringTagRecord[T]{Before: 5, After: "seed"}}
	wantErr := json.Unmarshal(input, &reference)
	want := reference.Record
	got := stringTagRecord[T]{Before: 5, After: "seed"}
	value := protocol.OptionalNullable[stringTagRecord[T]]{Present: true, Value: &got}
	gotErr := json.Unmarshal(input, &value)
	if string(bytes.TrimSpace(input)) == "null" {
		if gotErr != nil || !value.Present || value.Value != nil {
			t.Fatal("nullable root contract changed")
		}
		return
	}
	if (wantErr == nil) != (gotErr == nil) || !reflect.DeepEqual(got, want) || value.Value != &got || !value.Present {
		t.Fatal("native generic admission or partial receiver changed")
	}
	if wantErr == nil {
		return
	}
	if len(wantErr.Error()) <= 2048 {
		if gotErr.Error() != wantErr.Error() {
			t.Fatal("short native error changed")
		}
	} else if len(gotErr.Error()) > 4096 || !strings.Contains(gotErr.Error(), "bytes omitted") {
		t.Fatal("large native error unbounded")
	}
	var wantTyped, gotTyped *json.UnmarshalTypeError
	if errors.As(wantErr, &wantTyped) {
		if !errors.As(gotErr, &gotTyped) || wantTyped.Type != gotTyped.Type || wantTyped.Offset != gotTyped.Offset || wantTyped.Struct != gotTyped.Struct || wantTyped.Field != gotTyped.Field {
			t.Fatal("native type context changed")
		}
	}
}

func FuzzStringTagNativeCompatibility(f *testing.F) {
	for _, input := range []string{`{"n":"4"}`, `{"before":"bad","n":"x","after":"new"}`, `{"before":"bad","n":"nullx","after":"new"}`, `{"n":null,"n":"true","after":7}`, `{"n":"\"text\""}`, `null`, `{"n":"x` + strings.Repeat("y", 4096) + `"}`} {
		f.Add([]byte(input))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 16<<10 {
			t.Skip()
		}
		checkStringTagFuzz[int64](t, input)
		checkStringTagFuzz[bool](t, input)
		checkStringTagFuzz[string](t, input)
	})
}
