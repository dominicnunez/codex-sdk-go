package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func metadataItemJSON(branch, value string) []byte {
	switch branch {
	case "agentMessage":
		return fmt.Appendf(nil, `{"type":"agentMessage","id":"item","text":"hello","delivery":%s}`, value)
	case "commandExecution":
		return fmt.Appendf(nil, `{"type":"commandExecution","id":"item","command":"true","commandActions":[],"cwd":"/tmp","status":"completed","source":%s}`, value)
	default:
		return fmt.Appendf(nil, `{"type":"subAgentActivity","id":"item","agentPath":"worker/alpha","agentThreadId":"agent","kind":%s}`, value)
	}
}

func metadataEnumField(branch string) string {
	return map[string]string{"agentMessage": "delivery", "commandExecution": "source", "subAgentActivity": "kind"}[branch]
}

func assertItemMetadata(t *testing.T, item p.ThreadItemWrapper, branch, want string) {
	t.Helper()
	wire, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if got := string(fields[metadataEnumField(branch)]); got != want {
		t.Fatalf("delivered %s metadata = %s; want %s", branch, got, want)
	}
}

func TestThreadItemMetadataEnumValues(t *testing.T) {
	for branch, values := range map[string][]string{
		"agentMessage":     {`"async"`, `null`},
		"commandExecution": {`"agent"`, `"userShell"`, `"unifiedExecStartup"`, `"unifiedExecInteraction"`},
		"subAgentActivity": {`"started"`, `"interacted"`, `"interrupted"`, `"completed"`},
	} {
		for _, raw := range values {
			t.Run(branch+raw, func(t *testing.T) {
				var item p.ThreadItemWrapper
				if err := json.Unmarshal(metadataItemJSON(branch, raw), &item); err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(item)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				got := string(fields[metadataEnumField(branch)])
				if raw == "null" {
					if got != "" && got != "null" {
						t.Fatal("nullable delivery became populated")
					}
				} else if got != raw {
					t.Fatalf("metadata = %s; want %s", got, raw)
				}
			})
		}
	}
	for _, branch := range []string{"agentMessage", "commandExecution", "subAgentActivity"} {
		for _, raw := range []string{`""`, `17`, `false`, `{}`, `[]`} {
			var item p.ThreadItemWrapper
			if err := json.Unmarshal(metadataItemJSON(branch, raw), &item); err == nil {
				t.Fatalf("%s admitted %s", branch, raw)
			}
		}
		field := metadataEnumField(branch)
		missing := strings.Replace(string(metadataItemJSON(branch, `null`)), `,"`+field+`":null`, "", 1)
		var item p.ThreadItemWrapper
		err := json.Unmarshal([]byte(missing), &item)
		if (err == nil) != (branch != "subAgentActivity") {
			t.Fatalf("%s missing metadata = %v", branch, err)
		}
	}
}

func TestThreadItemMetadataDuplicateValidation(t *testing.T) {
	for branch, valid := range map[string]string{"agentMessage": `"async"`, "commandExecution": `"agent"`, "subAgentActivity": `"started"`} {
		field := metadataEnumField(branch)
		for _, key := range []string{field, strings.ToUpper(field), `\u00` + fmt.Sprintf("%02x", field[0]) + field[1:]} {
			wire := strings.TrimSuffix(string(metadataItemJSON(branch, `"future"`)), "}") + `,"` + key + `":` + valid + `}`
			var item p.ThreadItemWrapper
			if err := json.Unmarshal([]byte(wire), &item); err == nil {
				t.Fatalf("earlier invalid %s repaired by %s", field, key)
			}
		}
		if branch != "agentMessage" {
			wire := strings.TrimSuffix(string(metadataItemJSON(branch, `null`)), "}") + `,"` + field + `":` + valid + `}`
			var item p.ThreadItemWrapper
			if err := json.Unmarshal([]byte(wire), &item); !errors.Is(err, p.ErrNullResultField) {
				t.Fatalf("null %s repaired by duplicate: %v", field, err)
			}
		}
	}
}

