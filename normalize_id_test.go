package codex

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeIDAcceptsSpecCompatibleIDs(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{name: "string", in: "abc", want: "abc"},
		{name: "int64", in: int64(99), want: "99"},
		{name: "int32", in: int32(-7), want: "-7"},
		{name: "uint32", in: uint32(42), want: "42"},
		{name: "whole float64", in: float64(456), want: "456"},
		{name: "json.Number integer", in: json.Number("9007199254740993"), want: "9007199254740993"},
		{name: "json.Number negative zero", in: json.Number("-0"), want: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeID(tt.in)
			if err != nil {
				t.Fatalf("normalizeID(%v) returned unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeID(%v) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeIDRejectsOffSpecNumericForms(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
	}{
		{name: "fractional float64", in: float64(3.14)},
		{name: "decimal json.Number", in: json.Number("1.0")},
		{name: "fractional json.Number", in: json.Number("2.5")},
		{name: "scientific json.Number", in: json.Number("1e3")},
		{name: "out of range json.Number", in: json.Number("9223372036854775808")},
		{name: "out of range uint64", in: uint64(math.MaxInt64) + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeID(tt.in)
			if err == nil {
				t.Fatalf("normalizeID(%v) returned nil error", tt.in)
			}
			if !errors.Is(err, errUnexpectedIDType) {
				t.Fatalf("normalizeID(%v) error = %v; want errUnexpectedIDType", tt.in, err)
			}
		})
	}
}

func TestNormalizeIDReturnsErrorOnUnexpectedType(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
	}{
		{name: "bool", in: true},
		{name: "struct", in: struct{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeID(tt.in)
			if err == nil {
				t.Fatalf("normalizeID(%v) returned nil error", tt.in)
			}
			if !errors.Is(err, errUnexpectedIDType) {
				t.Fatalf("normalizeID(%v) error = %v; want errUnexpectedIDType", tt.in, err)
			}
		})
	}
}

func TestNormalizeIDReturnsErrorOnNilID(t *testing.T) {
	_, err := normalizeID(nil)
	if err == nil {
		t.Fatal("normalizeID(nil) returned nil error")
	}
	if !errors.Is(err, errNullID) {
		t.Fatalf("normalizeID(nil) error = %v; want errNullID", err)
	}
}

func TestNormalizePendingRequestIDPrefixesTypeFamilies(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{name: "string", in: "1", want: "s:1"},
		{name: "number", in: int32(1), want: "n:1"},
		{name: "whole float64", in: float64(7), want: "n:7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePendingRequestID(tt.in)
			if err != nil {
				t.Fatalf("normalizePendingRequestID(%v) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("normalizePendingRequestID(%v) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizePendingRequestIDRejectsOffSpecNumericForms(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
	}{
		{name: "decimal", in: json.Number("1.0")},
		{name: "scientific", in: json.Number("1e3")},
		{name: "fractional", in: float64(1.5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizePendingRequestID(tt.in)
			if err == nil {
				t.Fatalf("normalizePendingRequestID(%v) returned nil error", tt.in)
			}
			if !errors.Is(err, errUnexpectedIDType) {
				t.Fatalf("normalizePendingRequestID(%v) error = %v; want errUnexpectedIDType", tt.in, err)
			}
		})
	}
}

func TestRequestIDEqualMatchesAcrossCompatibleIntegerTypes(t *testing.T) {
	tests := []struct {
		name string
		a    RequestID
		b    RequestID
	}{
		{
			name: "int32 and uint16",
			a:    RequestID{Value: int32(7)},
			b:    RequestID{Value: uint16(7)},
		},
		{
			name: "json.Number and float64 integer",
			a:    RequestID{Value: json.Number("42")},
			b:    RequestID{Value: float64(42)},
		},
		{
			name: "negative zero and zero",
			a:    RequestID{Value: json.Number("-0")},
			b:    RequestID{Value: int64(0)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.a.Equal(tt.b) {
				t.Fatalf("RequestID(%v).Equal(%v) = false; want true", tt.a.Value, tt.b.Value)
			}
			if !tt.b.Equal(tt.a) {
				t.Fatalf("RequestID(%v).Equal(%v) = false; want true", tt.b.Value, tt.a.Value)
			}
		})
	}
}

func TestRequestIDEqualRejectsOffSpecNumericForms(t *testing.T) {
	tests := []struct {
		name string
		a    RequestID
		b    RequestID
	}{
		{
			name: "decimal json.Number does not equal integer",
			a:    RequestID{Value: json.Number("1.0")},
			b:    RequestID{Value: int64(1)},
		},
		{
			name: "fractional float64 does not equal integer",
			a:    RequestID{Value: float64(1.5)},
			b:    RequestID{Value: int64(1)},
		},
		{
			name: "scientific json.Number does not equal integer",
			a:    RequestID{Value: json.Number("1e3")},
			b:    RequestID{Value: int64(1000)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.a.Equal(tt.b) {
				t.Fatalf("RequestID(%v).Equal(%v) = true; want false", tt.a.Value, tt.b.Value)
			}
			if tt.b.Equal(tt.a) {
				t.Fatalf("RequestID(%v).Equal(%v) = true; want false", tt.b.Value, tt.a.Value)
			}
		})
	}
}

func TestRequestIDSpecAllowsOnlyStringAndInt64(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("specs", "RequestId.json"))
	if err != nil {
		t.Fatalf("ReadFile(specs/RequestId.json) error = %v", err)
	}

	var spec struct {
		AnyOf []struct {
			Type   string `json:"type"`
			Format string `json:"format"`
		} `json:"anyOf"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("Unmarshal(specs/RequestId.json) error = %v", err)
	}

	if len(spec.AnyOf) != 2 {
		t.Fatalf("spec anyOf length = %d; want 2", len(spec.AnyOf))
	}

	var sawString bool
	var sawInt64 bool
	for _, item := range spec.AnyOf {
		if item.Type == "string" {
			sawString = true
		}
		if item.Type == "integer" && item.Format == "int64" {
			sawInt64 = true
		}
	}

	if !sawString || !sawInt64 {
		t.Fatalf("spec anyOf = %#v; want string and int64 integer entries", spec.AnyOf)
	}
}
