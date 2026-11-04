package appserver

import (
	"strings"
	"testing"
)

func TestCollectorCommandByteCounterOwner(t *testing.T) {
	const chunkBytes = 8 << 20
	const retainedBytes = 64 << 10
	delta := strings.Repeat("x", chunkBytes-4) + "tail"
	collector := NewStreamCollector()
	for count := 1; count <= 257; count++ {
		collector.processCommandExecutionOutputDelta(CommandExecutionOutputDeltaNotification{ThreadID: "thread", TurnID: "turn", ItemID: "command", Delta: delta})
		if count < 256 {
			continue
		}
		command := collector.Summary().CommandExecutions["command"]
		if want := int64(count)*chunkBytes - retainedBytes; command.DroppedOutputDeltaBytes != want {
			t.Errorf("after %d chunks dropped %d bytes, want %d", count, command.DroppedOutputDeltaBytes, want)
		}
		if len(command.OutputDeltas) != 1 || command.OutputDeltas[0] != delta[chunkBytes-retainedBytes:] || command.DroppedOutputDeltas != count-1 {
			t.Fatal("command did not retain the expected suffix and delta count")
		}
	}
	collector.Process(&ItemCompleted{ThreadID: "thread", TurnID: "turn", Item: ThreadItemWrapper{Value: &CommandExecutionThreadItem{ID: "command", Status: CommandExecutionStatusCompleted}}}, nil)
	command := collector.Summary().CommandExecutions["command"]
	if !command.Completed || command.DroppedOutputDeltaBytes != int64(257)*chunkBytes-retainedBytes || command.DroppedOutputDeltas != 256 {
		t.Fatal("completion changed the accumulated accounting")
	}
}
