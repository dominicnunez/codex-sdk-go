package protocol

import (
	"fmt"
	"testing"
)

// BenchmarkThreadState measures metadata updates and full snapshot replacement.
// Listener callbacks retain their snapshots, as a Conversation does.
func BenchmarkThreadState(b *testing.B) {
	for _, turns := range []int{0, 100, 1000} {
		for _, listeners := range []int{0, 1, 4} {
			for _, operation := range []string{"metadata", "replace"} {
				b.Run(fmt.Sprintf("turns=%d/listeners=%d/%s", turns, listeners, operation), func(b *testing.B) {
					c := &Client{}
					thread := Thread{ID: "benchmark", Turns: make([]Turn, turns)}
					for i := range thread.Turns {
						thread.Turns[i] = Turn{ID: fmt.Sprint(i), Items: []ThreadItemWrapper{{Value: &AgentMessageThreadItem{Text: "A representative response retained in thread history."}}}}
					}
					c.cacheThreadState(thread)
					snapshots := make([]Thread, listeners)
					for i := range listeners {
						c.addThreadStateListener(thread.ID, func(t Thread) { snapshots[i] = t }, nil)
					}
					name := "renamed"
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if operation == "metadata" {
							c.mutateThreadState(thread.ID, func(t *Thread) { t.Name = cloneStringPtr(&name) })
						} else {
							c.cacheThreadState(thread)
						}
					}
				})
			}
		}
	}
}