func TestThreadItemMetadataPublicAdmission(t *testing.T) {
	for branch, valid := range map[string]string{"agentMessage": `"async"`, "commandExecution": `"agent"`, "subAgentActivity": `"started"`} {
		for _, route := range []string{"read", "items", "turns", "started", "completed"} {
			t.Run(branch+"/"+route, func(t *testing.T) {
				mock := NewMockTransport()
				failures, calls, updates := 0, 0, 0
				client := p.NewClient(mock, p.WithHandlerErrorCallback(func(string, error) { failures++ }))
				client.CacheThreadState(p.Thread{ID: "t", Preview: "prior"})
				remove := client.AddThreadStateListener("t", func(p.Thread) { updates++ }, nil)
				defer remove()
				updates = 0
				onStarted := func(n p.ItemStartedNotification) { calls++; assertItemMetadata(t, n.Item, branch, valid) }
				onCompleted := func(n p.ItemCompletedNotification) { calls++; assertItemMetadata(t, n.Item, branch, valid) }
				client.OnItemStarted(onStarted)
				client.OnItemCompleted(onCompleted)
				removeStarted := client.AddItemStartedListener(onStarted)
				defer removeStarted()
				removeCompleted := client.AddItemCompletedListener(onCompleted)
				defer removeCompleted()
				for _, raw := range []string{`"future"`, valid} {
					item := json.RawMessage(metadataItemJSON(branch, raw))
					turn := map[string]any{"id": "u", "status": "completed", "items": []json.RawMessage{item}}
					var err error
					switch route {
					case "read":
						thread := validThreadPayload("t")
						thread["turns"] = []any{turn}
						if err = mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
							t.Fatal(err)
						}
						var response p.ThreadReadResponse
						response, err = client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
						if err == nil {
							assertItemMetadata(t, response.Thread.Turns[0].Items[0], branch, valid)
						}
					case "items":
						if err = mock.SetResponseData("thread/items/list", map[string]any{"data": []any{map[string]any{"turnId": "u", "item": item}}}); err != nil {
							t.Fatal(err)
						}
						var response p.ThreadItemsListResponse
						response, err = client.Thread.ItemsList(context.Background(), p.ThreadItemsListParams{ThreadID: "t"})
						if err == nil {
							assertItemMetadata(t, response.Data[0].Item, branch, valid)
						}
					case "turns":
						if err = mock.SetResponseData("thread/turns/list", map[string]any{"data": []any{turn}}); err != nil {
							t.Fatal(err)
						}
						var response p.ThreadTurnsListResponse
						response, err = client.Thread.TurnsList(context.Background(), p.ThreadTurnsListParams{ThreadID: "t"})
						if err == nil {
							assertItemMetadata(t, response.Data[0].Items[0], branch, valid)
						}
					default:
						wire, marshalErr := json.Marshal(map[string]any{"threadId": "t", "turnId": "u", "item": item, "startedAtMs": 1, "completedAtMs": 2})
						if marshalErr != nil {
							t.Fatal(marshalErr)
						}
						mock.InjectServerNotification(context.Background(), p.Notification{Method: "item/" + route, Params: wire})
					}
					wantValid := raw == valid
					if route == "started" || route == "completed" {
						if !wantValid && (calls != 0 || failures == 0) || wantValid && calls != 2 {
							t.Fatalf("notification calls=%d failures=%d", calls, failures)
						}
					} else if (err == nil) != wantValid {
						t.Fatalf("public call error=%v, valid=%v", err, wantValid)
					}
					if !wantValid {
						cached, _ := client.ThreadStateSnapshot("t")
						if cached.Preview != "prior" || updates != 0 {
							t.Fatal("invalid item changed thread state")
						}
					}
				}
			})
		}
	}
}

func TestThreadItemCommandSourceDefault(t *testing.T) {
	var value p.ThreadItemWrapper
	wire := strings.Replace(string(metadataItemJSON("commandExecution", `null`)), `,"source":null`, "", 1)
	if err := json.Unmarshal([]byte(wire), &value); err != nil {
		t.Fatal(err)
	}
	command := value.Value.(*p.CommandExecutionThreadItem)
	if command.Source == nil || *command.Source != p.CommandExecutionSourceAgent {
		t.Errorf("missing source did not default to agent: %#v", command.Source)
	}
	old := value.Value
	if err := json.Unmarshal(metadataItemJSON("commandExecution", `null`), &value); !errors.Is(err, p.ErrNullResultField) {
		t.Errorf("null non-nullable command source = %v; want ErrNullResultField", err)
	}
	if value.Value != old {
		t.Fatal("failed null decode changed receiver")
	}
}

func TestThreadItemCommandDefaultOwnership(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	item := strings.Replace(string(metadataItemJSON("commandExecution", `null`)), `,"source":null`, "", 1)
	thread := validThreadPayload("t")
	thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []json.RawMessage{json.RawMessage(item)}}}
	if err := mock.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
		t.Fatal(err)
	}
	response, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
	if err != nil {
		t.Fatal(err)
	}
	source := func(thread p.Thread) *p.CommandExecutionSource {
		t.Helper()
		return thread.Turns[0].Items[0].Value.(*p.CommandExecutionThreadItem).Source
	}
	if source(response.Thread) == nil || *source(response.Thread) != p.CommandExecutionSourceAgent {
		t.Fatal("default not propagated through Thread.Read")
	}
	*source(response.Thread) = p.CommandExecutionSourceUserShell
	cached, _ := client.ThreadStateSnapshot("t")
	if source(cached) == nil || *source(cached) != p.CommandExecutionSourceAgent {
		t.Fatal("returned default pointer changed cache")
	}
	*source(cached) = p.CommandExecutionSourceUnifiedExecStartup
	fresh, _ := client.ThreadStateSnapshot("t")
	if *source(fresh) != p.CommandExecutionSourceAgent {
		t.Fatal("snapshot default pointer changed cache")
	}
}

func TestThreadItemMetadataKeepsExistingDecodeErrors(t *testing.T) {
	wire := strings.Replace(string(metadataItemJSON("commandExecution", `"future"`)), `"status":"completed"`, `"status":"queued"`, 1)
	var item p.ThreadItemWrapper
	err := json.Unmarshal([]byte(wire), &item)
	if err == nil || !strings.Contains(err.Error(), `invalid commandExecution.status "queued"`) {
		t.Fatalf("prior status error masked: %v", err)
	}
	var envelope struct {
		p.CommandExecutionSource `json:"source"`
		Note                     string `json:"note"`
	}
	if err := json.Unmarshal([]byte(`{"source":"agent","note":"kept"}`), &envelope); err != nil || envelope.Note != "kept" || envelope.CommandExecutionSource != p.CommandExecutionSourceAgent {
		t.Fatalf("plain scalar envelope changed: %#v, %v", envelope, err)
	}
}

func TestThreadItemMetadataRejectsUnknownEnums(t *testing.T) {
	for _, branch := range []string{"agentMessage", "commandExecution", "subAgentActivity"} {
		t.Run(branch, func(t *testing.T) {
			old := &p.PlanThreadItem{ID: "old", Text: "old"}
			value := p.ThreadItemWrapper{Value: old}
			if err := json.Unmarshal(metadataItemJSON(branch, `"future"`), &value); err == nil {
				t.Fatal("unknown metadata enum admitted")
			}
			if value.Value != old {
				t.Fatal("failed decode changed receiver")
			}
		})
	}
}
