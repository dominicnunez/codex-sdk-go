package appserver

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Every exported field is populated independently of the production copier.
// Interface variants are explicit so this exercises each supported reference
// shape, including arbitrary metadata, rather than only the reported fields.
func ownershipItems(mode int) []ThreadItemWrapper {
	items := []ThreadItem{
		&UserMessageThreadItem{Content: []UserInput{&TextUserInput{}, &ImageUserInput{}, &LocalImageUserInput{}, &protocol.AudioUserInput{}, &protocol.LocalAudioUserInput{}, &SkillUserInput{}, &MentionUserInput{}, &UnknownUserInput{}}},
		&AgentMessageThreadItem{}, &PlanThreadItem{}, &ReasoningThreadItem{},
		&CommandExecutionThreadItem{CommandActions: []CommandActionWrapper{{Value: &ReadCommandAction{}}, {Value: &ListFilesCommandAction{}}, {Value: &SearchCommandAction{}}, {Value: &UnknownCommandAction{}}}},
		&FileChangeThreadItem{Changes: []FileUpdateChange{{Kind: PatchChangeKindWrapper{Value: &AddPatchChangeKind{}}}, {Kind: PatchChangeKindWrapper{Value: &DeletePatchChangeKind{}}}, {Kind: PatchChangeKindWrapper{Value: &UpdatePatchChangeKind{}}}, {Kind: PatchChangeKindWrapper{Value: &UnknownPatchChangeKind{}}}}},
		&McpToolCallThreadItem{Arguments: map[string]any{"key": []any{map[string]any{"key": "original"}}}, Result: &McpToolCallResult{Content: []any{map[string]any{"key": []string{"original"}}}, StructuredContent: map[string]any{"key": []byte{1, 2}}}},
		&DynamicToolCallThreadItem{Arguments: map[string]any{"key": []string{"original"}}, ContentItems: []DynamicToolCallOutputContentItemWrapper{{Value: &InputTextDynamicToolCallOutputContentItem{}}, {Value: &InputImageDynamicToolCallOutputContentItem{}}, {Value: &protocol.InputAudioDynamicToolCallOutputContentItem{}}, {Value: &UnknownDynamicToolCallOutputContentItem{}}}},
		&CollabAgentToolCallThreadItem{},
		&WebSearchThreadItem{Action: &WebSearchActionWrapper{Value: &SearchWebSearchAction{}}},
		&WebSearchThreadItem{Action: &WebSearchActionWrapper{Value: &OpenPageWebSearchAction{}}},
		&WebSearchThreadItem{Action: &WebSearchActionWrapper{Value: &FindInPageWebSearchAction{}}},
		&WebSearchThreadItem{Action: &WebSearchActionWrapper{Value: &OtherWebSearchAction{}}},
		&WebSearchThreadItem{Action: &WebSearchActionWrapper{Value: &UnknownWebSearchAction{}}},
		&protocol.FunctionCallOutputThreadItem{Output: protocol.FunctionCallOutputBody{Content: []protocol.FunctionCallOutputContentItemWrapper{{Value: &protocol.InputTextFunctionCallOutputContentItem{}}, {Value: &protocol.InputImageFunctionCallOutputContentItem{}}, {Value: &protocol.InputAudioFunctionCallOutputContentItem{}}, {Value: &protocol.EncryptedContentFunctionCallOutputContentItem{}}, {Value: &protocol.UnknownFunctionCallOutputContentItem{}}}}},
		&ImageViewThreadItem{}, &EnteredReviewModeThreadItem{}, &ExitedReviewModeThreadItem{}, &ContextCompactionThreadItem{},
		&protocol.SubAgentActivityThreadItem{}, &protocol.SleepThreadItem{}, &UnknownThreadItem{},
	}
	wrappers := make([]ThreadItemWrapper, len(items))
	for i, item := range items {
		populateOwnershipValue(reflect.ValueOf(item).Elem(), mode)
		wrappers[i].Value = item
	}
	return wrappers
}

// mode0 populates references; mode1 uses present empty slices/maps; mode2 uses
// nil references. This does not serialize, normalize, validate or copy a scope.
func populateOwnershipValue(v reflect.Value, mode int) {
	switch v.Kind() {
	case reflect.Pointer:
		if mode == 2 {
			v.SetZero()
			return
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		populateOwnershipValue(v.Elem(), mode)
	case reflect.Interface:
		if mode == 2 {
			v.SetZero()
			return
		}
		if v.IsNil() {
			return
		}
		child := reflect.New(v.Elem().Type()).Elem()
		child.Set(v.Elem())
		populateOwnershipValue(child, mode)
		v.Set(child)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			populateOwnershipValue(v.Field(i), mode)
		}
	case reflect.Slice:
		if mode == 2 {
			v.SetZero()
			return
		}
		if mode == 1 {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
			return
		}
		if v.IsNil() || v.Len() == 0 {
			v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		}
		for i := 0; i < v.Len(); i++ {
			populateOwnershipValue(v.Index(i), mode)
		}
	case reflect.Map:
		if mode == 2 {
			v.SetZero()
			return
		}
		if mode == 1 {
			v.Set(reflect.MakeMap(v.Type()))
			return
		}
		if v.IsNil() || v.Len() == 0 {
			v.Set(reflect.MakeMap(v.Type()))
			key := reflect.New(v.Type().Key()).Elem()
			populateOwnershipValue(key, 0)
			value := reflect.New(v.Type().Elem()).Elem()
			populateOwnershipValue(value, 0)
			v.SetMapIndex(key, value)
		}
		for _, key := range v.MapKeys() {
			value := reflect.New(v.Type().Elem()).Elem()
			value.Set(v.MapIndex(key))
			populateOwnershipValue(value, mode)
			v.SetMapIndex(key, value)
		}
	case reflect.String:
		v.SetString("original")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	}
}

func mutateOwnershipValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			mutateOwnershipValue(v.Elem())
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		child := reflect.New(v.Elem().Type()).Elem()
		child.Set(v.Elem())
		mutateOwnershipValue(child)
		if v.CanSet() {
			v.Set(child)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			mutateOwnershipValue(v.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			mutateOwnershipValue(v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			child := reflect.New(v.Type().Elem()).Elem()
			child.Set(v.MapIndex(key))
			mutateOwnershipValue(child)
			v.SetMapIndex(key, child)
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString("mutated")
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.CanSet() {
			v.SetInt(v.Int() + 1)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.CanSet() {
			v.SetUint(v.Uint() + 1)
		}
	}
}

func TestEveryItemSnapshotOwnsReferences(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		for i, item := range ownershipItems(mode) {
			for _, valueForm := range []bool{false, true} {
				source := ownershipItems(mode)[i]
				want := ownershipItems(mode)[i]
				if valueForm {
					source.Value = reflect.ValueOf(source.Value).Elem().Interface().(ThreadItem)
					want.Value = reflect.ValueOf(want.Value).Elem().Interface().(ThreadItem)
				}
				snapshot := cloneThreadItemWrapper(source)
				if !reflect.DeepEqual(snapshot, want) {
					t.Fatalf("%T mode%d value%v changed shape", item.Value, mode, valueForm)
				}
				mutateOwnershipValue(reflect.ValueOf(&source).Elem())
				if !reflect.DeepEqual(snapshot, want) {
					t.Fatalf("%T mode%d value%v source mutation leaked", item.Value, mode, valueForm)
				}
				fresh := ownershipItems(mode)[i]
				cloned := cloneThreadItemWrapper(fresh)
				mutateOwnershipValue(reflect.ValueOf(&cloned).Elem())
				if !reflect.DeepEqual(fresh, ownershipItems(mode)[i]) {
					t.Fatalf("%T snapshot mutation leaked", item.Value)
				}
			}
		}
	}
	for _, item := range []ThreadItem{nil, (*CommandExecutionThreadItem)(nil), (*McpToolCallThreadItem)(nil)} {
		if !reflect.DeepEqual(cloneThreadItemWrapper(ThreadItemWrapper{Value: item}), ThreadItemWrapper{Value: item}) {
			t.Fatal("nil union shape changed")
		}
	}
}

func TestCollectorEveryRetainedReference(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		for i, item := range ownershipItems(mode) {
			switch item.Value.(type) {
			case *CommandExecutionThreadItem, *McpToolCallThreadItem, *WebSearchThreadItem, *FileChangeThreadItem:
			default:
				continue
			}
			for _, completed := range []bool{false, true} {
				for _, valueForm := range []bool{false, true} {
					source := ownershipItems(mode)[i]
					want := ownershipItems(mode)[i].Value
					if valueForm {
						source.Value = reflect.ValueOf(source.Value).Elem().Interface().(ThreadItem)
					}
					collector := NewStreamCollector()
					if completed {
						collector.Process(&ItemCompleted{Item: source}, nil)
					} else {
						collector.Process(&ItemStarted{Item: source}, nil)
					}
					get := func() ThreadItem {
						summary := collector.Summary()
						switch item.Value.(type) {
						case *CommandExecutionThreadItem:
							state := summary.CommandExecutions["original"]
							if completed {
								return state.CompletedItem
							}
							return state.StartedItem
						case *McpToolCallThreadItem:
							state := summary.McpToolCalls["original"]
							if completed {
								return state.CompletedItem
							}
							return state.StartedItem
						case *WebSearchThreadItem:
							state := summary.WebSearches["original"]
							if completed {
								return state.CompletedItem
							}
							return state.StartedItem
						case *FileChangeThreadItem:
							state := summary.FileChanges["original"]
							if completed {
								return state.CompletedItem
							}
							return state.StartedItem
						default:
							panic("unreachable")
						}
					}
					mutateOwnershipValue(reflect.ValueOf(&source).Elem())
					first := get()
					if !reflect.DeepEqual(first, want) {
						t.Fatalf("%T mode%d completed%v value%v input alias", item.Value, mode, completed, valueForm)
					}
					mutateOwnershipValue(reflect.ValueOf(first))
					if !reflect.DeepEqual(get(), want) {
						t.Fatalf("%T summary alias", item.Value)
					}
				}
			}
		}
	}
	collector := NewStreamCollector()
	collector.Process((*ItemCompleted)(nil), nil)
	collector.Process(&ItemCompleted{Item: ThreadItemWrapper{Value: (*CommandExecutionThreadItem)(nil)}}, nil)
	collector.Process(&ItemStarted{Item: ThreadItemWrapper{Value: (*McpToolCallThreadItem)(nil)}}, nil)
	if len(collector.Summary().CommandExecutions) != 0 {
		t.Fatal("nil item admitted")
	}
}

