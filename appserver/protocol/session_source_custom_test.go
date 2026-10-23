package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestSessionSourceCustomRejectsMalformedKnownBranch(t *testing.T) {
	for _, raw := range []string{`null`, `17`, `false`, `{}`, `[]`} {
		t.Run(raw, func(t *testing.T) {
			source := p.SessionSourceWrapper{Value: p.SessionSourceCLI}
			if err := json.Unmarshal([]byte(`{"custom":`+raw+`}`), &source); err == nil {
				t.Fatal("malformed known custom source admitted")
			}
			if source.Value != p.SessionSourceCLI {
				t.Fatal("failed decode changed receiver")
			}
		})
	}
}

func TestSessionSourceCustomRoundTrip(t *testing.T) {
	for _, name := range []string{"", "atlas", "worker/alpha", "quote\"\n日本語"} {
		t.Run(name, func(t *testing.T) {
			wire, err := json.Marshal(map[string]string{"custom": name})
			if err != nil {
				t.Fatal(err)
			}
			var source p.SessionSourceWrapper
			if err := json.Unmarshal(wire, &source); err != nil {
				t.Fatal(err)
			}
			custom, ok := source.Value.(p.SessionSourceCustom)
			if !ok || custom.Custom != name {
				t.Fatalf("decoded source = %#v", source.Value)
			}
			for _, value := range []any{custom, &custom, source, p.SessionSourceWrapper{Value: &custom}, codex.SessionSourceCustom{Custom: name}} {
				assertUnionJSON(t, value, string(wire))
			}
		})
	}
	assertUnionJSON(t, p.SessionSourceWrapper{Value: (*p.SessionSourceCustom)(nil)}, `null`)
}

func TestSessionSourceCustomDirectValidation(t *testing.T) {
	for _, wire := range []string{
		`{}`, `null`, `{"custom":null}`, `{"custom":17}`,
		`{"custom":null,"custom":"later"}`, `{"custom":"prior","custom":null}`,
		`{"custom":false,"custom":"later"}`, `{"custom":null,"cu\u0073tom":"later"}`,
	} {
		t.Run(wire, func(t *testing.T) {
			value := p.SessionSourceCustom{Custom: "old"}
			if err := json.Unmarshal([]byte(wire), &value); err == nil {
				t.Fatal("invalid direct custom source admitted")
			}
			if value.Custom != "old" {
				t.Fatal("failed direct decode changed receiver")
			}
		})
	}
	var source p.SessionSourceWrapper
	if err := json.Unmarshal([]byte(`{"cu\u0073tom":"escaped"}`), &source); err != nil {
		t.Fatal(err)
	}
	assertUnionJSON(t, source, `{"custom":"escaped"}`)
	if err := json.Unmarshal([]byte(`{"future":{"payload":42}}`), &source); err != nil {
		t.Fatal(err)
	}
	if _, ok := source.Value.(p.UnknownSessionSource); !ok {
		t.Fatalf("future object = %T", source.Value)
	}
	assertUnionJSON(t, source, `{"future":{"payload":42}}`)
}

func TestSessionSourceCustomPublicAdmission(t *testing.T) {
	for _, route := range []string{"read", "started"} {
		for _, custom := range []json.RawMessage{json.RawMessage(`""`), json.RawMessage(`"atlas"`), json.RawMessage(`null`), json.RawMessage(`17`)} {
			t.Run(route+string(custom), func(t *testing.T) {
				transport := NewMockTransport()
				failures := 0
				client := p.NewClient(transport, p.WithHandlerErrorCallback(func(string, error) { failures++ }))
				client.CacheThreadState(p.Thread{ID: "t", Source: p.SessionSourceWrapper{Value: p.SessionSourceCLI}})
				updates := 0
				remove := client.AddThreadStateListener("t", func(p.Thread) { updates++ }, nil)
				defer remove()
				updates = 0
				typed := 0
				var returned p.Thread
				thread := validThreadPayload("t")
				thread["source"] = map[string]any{"custom": custom}
				valid := custom[0] == '"'
				if route == "read" {
					if err := transport.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
						t.Fatal(err)
					}
					response, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
					if (err == nil) != valid {
						t.Fatalf("Thread.Read error = %v; valid=%v", err, valid)
					}
					if err == nil {
						returned = response.Thread
					}
				} else {
					client.OnThreadStarted(func(n p.ThreadStartedNotification) { typed++; returned = n.Thread })
					wire, err := json.Marshal(map[string]any{"thread": thread})
					if err != nil {
						t.Fatal(err)
					}
					transport.InjectServerNotification(context.Background(), p.Notification{Method: "thread/started", Params: wire})
					if valid && (typed != 1 || failures != 0) || !valid && (typed != 0 || failures == 0) {
						t.Fatalf("notification delivery: typed=%d, failures=%d", typed, failures)
					}
				}
				cached, ok := client.ThreadStateSnapshot("t")
				if !ok {
					t.Fatal("lost prior cache state")
				}
				if !valid {
					if updates != 0 {
						t.Fatal("invalid source published to listener")
					}
					assertUnionJSON(t, cached.Source, `"cli"`)
					return
				}
				if updates != 1 {
					t.Fatalf("listener updates = %d", updates)
				}
				want := `{"custom":` + string(custom) + `}`
				assertUnionJSON(t, returned.Source, want)
				assertUnionJSON(t, cached.Source, want)
				if _, ok := cached.Source.Value.(p.SessionSourceCustom); !ok {
					t.Fatalf("cached branch = %T", cached.Source.Value)
				}
			})
		}
	}
}
