package protocol_test

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRequestIDRejectsCompositeWithoutMaterialization(t *testing.T) {
	object := `{"k":[` + strings.Repeat(`{},`, 50000) + `null]}`
	for _, data := range []string{object, `[` + object + `]`, `7 ` + object, object + ` x`, object[:len(object)-1], strings.Repeat("9", 1024*1024)} {
		raw := []byte(data)
		var id protocol.RequestID
		var err error
		if allocations := testing.AllocsPerRun(1, func() { err = id.UnmarshalJSON(raw) }); allocations > 30 {
			t.Fatalf("rejected ID allocated %.0f times", allocations)
		}
		if err == nil || len(err.Error()) > 128 || id.Value != nil {
			t.Fatalf("rejected ID changed state or exposed unbounded input: value=%v error=%v", id.Value, err)
		}
	}
}

func FuzzRequestIDScalarSemantics(f *testing.F) {
	for _, seed := range []string{"7", "null", " \t\r\n7 ", "\v7", "\f7", "\u00a07", `"x"`, `"\ud800"`, "1.0", "9223372036854775807", "9223372036854775808", `{}`, `7 {}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var value, trailing any
		err := dec.Decode(&value)
		valid := err == nil && dec.Decode(&trailing) == io.EOF
		if valid {
			switch v := value.(type) {
			case nil, string:
			case json.Number:
				value, err = v.Int64()
				valid = err == nil
			default:
				valid = false
			}
		}
		id := protocol.RequestID{Value: "unchanged"}
		err = id.UnmarshalJSON(data)
		if (err == nil) != valid || (valid && !reflect.DeepEqual(id.Value, value)) || (!valid && id.Value != "unchanged") {
			t.Fatalf("ID differs: got=%v/%v want=%v/%v data=%q", id.Value, err, value, valid, data)
		}
	})
}
