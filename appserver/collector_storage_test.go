package appserver

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"
)

func assertOwnedTrimmedString(t *testing.T, source, retained string) {
	t.Helper()
	if len(retained) == 0 {
		return
	}
	start := uintptr(unsafe.Pointer(unsafe.StringData(source)))
	address := uintptr(unsafe.Pointer(unsafe.StringData(retained)))
	if address >= start && address < start+uintptr(len(source)) {
		t.Fatal("trimmed suffix retains source backing storage")
	}
}

func TestCollectorTrimmedSuffixOwnsStorage(t *testing.T) {
	for _, size := range []int{streamCollectorPlanTextBytesLimit + 1, 1 << 20, 8 << 20} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			input := strings.Repeat("x", size)
			retained, dropped := retainSuffixWithinByteLimit(input, streamCollectorPlanTextBytesLimit)
			if len(retained) != streamCollectorPlanTextBytesLimit || dropped != size-len(retained) {
				t.Fatal("suffix accounting changed")
			}
			assertOwnedTrimmedString(t, input, retained)
			collector := NewStreamCollector()
			collector.Process(&PlanDelta{ItemID: "plan", Delta: input}, nil)
			assertOwnedTrimmedString(t, input, *collector.Summary().LatestPlanText)
			collector.Process(&ItemCompleted{Item: ThreadItemWrapper{Value: &PlanThreadItem{ID: "plan", Text: input}}}, nil)
			assertOwnedTrimmedString(t, input, *collector.Summary().LatestPlanText)
			collector.processCommandExecutionOutputDelta(CommandExecutionOutputDeltaNotification{ItemID: "command", Delta: input})
			command := collector.Summary().CommandExecutions["command"]
			if len(command.OutputDeltas) != 1 || command.DroppedOutputDeltaBytes != size-streamCollectorOutputDeltaBytesLimit {
				t.Fatal("command retention accounting changed")
			}
			assertOwnedTrimmedString(t, input, command.OutputDeltas[0])
		})
	}
}

func TestCollectorDiscardedHistorySlotsAreCleared(t *testing.T) {
	for _, byCount := range []bool{true, false} {
		t.Run(fmt.Sprint("append count=", byCount), func(t *testing.T) {
			backing := []string{"111", "22", ""}
			entries, bytes, droppedEntries, droppedBytes := 2, 3, 1, 3
			maxBytes, maxEntries := 3, 10
			if byCount {
				maxBytes, maxEntries = 100, 2
			}
			retained, total, dropped, droppedSize := appendBoundedStringHistory(backing[:2], 5, "4", 0, 0, maxEntries, maxBytes)
			if !reflect.DeepEqual(retained, []string{"22", "4"}) || total != bytes || len(retained) != entries || dropped != droppedEntries || droppedSize != droppedBytes {
				t.Fatal("append history semantics changed")
			}
			if backing[0] != "" {
				t.Fatal("discarded delta remains in backing slot")
			}
		})
		t.Run(fmt.Sprint("raw count=", byCount), func(t *testing.T) {
			backing := []string{"111", "22", "3"}
			maxBytes, maxChunks := 3, 10
			if byCount {
				maxBytes, maxChunks = 100, 2
			}
			retained, total := trimBoundedStringHistory(backing, 6, maxChunks, maxBytes)
			if !reflect.DeepEqual(retained, []string{"22", "3"}) || total != 3 {
				t.Fatal("raw history semantics changed")
			}
			if backing[0] != "" {
				t.Fatal("discarded raw chunk remains in backing slot")
			}
		})
	}
	backing := []string{"111", "22", "33"}
	retained, total := trimBoundedStringHistory(backing, 7, 10, 1)
	if len(retained) != 0 || total != 0 || !reflect.DeepEqual(backing, []string{"", "", ""}) {
		t.Fatal("fully discarded history retains backing references")
	}
}

func FuzzCollectorSuffixRetention(f *testing.F) {
	for _, seed := range []string{"", "ascii", "a😀b", "汉字日本語", strings.Repeat("x", 4096)} {
		f.Add(seed, uint16(4))
	}
	f.Fuzz(func(t *testing.T, input string, limit uint16) {
		if len(input) > 65536 {
			return
		}
		input = strings.ToValidUTF8(input, "�")
		maxBytes := int(limit)
		// Independent reference walks complete runes backward rather than
		// choosing a byte offset and advancing past continuation bytes.
		start := len(input)
		used := 0
		for start > 0 {
			_, size := utf8.DecodeLastRuneInString(input[:start])
			if used+size > maxBytes {
				break
			}
			used += size
			start -= size
		}
		retained, dropped := retainSuffixWithinByteLimit(input, maxBytes)
		if retained != input[start:] || dropped != start || !utf8.ValidString(retained) {
			t.Fatalf("suffix=%q/%d expected=%q/%d", retained, dropped, input[start:], start)
		}
		if len(input) > maxBytes {
			assertOwnedTrimmedString(t, input, retained)
		}
	})
}

func BenchmarkCollectorStorage(b *testing.B) {
	for _, size := range []int{1024, 64 << 10, 8 << 20} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			input := strings.Repeat("x", size)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				collector := NewStreamCollector()
				collector.Process(&PlanDelta{ItemID: "plan", Delta: input}, nil)
				if collector.Summary().LatestPlanText == nil {
					b.Fatal("missing plan")
				}
			}
		})
	}
}
