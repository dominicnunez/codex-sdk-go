package codex

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		// float64 (JSON default for numbers)
		{"positive float64 integer", float64(42), "42"},
		{"zero float64", float64(0), "0"},
		{"negative float64 integer", float64(-1), "-1"},
		{"negative float64 large", float64(-42), "-42"},
		{"fractional float64", float64(3.14), "3.14"},
		{"negative fractional float64", float64(-3.14), "-3.14"},

		// int64
		{"positive int64", int64(99), "99"},
		{"zero int64", int64(0), "0"},
		{"negative int64", int64(-1), "-1"},

		// int
		{"positive int", int(7), "7"},
		{"zero int", int(0), "0"},
		{"negative int", int(-5), "-5"},

		// uint64 passthrough
		{"uint64", uint64(100), "100"},
		{"json.Number large integer", json.Number("9007199254740993"), "9007199254740993"},
		{"json.Number integer decimal form", json.Number("1.0"), "1"},
		{"json.Number negative integer decimal form", json.Number("-7.0"), "-7"},
		{"json.Number fractional", json.Number("2.5"), "2.5"},
		{"json.Number exponent integer", json.Number("1e0"), "1"},
		{"json.Number negative zero", json.Number("-0"), "0"},
		{"json.Number negative zero decimal", json.Number("-0.0"), "0"},

		// string passthrough
		{"string", "abc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeID(tt.in)
			if err != nil {
				t.Fatalf("normalizeID(%v) returned unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("normalizeID(%v) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeIDReturnsErrorOnUnexpectedType(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
	}{
		{"bool", true},
		{"struct", struct{}{}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeID(tt.in)
			if err == nil {
				t.Errorf("normalizeID(%v) returned nil error for unexpected type", tt.in)
			}
			if !errors.Is(err, errUnexpectedIDType) {
				t.Errorf("normalizeID(%v) error = %v; want errUnexpectedIDType", tt.in, err)
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
		t.Errorf("normalizeID(nil) error = %v; want errNullID", err)
	}
}

func TestRequestIDEqualTreatsNegativeZeroAsZero(t *testing.T) {
	tests := []struct {
		name string
		a    RequestID
		b    RequestID
	}{
		{
			name: "json.Number -0 equals float zero",
			a:    RequestID{Value: json.Number("-0")},
			b:    RequestID{Value: float64(0)},
		},
		{
			name: "json.Number -0 equals int zero",
			a:    RequestID{Value: json.Number("-0")},
			b:    RequestID{Value: int(0)},
		},
		{
			name: "json.Number -0 equals uint64 zero",
			a:    RequestID{Value: json.Number("-0")},
			b:    RequestID{Value: uint64(0)},
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
