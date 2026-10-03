package appserver

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCollectorOpaqueIdentity(t *testing.T) {
	cases := map[string][][3]string{
		"internal separator":     {{"a\x1fb", "c", "same"}, {"a", "b\x1fc", "same"}},
		"presentation separator": {{"a/b", "c", "same"}, {"a", "b/c", "same"}},
		"bare versus internal":   {{"", "", "a\x1fb\x1fc"}, {"a", "b", "c"}},
		"bare versus scoped":     {{"a", "b", "same"}, {"x", "y", "same"}, {"", "", "a/b/same"}},
		"empty scopes":           {{"", "", "same"}, {"", "turn", "same"}, {"thread", "", "same"}},
		"empty item":             {{"", "", ""}, {"/", "", ""}, {"", "/", ""}},
		"reserved fallbacks":     {{"a/b", "c", "same"}, {"a", "b/c", "same"}, {"", "", "@3:a/b1:c4:same"}, {"", "", "@@3:a/b1:c4:same"}},
	}
	for name, scopes := range cases {
		t.Run(name, func(t *testing.T) {
			c := NewStreamCollector()
			for i, scope := range scopes {
				for _, item := range []ThreadItem{
					&CommandExecutionThreadItem{ID: scope[2], Command: fmt.Sprint(i), Status: CommandExecutionStatusCompleted, AggregatedOutput: Ptr(fmt.Sprint(i))},
					&McpToolCallThreadItem{ID: scope[2], Tool: fmt.Sprint(i), Status: McpToolCallStatusCompleted},
					&WebSearchThreadItem{ID: scope[2], Query: fmt.Sprint(i)},
					&FileChangeThreadItem{ID: scope[2], Status: PatchApplyStatusCompleted, Changes: []FileUpdateChange{{Path: fmt.Sprint(i)}}},
				} {
					c.Process(&ItemStarted{ThreadID: scope[0], TurnID: scope[1], Item: ThreadItemWrapper{Value: item}}, nil)
					c.Process(&ItemCompleted{ThreadID: scope[0], TurnID: scope[1], Item: ThreadItemWrapper{Value: item}}, nil)
				}
			}
			summary := c.Summary()
			for family, values := range map[string]reflect.Value{"command": reflect.ValueOf(summary.CommandExecutions), "mcp": reflect.ValueOf(summary.McpToolCalls), "web": reflect.ValueOf(summary.WebSearches), "file": reflect.ValueOf(summary.FileChanges)} {
				if values.Len() != len(scopes) {
					t.Errorf("%s records = %d, want %d", family, values.Len(), len(scopes))
					continue
				}
				seen := make(map[[3]string]bool)
				for _, key := range values.MapKeys() {
					v := values.MapIndex(key)
					scope := [3]string{v.FieldByName("ThreadID").String(), v.FieldByName("TurnID").String(), v.FieldByName("ItemID").String()}
					seen[scope] = true
					if !v.FieldByName("Started").Bool() || !v.FieldByName("Completed").Bool() {
						t.Errorf("%s scope %q lost lifecycle status", family, scope)
					}
					for i, expected := range scopes {
						if scope != expected {
							continue
						}
						var marker string
						switch family {
						case "command":
							marker = v.FieldByName("AggregatedOutput").String()
						case "mcp":
							marker = v.FieldByName("CompletedItem").Elem().FieldByName("Tool").String()
						case "web":
							marker = v.FieldByName("CompletedItem").Elem().FieldByName("Query").String()
						case "file":
							marker = v.FieldByName("CompletedItem").Elem().FieldByName("Changes").Index(0).FieldByName("Path").String()
						}
						if marker != fmt.Sprint(i) {
							t.Errorf("%s scope %q metadata mixed: %q", family, scope, marker)
						}
					}
				}
				for _, scope := range scopes {
					if !seen[scope] {
						t.Errorf("%s missing scope %q", family, scope)
					}
				}
			}
		})
	}
}

