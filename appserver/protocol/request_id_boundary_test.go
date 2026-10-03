package protocol_test

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRequestIDNumericBoundaries(t *testing.T) {
	upper := math.Ldexp(1, 63)
	for _, tc := range []struct {
		name  string
		value any
		want  string
	}{
		{"upper-float", upper, ""},
		{"rounded-max-float", float64(math.MaxInt64), ""},
		{"above-upper", math.Nextafter(upper, math.Inf(1)), ""},
		{"nearest-valid-upper", math.Nextafter(upper, 0), "9223372036854774784"},
		{"lower-float", -upper, "-9223372036854775808"},
		{"below-lower", math.Nextafter(-upper, math.Inf(-1)), ""},
		{"nearest-valid-lower", math.Nextafter(-upper, 0), "-9223372036854774784"},
		{"nan", math.NaN(), ""}, {"positive-infinity", math.Inf(1), ""}, {"negative-infinity", math.Inf(-1), ""},
		{"fraction", 1.5, ""}, {"negative-zero", math.Copysign(0, -1), "0"},
		{"max-int64", int64(math.MaxInt64), "9223372036854775807"},
		{"min-int64", int64(math.MinInt64), "-9223372036854775808"},
		{"max-uint64-valid", uint64(math.MaxInt64), "9223372036854775807"},
		{"uint64-overflow", uint64(math.MaxInt64) + 1, ""},
		{"max-number", json.Number("9223372036854775807"), "9223372036854775807"},
		{"min-number", json.Number("-9223372036854775808"), "-9223372036854775808"},
		{"number-overflow", json.Number("9223372036854775808"), ""},
		{"number-underflow", json.Number("-9223372036854775809"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := codex.RequestID{Value: tc.value}
			data, err := json.Marshal(id)
			if tc.want == "" {
				if err == nil || id.Equal(id) || id.Equal(codex.RequestID{Value: int64(math.MinInt64)}) || (codex.RequestID{Value: int64(math.MinInt64)}).Equal(id) {
					t.Fatalf("invalid numeric ID accepted: wire=%s err=%v", data, err)
				}
				return
			}
			if err != nil || string(data) != tc.want {
				t.Fatalf("wire=%s err=%v want=%s", data, err, tc.want)
			}
			var decoded codex.RequestID
			if err := json.Unmarshal(data, &decoded); err != nil || !id.Equal(decoded) || !decoded.Equal(id) {
				t.Fatalf("valid ID round trip failed: %+v %v", decoded, err)
			}
			if id.Equal(codex.RequestID{Value: tc.want}) || (codex.RequestID{Value: tc.want}).Equal(id) {
				t.Fatal("numeric and string families collided")
			}
		})
	}
}

func FuzzRequestIDFloatRange(f *testing.F) {
	for _, value := range []float64{0, 1.5, math.Ldexp(1, 63), -math.Ldexp(1, 63), math.Nextafter(math.Ldexp(1, 63), 0), math.NaN(), math.Inf(1)} {
		f.Add(math.Float64bits(value))
	}
	f.Fuzz(func(t *testing.T, bits uint64) {
		value := math.Float64frombits(bits)
		// Exact rational arithmetic is independent of the floating range guard.
		rational := new(big.Rat).SetFloat64(value)
		valid := rational != nil && rational.IsInt() && rational.Num().IsInt64()
		id := codex.RequestID{Value: value}
		data, err := json.Marshal(id)
		if !valid {
			if err == nil || id.Equal(id) {
				t.Fatalf("invalid value %v admitted as %s", value, data)
			}
			return
		}
		if err != nil || string(data) != rational.Num().String() {
			t.Fatalf("value=%v wire=%s err=%v want=%s", value, data, err, rational.Num())
		}
		other := codex.RequestID{Value: rational.Num().Int64()}
		if !id.Equal(other) || !other.Equal(id) {
			t.Fatal("valid float and integer disagree")
		}
	})
}
