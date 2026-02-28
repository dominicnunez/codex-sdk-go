package codex

import "testing"

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want interface{}
	}{
		// float64 (JSON default for numbers)
		{"positive float64 integer", float64(42), uint64(42)},
		{"zero float64", float64(0), uint64(0)},
		{"negative float64", float64(-1), float64(-1)},
		{"fractional float64", float64(3.14), float64(3.14)},

		// int64
		{"positive int64", int64(99), uint64(99)},
		{"zero int64", int64(0), uint64(0)},
		{"negative int64", int64(-1), int64(-1)},

		// int
		{"positive int", int(7), uint64(7)},
		{"zero int", int(0), uint64(0)},
		{"negative int", int(-5), int(-5)},

		// uint64 passthrough
		{"uint64", uint64(100), uint64(100)},

		// string passthrough
		{"string", "abc", "abc"},

		// unknown types stringified
		{"bool", true, "true"},
		{"nil", nil, "<nil>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeID(tt.in)
			if got != tt.want {
				t.Errorf("normalizeID(%v) = %v (%T); want %v (%T)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}
