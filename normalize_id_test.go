package codex

import (
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
