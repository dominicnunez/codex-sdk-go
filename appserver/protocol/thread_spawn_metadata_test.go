package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestThreadSpawnMetadataStates(t *testing.T) {
	states := []struct {
		name string
		raw  string
		want string
	}{
		{"missing", "", "null"}, {"null", "null", "null"}, {"empty", `""`, `""`}, {"text", `"worker/alpha"`, `"worker/alpha"`},
	}
	for _, path := range states {
		for _, nickname := range states {
			for _, role := range states {
				t.Run(path.name+"/"+nickname.name+"/"+role.name, func(t *testing.T) {
					fields := map[string]json.RawMessage{"depth": json.RawMessage(`2`), "parent_thread_id": json.RawMessage(`"parent"`)}
					for key, raw := range map[string]string{"agent_path": path.raw, "agent_nickname": nickname.raw, "agent_role": role.raw} {
						if raw != "" {
							fields[key] = json.RawMessage(raw)
						}
					}
					body, err := json.Marshal(fields)
					if err != nil {
						t.Fatal(err)
					}
					var source p.SessionSourceWrapper
					if err := json.Unmarshal([]byte(`{"subAgent":{"thread_spawn":`+string(body)+`}}`), &source); err != nil {
						t.Fatal(err)
					}
					want := fmt.Sprintf(`{"thread_spawn":{"agent_path":%s,"agent_nickname":%s,"agent_role":%s,"depth":2,"parent_thread_id":"parent"}}`, path.want, nickname.want, role.want)
					assertUnionJSON(t, source, `{"subAgent":`+want+`}`)
					branch, ok := source.Value.(p.SessionSourceSubAgent).SubAgent.(p.SubAgentSourceThreadSpawn)
					if !ok {
						t.Fatalf("decoded branch = %T", source.Value.(p.SessionSourceSubAgent).SubAgent)
					}
					assertUnionJSON(t, branch, want)
					assertUnionJSON(t, &branch, want)
				})
			}
		}
	}
}

const threadSpawnSourceJSON = `{"subAgent":{"thread_spawn":{"agent_path":"worker/alpha","agent_nickname":"worker","agent_role":"researcher","depth":2,"parent_thread_id":"parent"}}}`

func TestThreadSpawnPublicCarriersAndOwnership(t *testing.T) {
	for _, route := range []string{"read", "started"} {
		t.Run(route, func(t *testing.T) {
			transport := NewMockTransport()
			client := p.NewClient(transport)
			var observed, returned p.Thread
			remove := client.AddThreadStateListener("t", func(thread p.Thread) { observed = thread }, nil)
			defer remove()
			thread := validThreadPayload("t")
			thread["source"] = json.RawMessage(threadSpawnSourceJSON)
			if route == "read" {
				if err := transport.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
					t.Fatal(err)
				}
				response, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
				if err != nil {
					t.Fatal(err)
				}
				returned = response.Thread
			} else {
				client.OnThreadStarted(func(n p.ThreadStartedNotification) { returned = n.Thread })
				data, err := json.Marshal(map[string]any{"thread": thread})
				if err != nil {
					t.Fatal(err)
				}
				transport.InjectServerNotification(context.Background(), p.Notification{Method: "thread/started", Params: data})
			}
			assertUnionJSON(t, returned.Source, threadSpawnSourceJSON)
			assertUnionJSON(t, observed.Source, threadSpawnSourceJSON)
			spawn := returned.Source.Value.(p.SessionSourceSubAgent).SubAgent.(p.SubAgentSourceThreadSpawn)
			*spawn.ThreadSpawn.AgentPath = "returned mutation"
			*spawn.ThreadSpawn.AgentNickname = "returned mutation"
			*spawn.ThreadSpawn.AgentRole = "returned mutation"
			assertUnionJSON(t, observed.Source, threadSpawnSourceJSON)
			cached, ok := client.ThreadStateSnapshot("t")
			if !ok {
				t.Fatal("missing cache")
			}
			assertUnionJSON(t, cached.Source, threadSpawnSourceJSON)
			cachedSpawn := cached.Source.Value.(p.SessionSourceSubAgent).SubAgent.(p.SubAgentSourceThreadSpawn)
			*cachedSpawn.ThreadSpawn.AgentPath = "snapshot mutation"
			fresh, _ := client.ThreadStateSnapshot("t")
			assertUnionJSON(t, fresh.Source, threadSpawnSourceJSON)
		})
	}
}

func TestThreadSpawnMetadataRejectsInvalidValues(t *testing.T) {
	for _, field := range []string{`"agent_path":17`, `"agent_nickname":{}`, `"agent_role":false`, `"depth":2147483648`} {
		t.Run(field, func(t *testing.T) {
			var value p.SubAgentSourceThreadSpawn
			if err := json.Unmarshal([]byte(`{"thread_spawn":{"agent_path":"old","depth":1,"parent_thread_id":"parent"}}`), &value); err != nil {
				t.Fatal(err)
			}
			before := value
			if err := json.Unmarshal([]byte(`{"thread_spawn":{"depth":2,"parent_thread_id":"new",`+field+`}}`), &value); err == nil {
				t.Fatal("invalid metadata admitted")
			}
			if !reflect.DeepEqual(value, before) {
				t.Fatal("failed decode mutated receiver")
			}
		})
	}
}

func TestThreadSpawnConstructedMetadata(t *testing.T) {
	var value p.SubAgentSourceThreadSpawn
	value.ThreadSpawn.AgentNickname = ptr("")
	value.ThreadSpawn.AgentPath = ptr("worker/alpha")
	value.ThreadSpawn.AgentRole = ptr("researcher")
	value.ThreadSpawn.Depth = -1
	value.ThreadSpawn.ParentThreadID = "parent"
	assertUnionJSON(t, value, `{"thread_spawn":{"agent_path":"worker/alpha","agent_nickname":"","agent_role":"researcher","depth":-1,"parent_thread_id":"parent"}}`)
}

func TestThreadSpawnDepthMatchesInt32(t *testing.T) {
	for _, tc := range []struct {
		depth string
		valid bool
	}{
		{"-2147483648", true}, {"-1", true}, {"0", true}, {"2147483647", true}, {"-2147483649", false}, {"2147483648", false},
	} {
		t.Run(tc.depth, func(t *testing.T) {
			var value p.SubAgentSourceThreadSpawn
			err := json.Unmarshal([]byte(`{"thread_spawn":{"depth":`+tc.depth+`,"parent_thread_id":"parent"}}`), &value)
			if (err == nil) != tc.valid {
				t.Fatalf("decode depth %s = %v, valid=%v", tc.depth, err, tc.valid)
			}
			if tc.valid {
				assertUnionJSON(t, value, `{"thread_spawn":{"agent_path":null,"agent_nickname":null,"agent_role":null,"depth":`+tc.depth+`,"parent_thread_id":"parent"}}`)
			}
		})
	}
}
