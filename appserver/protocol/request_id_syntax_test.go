package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func TestRequestIDDirectSyntaxDiagnostics(t *testing.T) {
	for _, input := range []string{"", "{", "[1,", `"unterminated`, "1 x", "01", "1e", "null false", "  {bad}"} {
		t.Run(input, func(t *testing.T) {
			decoder := json.NewDecoder(bytes.NewReader([]byte(input)))
			decoder.UseNumber()
			var value interface{}
			want := decoder.Decode(&value)
			if want == nil {
				var trailing interface{}
				if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
					want = errors.New("invalid request id")
				}
			}
			id := RequestID{Value: "initial"}
			err := id.UnmarshalJSON([]byte(input))
			var gotSyntax, wantSyntax *json.SyntaxError
			gotTyped, wantTyped := errors.As(err, &gotSyntax), errors.As(want, &wantSyntax)
			if gotTyped != wantTyped || (gotTyped && gotSyntax.Offset != wantSyntax.Offset) || err.Error() != want.Error() || errors.Is(err, io.ErrUnexpectedEOF) != errors.Is(want, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) != errors.Is(want, io.EOF) {
				t.Fatalf("syntax diagnostics got=%v want=%v", err, want)
			}
			if id.Value != "initial" {
				t.Fatalf("invalid syntax changed receiver: %+v", id)
			}
		})
	}
}
