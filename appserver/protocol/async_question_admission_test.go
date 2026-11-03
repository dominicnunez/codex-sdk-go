package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAsyncQuestionClosedShape(t *testing.T) {
	for _, tc := range []struct {
		name, question string
		valid          bool
	}{
		{"empty-title", `{"title":""}`, true}, {"nullable-options", `{"title":"Q","options":null}`, true}, {"empty-options", `{"title":"Q","options":[]}`, true}, {"populated-options", `{"title":"Q","options":["","a"]}`, true}, {"duplicate-title", `{"title":"Q","title":"last"}`, true},
		{"escaped-title", `{"tit\u006ce":"Q","opt\u0069ons":[""]}`, true},
		{"escaped-extra", `{"title":"Q","ex\u0074ra":false}`, false},
		{"capital-extra", `{"title":"Q","Options":[]}`, false},
		{"empty-key", `{"title":"Q","":null}`, false},
		{"extra-before-title", `{"extra":1e10000,"title":"Q"}`, false},
		{"extra-bool", `{"title":"Q","extra":true}`, false}, {"extra-null", `{"title":"Q","options":[],"extra":null}`, false}, {"extra-object", `{"title":"Q","options":[],"extra":{"nested":[]}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var direct p.AsyncUserInputQuestion
			directErr := json.Unmarshal([]byte(tc.question), &direct)
			if tc.valid && directErr != nil {
				t.Fatalf("valid question rejected: %v", directErr)
			}
			if !tc.valid && directErr == nil {
				t.Error("closed question accepted extra property directly")
			}
			item := json.RawMessage(`{"type":"agentMessage","id":"i","text":"text","outerExtension":true,"questions":[` + tc.question + `]}`)
			thread := validProcessThreadPayload("t")
			thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []any{item}}}
			mock := NewMockTransport()
			diagnostics := 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { diagnostics++ }))
			t.Cleanup(func() { _ = client.Close() })
			if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
				t.Fatal(err)
			}
			_, readErr := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t", IncludeTurns: p.Ptr(true)})
			if tc.valid && readErr != nil {
				t.Fatalf("valid public read failed: %v", readErr)
			}
			if !tc.valid && readErr == nil {
				t.Error("closed question published through Thread.Read")
			}
			startedOn, startedAdd, completedOn, completedAdd := 0, 0, 0, 0
			client.OnItemStarted(func(p.ItemStartedNotification) { startedOn++ })
			removeStarted := client.AddItemStartedListener(func(p.ItemStartedNotification) { startedAdd++ })
			defer removeStarted()
			client.OnItemCompleted(func(p.ItemCompletedNotification) { completedOn++ })
			removeCompleted := client.AddItemCompletedListener(func(p.ItemCompletedNotification) { completedAdd++ })
			defer removeCompleted()
			for _, phase := range []string{"started", "completed"} {
				stamp := "startedAtMs"
				if phase == "completed" {
					stamp = "completedAtMs"
				}
				raw := json.RawMessage(fmt.Sprintf(`{"%s":1,"threadId":"t","turnId":"u","item":%s}`, stamp, item))
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "item/" + phase, Params: raw})
			}
			want := 0
			if tc.valid {
				want = 1
			}
			if startedOn != want || startedAdd != want || completedOn != want || completedAdd != want {
				t.Errorf("publication counts started On=%d Add=%d completed On=%d Add=%d want=%d diagnostics=%d", startedOn, startedAdd, completedOn, completedAdd, want, diagnostics)
			}
			if !tc.valid && diagnostics == 0 {
				t.Fatal("invalid notification did not report admission failure")
			}
			if tc.valid && diagnostics != 0 {
				t.Fatal("valid question reported diagnostics")
			}
		})
	}
}

func TestAsyncQuestionReceiverAndPriorErrors(t *testing.T) {
	options := []string{"prior"}
	value := p.AsyncUserInputQuestion{Title: "prior", Options: &options}
	before := value
	for _, raw := range []string{`{"title":"new","options":[],"extra":true}`, `{"title":"new","options":null,"Title":"extra"}`} {
		if err := value.UnmarshalJSON([]byte(raw)); err == nil {
			t.Fatal("closed question accepted")
		}
		if !reflect.DeepEqual(value, before) || value.Options != before.Options || &(*value.Options)[0] != &options[0] {
			t.Fatal("failure changed receiver references")
		}
	}
	for _, body := range []string{`{}`, `{"title":null}`, `{"title":42}`, `{"title":"Q","options":42}`, `{"title":"Q","options":[null]}`, `{"title":"Q","options":[42]}`} {
		var control, mixed p.AsyncUserInputQuestion
		controlErr := control.UnmarshalJSON([]byte(body))
		prefix := strings.TrimSuffix(body, "}")
		if body != "{}" {
			prefix += ","
		}
		mixedErr := mixed.UnmarshalJSON([]byte(prefix + `"extra":true}`))
		if controlErr == nil || mixedErr == nil || controlErr.Error() != mixedErr.Error() {
			t.Fatalf("prior check changed: %v vs %v", controlErr, mixedErr)
		}
	}
	var selected p.AsyncUserInputQuestion
	if err := json.Unmarshal([]byte(`{"title":"first","options":["old"],"title":"last","options":null}`), &selected); err != nil || selected.Title != "last" || selected.Options != nil {
		t.Fatalf("valid duplicate selection changed: %+v %v", selected, err)
	}
	envelope := struct {
		Before   int
		Question p.AsyncUserInputQuestion
		After    int
	}{Question: before}
	err := json.Unmarshal([]byte(`{"Before":"bad","Question":{"title":"new","extra":true},"After":7}`), &envelope)
	var native *json.UnmarshalTypeError
	if !errors.As(err, &native) || native.Field != "Before" || envelope.After != 7 || envelope.Question.Options != before.Options || envelope.Question.Title != "prior" {
		t.Fatalf("enclosing decoder contract changed: %+v %v", envelope, err)
	}
	huge := strings.Repeat("peer-", 20000)
	key, _ := json.Marshal(huge)
	err = value.UnmarshalJSON([]byte(`{"title":"Q",` + string(key) + `:null}`))
	if err == nil || len(err.Error()) > 256 {
		t.Fatal("unknown peer key retained in error")
	}
}
