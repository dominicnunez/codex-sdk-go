package jsondecode

import (
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNativeErrorOwnershipAndNestedBounds(t *testing.T) {
	for _, literal := range []string{strings.Repeat("9", 1<<20), strings.Repeat("\x00", 1<<20)} {
		original := &json.UnmarshalTypeError{Value: "number " + literal, Type: reflect.TypeFor[int64](), Offset: 17, Struct: "Record", Field: "value"}
		before := *original
		bounded, ok := NativeError(original).(*json.UnmarshalTypeError)
		if !ok || bounded == original || !reflect.DeepEqual(*original, before) {
			t.Fatal("native metadata was mutated or the original retained")
		}
		if bounded.Type != before.Type || bounded.Offset != before.Offset || bounded.Struct != before.Struct || bounded.Field != before.Field || len(bounded.Value) > 2048 || !strings.Contains(bounded.Value, "bytes omitted") {
			t.Fatal("bounded native error lost metadata or retained an oversized description")
		}
		if NativeError(bounded) != bounded || errors.Unwrap(bounded) != nil {
			t.Fatal("nested decoding changed a bounded preview or retained its original cause")
		}
	}
	for _, size := range []int{0, 256, 2041} {
		original := &json.UnmarshalTypeError{Value: "number " + strings.Repeat("9", size), Type: reflect.TypeFor[int64]()}
		if NativeError(original) != original {
			t.Fatal("ordinary native error identity changed")
		}
	}
	syntax := &json.SyntaxError{Offset: 23}
	if NativeError(syntax) != syntax || NativeError(nil) != nil {
		t.Fatal("non-numeric JSON errors changed")
	}
}

type standardCycle struct {
	Next   *standardCycle
	Number int64
	After  string
}

func TestStandardCompositeNumericDecode(t *testing.T) {
	number := "1" + strings.Repeat("0", 1<<18)
	for _, test := range []struct {
		name string
		body string
		dest any
	}{
		{"signed", number, new(int64)},
		{"unsigned", number, new(uint64)},
		{"float", number, new(float64)},
		{"slice", "[" + number + "]", new([]int64)},
		{"map-value", `{"value":` + number + `}`, new(map[string]int64)},
		{"map-key", `{"` + number + `":0}`, new(map[int64]int)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var typed *json.UnmarshalTypeError
			if !errors.As(UnmarshalStandard([]byte(test.body), test.dest), &typed) || len(typed.Value) > 2048 || !strings.Contains(typed.Value, "bytes omitted") {
				t.Fatal("native generic composition lost bounded numeric rejection")
			}
		})
	}
	value := standardCycle{Number: 3, After: "seed"}
	value.Next = &value
	var typed *json.UnmarshalTypeError
	if !errors.As(UnmarshalStandard([]byte(`{"Number":`+number+`,"After":"new"}`), &value), &typed) || len(typed.Value) > 2048 || value.Number != 3 || value.After != "new" || value.Next != &value {
		t.Fatal("cyclic standard destination changed partial updates or failed to bound its diagnostic")
	}
}

func TestStringTagNativeOwnedError(t *testing.T) {
	var dest struct {
		N int64 `json:"n,string"`
	}
	quoted, err := json.Marshal(strings.Repeat("x", 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	original := json.Unmarshal([]byte(`{"n":`+string(quoted)+`}`), &dest)
	if original == nil {
		t.Fatal("native rejection missing")
	}
	before := original.Error()
	bounded := NativeError(original)
	if bounded == original || len(bounded.Error()) > 4096 || original.Error() != before || !strings.Contains(bounded.Error(), "bytes omitted") {
		t.Fatal("owned formatter normalization changed original or retained full diagnostic")
	}
	if NativeError(bounded) != bounded || errors.Unwrap(bounded) != nil {
		t.Fatal("formatter normalization is not idempotent or retains original cause")
	}
}

// The default backend includes quoted string values and dynamic object names
// in typed errors. Both must be bounded without mutating the native error.
func TestDefaultNativeDiagnosticBounds(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		target     any
	}{
		{"Number string", `"` + strings.Repeat("x", 1<<16) + `"`, new(json.Number)},
		{"bool string tag", `{"N":"` + strings.Repeat("x", 1<<16) + `"}`, new(struct {
			N bool `json:",string"`
		})},
		{"dynamic key", `{"` + strings.Repeat("k", 1<<16) + `":"bad"}`, new(map[string]int)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := json.Unmarshal([]byte(tc.body), tc.target)
			if original == nil {
				t.Fatal("native rejection missing")
			}
			before := original.Error()
			bounded := NativeError(original)
			if len(bounded.Error()) > 4096 || !strings.Contains(bounded.Error(), "bytes omitted") {
				t.Fatal("native diagnostic not bounded")
			}
			if original.Error() != before {
				t.Fatal("native error mutated")
			}
			if NativeError(bounded) != bounded {
				t.Fatal("bounding is not idempotent")
			}
			var native, got *json.UnmarshalTypeError
			if !errors.As(original, &native) || !errors.As(bounded, &got) || native.Type != got.Type || native.Offset != got.Offset || native.Struct != got.Struct || native.Err != got.Err {
				t.Fatal("typed cause or location lost")
			}
			for e := bounded; e != nil; e = errors.Unwrap(e) {
				if len(e.Error()) > 4096 {
					t.Fatal("full diagnostic retained in chain")
				}
			}
		})
	}
}

type defaultApplicationCodec int

var errDefaultApplication = &json.UnmarshalTypeError{
	Value: "number " + strings.Repeat("9", 1<<16), Type: reflect.TypeFor[int64](),
}

func (*defaultApplicationCodec) UnmarshalJSONFrom(*jsontext.Decoder) error {
	return errDefaultApplication
}

func TestDefaultApplicationDecoderOwnership(t *testing.T) {
	for _, tc := range []struct {
		body   string
		target any
	}{
		{`1`, new(defaultApplicationCodec)},
		{`{"N":1}`, new(struct{ N defaultApplicationCodec })},
		{`[1]`, new([]defaultApplicationCodec)},
		{`{"key":1}`, new(map[string]defaultApplicationCodec)},
	} {
		if err := UnmarshalStandard([]byte(tc.body), tc.target); err != errDefaultApplication {
			t.Fatalf("application streaming decoder error identity changed: %T", err)
		}
	}
}