func TestRunResultOwnsAllItemAndTurnReferences(t *testing.T) {
	for mode := 0; mode < 3; mode++ {
		items := ownershipItems(mode)
		started, completed, duration := int64(1), int64(2), int64(3)
		turn := Turn{ID: "turn", Items: items, StartedAt: &started, CompletedAt: &completed, DurationMs: &duration}
		thread := Thread{ID: "thread", Turns: []Turn{{ID: "old", Items: items}}}
		result := buildRunResult(thread, turn, items)
		mutateOwnershipValue(reflect.ValueOf(&items).Elem())
		started, completed, duration = 99, 99, 99
		want := ownershipItems(mode)
		for _, view := range [][]ThreadItemWrapper{result.Items, result.Turn.Items, result.Thread.Turns[0].Items, result.Thread.Turns[1].Items} {
			if !reflect.DeepEqual(view, want) {
				t.Fatalf("mode%d result retained input aliases", mode)
			}
		}
		if *result.Turn.StartedAt != 1 || *result.Turn.CompletedAt != 2 || *result.Turn.DurationMs != 3 {
			t.Fatal("turn metadata retained input pointers")
		}
		mutateOwnershipValue(reflect.ValueOf(&result.Items).Elem())
		if !reflect.DeepEqual(result.Turn.Items, want) || !reflect.DeepEqual(result.Thread.Turns[1].Items, want) {
			t.Fatal("result Items mutation reached sibling views")
		}
		mutateOwnershipValue(reflect.ValueOf(&result.Turn).Elem())
		if !reflect.DeepEqual(result.Thread.Turns[1].Items, want) || *result.Thread.Turns[1].StartedAt != 1 {
			t.Fatal("result Turn mutation reached Thread")
		}
	}
}

func TestOptionalSliceAndNestedUnionOwnership(t *testing.T) {
	var nilResults []json.RawMessage
	var nilQueries []string
	var nilQuestions []protocol.AsyncUserInputQuestion
	items := []ThreadItemWrapper{
		{Value: &WebSearchThreadItem{Results: &nilResults, Action: &WebSearchActionWrapper{Value: &SearchWebSearchAction{Queries: &nilQueries}}}},
		{Value: &AgentMessageThreadItem{Questions: &nilQuestions}},
	}
	cloned := cloneThreadItems(items)
	if !reflect.DeepEqual(cloned, items) {
		t.Fatal("pointer to nil slice changed shape")
	}
	nilResults = append(nilResults, json.RawMessage(`{"value":1}`))
	nilQueries = append(nilQueries, "query")
	nilQuestions = append(nilQuestions, protocol.AsyncUserInputQuestion{Title: "question"})
	web := cloned[0].Value.(*WebSearchThreadItem)
	if *web.Results != nil || *web.Action.Value.(*SearchWebSearchAction).Queries != nil || *cloned[1].Value.(*AgentMessageThreadItem).Questions != nil {
		t.Fatal("nil inner slice pointer retained alias")
	}
	for _, source := range []ThreadItemWrapper{
		{Value: &FileChangeThreadItem{Changes: []FileUpdateChange{{Kind: PatchChangeKindWrapper{Value: UnknownPatchChangeKind{Type: "future", Raw: json.RawMessage(`{"extra":[1]}`)}}}}}},
		{Value: &WebSearchThreadItem{Action: &WebSearchActionWrapper{Value: UnknownWebSearchAction{Type: "future", Raw: json.RawMessage(`{"extra":[1]}`)}}}},
	} {
		cloned := cloneThreadItemWrapper(source)
		if !reflect.DeepEqual(source, cloned) {
			t.Fatal("nested value union lost type or raw metadata")
		}
		mutateOwnershipValue(reflect.ValueOf(&cloned).Elem())
		if reflect.DeepEqual(source, cloned) {
			t.Fatal("mutation fixture did not change snapshot")
		}
		// Both source raw buffers must retain their original spelling.
		switch value := source.Value.(type) {
		case *FileChangeThreadItem:
			if string(value.Changes[0].Kind.Value.(UnknownPatchChangeKind).Raw) != `{"extra":[1]}` {
				t.Fatal("patch raw buffer aliased")
			}
		case *WebSearchThreadItem:
			if string(value.Action.Value.(UnknownWebSearchAction).Raw) != `{"extra":[1]}` {
				t.Fatal("web raw buffer aliased")
			}
		}
	}
}