func TestCollectorCommandIdentityOwnership(t *testing.T) {
	c := NewStreamCollector()
	scopes := [][3]string{{"a\x1fb", "c", "same"}, {"a", "b\x1fc", "same"}, {"", "", "a\x1fb\x1fc\x1fsame"}}
	for i, scope := range scopes {
		delta := fmt.Sprint(i)
		if i == 0 {
			delta = strings.Repeat("X", streamCollectorOutputDeltaBytesLimit+1)
		}
		c.processCommandExecutionOutputDelta(CommandExecutionOutputDeltaNotification{ThreadID: scope[0], TurnID: scope[1], ItemID: scope[2], Delta: delta})
	}
	complete := func(scope [3]string) {
		c.Process(&ItemCompleted{ThreadID: scope[0], TurnID: scope[1], Item: ThreadItemWrapper{Value: &CommandExecutionThreadItem{ID: scope[2], Status: CommandExecutionStatusCompleted}}}, nil)
	}
	complete(scopes[0])
	partial := c.Summary()
	if len(partial.CommandExecutions) != len(scopes) {
		t.Fatalf("partial commands = %d", len(partial.CommandExecutions))
	}
	for _, v := range partial.CommandExecutions {
		switch v.ThreadID {
		case scopes[0][0]:
			if !v.Completed || v.Status == nil || *v.Status != CommandExecutionStatusCompleted || v.DroppedOutputDeltaBytes != 1 || len(v.AggregatedOutput) != streamCollectorOutputDeltaBytesLimit+1 {
				t.Fatalf("first command state mixed: %+v", v)
			}
		default:
			want := "1"
			if v.ThreadID == "" {
				want = "2"
			}
			if v.Completed || v.DroppedOutputDeltaBytes != 0 || v.AggregatedOutput != want {
				t.Fatalf("pending command state mixed: %+v", v)
			}
		}
	}
	complete(scopes[1])
	complete(scopes[2])
	for _, v := range c.Summary().CommandExecutions {
		if !v.Completed || v.AggregatedOutput == "" {
			t.Fatal("completion cleared another command's chunks")
		}
	}
	if len(c.commandOutputChunks) != 0 || len(c.commandOutputBytes) != 0 || len(c.commandOutputDeltaBytes) != 0 {
		t.Fatal("completed command auxiliary ownership remains")
	}
	snapshot := c.Summary()
	for key, v := range snapshot.CommandExecutions {
		v.OutputDeltas[0] = "mutated"
		v.CompletedItem.Command = "mutated"
		snapshot.CommandExecutions[key] = v
	}
	for _, v := range c.Summary().CommandExecutions {
		if v.OutputDeltas[0] == "mutated" || v.CompletedItem.Command == "mutated" {
			t.Fatal("summary aliases collector")
		}
	}
}

func TestCollectorPlanScope(t *testing.T) {
	c := NewStreamCollector()
	c.Process(&PlanDelta{ThreadID: "first", TurnID: "turn", ItemID: "same", Delta: strings.Repeat("A", streamCollectorPlanTextBytesLimit+1)}, nil)
	c.Process(&PlanDelta{ThreadID: "second", TurnID: "turn", ItemID: "same", Delta: "B"}, nil)
	assertPlan := func(want string) {
		t.Helper()
		summary := c.Summary()
		if summary.LatestPlanText == nil || *summary.LatestPlanText != want || summary.LatestPlanItemID == nil || *summary.LatestPlanItemID != "same" || summary.DroppedLatestPlanTextBytes != 0 {
			t.Fatalf("plan mixed across scopes: %+v", summary)
		}
	}
	assertPlan("B")
	c.Process(&PlanDelta{ThreadID: "second", TurnID: "other", ItemID: "same", Delta: "C"}, nil)
	assertPlan("C")
	c.Process(&PlanDelta{ThreadID: "second", TurnID: "other", ItemID: "same", Delta: "D"}, nil)
	assertPlan("CD")
	c.Process(&ItemCompleted{ThreadID: "completed", TurnID: "", Item: ThreadItemWrapper{Value: &PlanThreadItem{ID: "same", Text: "E"}}}, nil)
	c.Process(&PlanDelta{ThreadID: "completed", ItemID: "same", Delta: "F"}, nil)
	assertPlan("EF")
	c.Process(&PlanDelta{ItemID: "same", Delta: "G"}, nil)
	assertPlan("G")
}

