package codex_test

import (
	"errors"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/sdk"
)

func assertRPCErrorCode(t *testing.T, err error, want int) {
	t.Helper()

	if err == nil {
		t.Fatal("expected RPC error")
	}

	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error type = %T; want *RPCError", err)
	}
	if rpcErr.Code() != want {
		t.Fatalf("rpc error code = %d; want %d", rpcErr.Code(), want)
	}
}
