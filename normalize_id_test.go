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
		{"positive int8", int8(8), "8"},
		{"positive int16", int16(16), "16"},
		{"positive int32", int32(32), "32"},

		// uint64 passthrough
		{"uint", uint(99), "99"},
		{"uint8", uint8(8), "8"},
		{"uint16", uint16(16), "16"},
		{"uint32", uint32(32), "32"},
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

func TestNormalizeIDPreservesLargeExponentIntegers(t *testing.T) {
	tests := []struct {
		in   json.Number
		want string
	}{
		{in: json.Number("9.007199254740992e15"), want: "9007199254740992"},
		{in: json.Number("9.007199254740993e15"), want: "9007199254740993"},
		{in: json.Number("9007199254740993e0"), want: "9007199254740993"},
	}

	for _, tt := range tests {
		got, err := normalizeID(tt.in)
		if err != nil {
			t.Fatalf("normalizeID(%q) returned error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeID(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeIDCanonicalizesEquivalentFloatAndJSONNumber(t *testing.T) {
	tests := []struct {
		name    string
		floatID float64
		rawID   json.Number
		want    string
	}{
		{
			name:    "fractional scientific notation",
			floatID: 1e-6,
			rawID:   json.Number("0.000001"),
			want:    "0.000001",
		},
		{
			name:    "integer scientific notation",
			floatID: 1e3,
			rawID:   json.Number("1000"),
			want:    "1000",
		},
		{
			name:    "negative scientific notation",
			floatID: -2.5e-4,
			rawID:   json.Number("-0.00025"),
			want:    "-0.00025",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFloat, err := normalizeID(tt.floatID)
			if err != nil {
				t.Fatalf("normalizeID(float %v) returned error: %v", tt.floatID, err)
			}
			gotRaw, err := normalizeID(tt.rawID)
			if err != nil {
				t.Fatalf("normalizeID(raw %q) returned error: %v", tt.rawID, err)
			}
			if gotFloat != tt.want {
				t.Fatalf("normalizeID(float %v) = %q; want %q", tt.floatID, gotFloat, tt.want)
			}
			if gotRaw != tt.want {
				t.Fatalf("normalizeID(raw %q) = %q; want %q", tt.rawID, gotRaw, tt.want)
			}
		})
	}
}

func TestRequestIDEqualMatchesEquivalentScientificAndDecimalForms(t *testing.T) {
	a := RequestID{Value: float64(1e-6)}
	b := RequestID{Value: json.Number("0.000001")}
	if !a.Equal(b) {
		t.Fatalf("RequestID(%v).Equal(%v) = false; want true", a.Value, b.Value)
	}
	if !b.Equal(a) {
		t.Fatalf("RequestID(%v).Equal(%v) = false; want true", b.Value, a.Value)
	}
}

func TestNormalizePendingRequestIDDoesNotCollideLargeExponentIntegers(t *testing.T) {
	a, err := normalizePendingRequestID(json.Number("9.007199254740992e15"))
	if err != nil {
		t.Fatalf("normalizePendingRequestID(a) returned error: %v", err)
	}
	b, err := normalizePendingRequestID(json.Number("9.007199254740993e15"))
	if err != nil {
		t.Fatalf("normalizePendingRequestID(b) returned error: %v", err)
	}
	if a == b {
		t.Fatalf("normalized IDs collided: %q == %q", a, b)
	}
}

func TestNormalizePendingRequestIDRejectsOversizedPositiveExponent(t *testing.T) {
	_, err := normalizePendingRequestID(json.Number("1e5000"))
	if err == nil {
		t.Fatal("normalizePendingRequestID should reject oversized positive exponent")
	}
	if !errors.Is(err, errUnexpectedIDType) {
		t.Fatalf("normalizePendingRequestID error = %v; want errUnexpectedIDType", err)
	}
}

func TestNormalizePendingRequestIDRejectsOversizedNegativeExponent(t *testing.T) {
	_, err := normalizePendingRequestID(json.Number("1e-5000"))
	if err == nil {
		t.Fatal("normalizePendingRequestID should reject oversized negative exponent")
	}
	if !errors.Is(err, errUnexpectedIDType) {
		t.Fatalf("normalizePendingRequestID error = %v; want errUnexpectedIDType", err)
	}
}

func TestNormalizePendingRequestIDSupportsAllIntegerKinds(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{name: "int32", in: int32(42), want: "n:42"},
		{name: "uint32", in: uint32(42), want: "n:42"},
		{name: "uint", in: uint(42), want: "n:42"},
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

func TestRequestIDEqualMatchesAcrossIntegerKinds(t *testing.T) {
	a := RequestID{Value: int32(7)}
	b := RequestID{Value: uint16(7)}
	if !a.Equal(b) {
		t.Fatalf("RequestID(%v).Equal(%v) = false; want true", a.Value, b.Value)
	}
	if !b.Equal(a) {
		t.Fatalf("RequestID(%v).Equal(%v) = false; want true", b.Value, a.Value)
	}
}

func TestRequestIDEqualRejectsOversizedNumericIDs(t *testing.T) {
	tests := []struct {
		name string
		a    RequestID
		b    RequestID
	}{
		{
			name: "distinct oversized positive exponents are not equal",
			a:    RequestID{Value: json.Number("1e5000")},
			b:    RequestID{Value: json.Number("2e5000")},
		},
		{
			name: "distinct oversized negative exponents are not equal",
			a:    RequestID{Value: json.Number("1e-5000")},
			b:    RequestID{Value: json.Number("2e-5000")},
		},
		{
			name: "oversized positive and negative exponents are not equal",
			a:    RequestID{Value: json.Number("1e5000")},
			b:    RequestID{Value: json.Number("1e-5000")},
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