func FuzzCollectorIdentity(f *testing.F) {
	f.Add("a/b", "c", "same", "other")
	f.Add("a\x1fb", "c", "same", "a\x1fb\x1fc\x1fsame")
	f.Add("", "", "", "/")
	f.Add("界", "🙂", "@3:界4:🙂1:x", "x")
	f.Fuzz(func(t *testing.T, a, b, item, other string) {
		if len(a)+len(b)+len(item)+len(other) > 2048 {
			t.Skip()
		}
		scopes := [][3]string{{a, b, item}, {b, a, item}, {a, b, other}, {"", "", a + "/" + b + "/" + item}}
		expected := make(map[[3]string]string)
		for i, scope := range scopes {
			expected[scope] = fmt.Sprint(i)
		}
		build := func(reverse bool) StreamSummary {
			c := NewStreamCollector()
			for n := range len(scopes) {
				i := n
				if reverse {
					i = len(scopes) - 1 - n
				}
				scope := scopes[i]
				c.Process(&ItemCompleted{ThreadID: scope[0], TurnID: scope[1], Item: ThreadItemWrapper{Value: &CommandExecutionThreadItem{ID: scope[2], Status: CommandExecutionStatusCompleted, AggregatedOutput: Ptr(expected[scope])}}}, nil)
			}
			first, second := c.Summary(), c.Summary()
			if !reflect.DeepEqual(first, second) {
				t.Fatal("repeated summary changes identity assignment")
			}
			return first
		}
		forward, reverse := build(false), build(true)
		if !reflect.DeepEqual(forward, reverse) {
			t.Fatal("insertion order changes identity assignment")
		}
		if len(forward.CommandExecutions) != len(expected) {
			t.Fatalf("records = %d, want %d", len(forward.CommandExecutions), len(expected))
		}
		counts := make(map[string]int)
		for scope := range expected {
			counts[scope[2]]++
		}
		seen := make(map[[3]string]bool)
		for key, value := range forward.CommandExecutions {
			scope := [3]string{value.ThreadID, value.TurnID, value.ItemID}
			want, ok := expected[scope]
			if !ok || seen[scope] || value.AggregatedOutput != want || !value.Completed || value.Status == nil || *value.Status != CommandExecutionStatusCompleted {
				t.Fatalf("state not preserved for %q", scope)
			}
			seen[scope] = true
			if counts[scope[2]] == 1 && key != scope[2] {
				t.Fatalf("unique bare key changed: %q -> %q", scope[2], key)
			}
		}
	})
}

func BenchmarkCollectorIdentity(b *testing.B) {
	for _, records := range []int{1, 128, 1024} {
		for _, duplicate := range []bool{false, true} {
			b.Run(fmt.Sprintf("records%d/duplicate%v", records, duplicate), func(b *testing.B) {
				events := make([]*ItemCompleted, records)
				for i := range events {
					id := fmt.Sprint(i)
					if duplicate {
						id = "same"
					}
					events[i] = &ItemCompleted{ThreadID: fmt.Sprint(i), TurnID: "turn", Item: ThreadItemWrapper{Value: &CommandExecutionThreadItem{ID: id, Status: CommandExecutionStatusCompleted}}}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					c := NewStreamCollector()
					for _, event := range events {
						c.Process(event, nil)
					}
					c.Summary()
				}
			})
		}
	}
}
