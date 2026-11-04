package appserver_test

import (
	"runtime"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
)

func TestCollectorPlanByteCounter(t *testing.T) {
	const chunkBytes = 8 << 20
	const retainedBytes = 64 << 10
	delta := strings.Repeat("x", chunkBytes-4) + "tail"
	collector := codex.NewStreamCollector()
	for count := 1; count <= 257; count++ {
		collector.Process(&codex.PlanDelta{ThreadID: "thread", TurnID: "turn", ItemID: "plan", Delta: delta}, nil)
		if count < 256 {
			continue
		}
		summary := collector.Summary()
		want := int64(count)*chunkBytes - retainedBytes
		if summary.DroppedLatestPlanTextBytes != want {
			t.Errorf("%s: after %d chunks dropped %d bytes, want %d", runtime.GOARCH, count, summary.DroppedLatestPlanTextBytes, want)
		}
		if summary.LatestPlanText == nil || *summary.LatestPlanText != delta[chunkBytes-retainedBytes:] {
			t.Fatal("latest plan did not retain the expected suffix")
		}
	}
	// Each component of the plan identity owns a separate accumulation.
	for _, scope := range [][3]string{{"other", "turn", "plan"}, {"other", "next", "plan"}, {"other", "next", "new"}} {
		collector.Process(&codex.PlanDelta{ThreadID: scope[0], TurnID: scope[1], ItemID: scope[2], Delta: delta[chunkBytes-retainedBytes-1:]}, nil)
		summary := collector.Summary()
		if summary.DroppedLatestPlanTextBytes != 1 || summary.LatestPlanItemID == nil || *summary.LatestPlanItemID != scope[2] || summary.LatestPlanText == nil || *summary.LatestPlanText != delta[chunkBytes-retainedBytes:] {
			t.Fatalf("plan identity %q did not reset text and byte accounting", scope)
		}
	}
	// A completed plan also replaces text within the same identity.
	collector.Process(&codex.ItemCompleted{ThreadID: "other", TurnID: "next", Item: codex.ThreadItemWrapper{Value: &codex.PlanThreadItem{ID: "new", Text: "complete"}}}, nil)
	summary := collector.Summary()
	if summary.DroppedLatestPlanTextBytes != 0 || summary.LatestPlanText == nil || *summary.LatestPlanText != "complete" {
		t.Fatal("completed plan did not reset the accumulated text and count")
	}
}
