package protocol_test

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestRPCErrorCodeInt64(t *testing.T) {
	for _, code := range []int64{math.MinInt64, -2147483649, -32700, 0, 2147483648, math.MaxInt64} {
		t.Run(strconv.FormatInt(code, 10), func(t *testing.T) {
			underlying := &p.Error{Code: code, Message: "fixture", Data: json.RawMessage(`"private data"`)}
			rpc := p.NewRPCError(underlying)
			accessor := rpc.Code
			if accessor() != code || rpc.RPCError() != underlying || !errors.Is(rpc, p.NewRPCError(&p.Error{Code: code})) {
				t.Fatal("64-bit error access or identity changed")
			}
			if errors.Is(rpc, p.NewRPCError(&p.Error{Code: code ^ 1<<32})) {
				t.Fatal("different high bits matched")
			}
			if !strings.Contains(rpc.Error(), strconv.FormatInt(code, 10)) || strings.Contains(rpc.Error(), "private data") {
				t.Fatal("diagnostic lost code or exposed data")
			}
			encoded, err := json.Marshal(underlying)
			if err != nil {
				t.Fatal(err)
			}
			var round p.Error
			if err := json.Unmarshal(encoded, &round); err != nil || round.Code != code || round.Message != underlying.Message || string(round.Data) != string(underlying.Data) {
				t.Fatalf("error roundtrip changed: %+v %v", round, err)
			}
		})
	}
	if p.NewRPCError(nil).Code() != 0 {
		t.Fatal("nil underlying error code changed")
	}
}
