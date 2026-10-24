//go:build !race

package protocol_test

import (
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Race instrumentation randomly discards sync.Pool entries, so quantitative
// allocation limits use standard builds. Rejection semantics run in both builds.
// Several measured runs amortize occasional pool misses in the standard build.
func TestRequestIDRejectsCompositeWithoutMaterialization(t *testing.T) {
	for _, data := range rejectedRequestIDInputs() {
		raw := []byte(data)
		var id protocol.RequestID
		var err error
		if allocations := testing.AllocsPerRun(10, func() { err = id.UnmarshalJSON(raw) }); allocations > 30 {
			t.Fatalf("rejected ID allocated %.0f times", allocations)
		}
		if err == nil || len(err.Error()) > 128 || id.Value != nil {
			t.Fatalf("rejected ID changed state or exposed unbounded input: value=%v error=%v", id.Value, err)
		}
	}
}
