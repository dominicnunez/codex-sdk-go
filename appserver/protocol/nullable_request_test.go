package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type nullableRequestRoute struct {
	name, method, field string
	nested              bool
	value               any
	params              func() any
	call                func(*p.Client, any) error
}

func nullableRequestRoutes() []nullableRequestRoute {
	routes := []nullableRequestRoute{
		{"start", "thread/start", "serviceTier", false, "fast", func() any { return new(p.ThreadStartParams) }, func(c *p.Client, v any) error {
			_, e := c.Thread.Start(context.Background(), *v.(*p.ThreadStartParams))
			return e
		}},
		{"fork", "thread/fork", "serviceTier", false, "fast", func() any { return new(p.ThreadForkParams) }, func(c *p.Client, v any) error {
			_, e := c.Thread.Fork(context.Background(), *v.(*p.ThreadForkParams))
			return e
		}},
		{"resume", "thread/resume", "serviceTier", false, "fast", func() any { return new(p.ThreadResumeParams) }, func(c *p.Client, v any) error {
			_, e := c.Thread.Resume(context.Background(), *v.(*p.ThreadResumeParams))
			return e
		}},
		{"turn", "turn/start", "serviceTier", false, "fast", func() any { return new(p.TurnStartParams) }, func(c *p.Client, v any) error {
			_, e := c.Turn.Start(context.Background(), *v.(*p.TurnStartParams))
			return e
		}},
		{"goal", "thread/goal/set", "tokenBudget", false, float64(17), func() any { return new(p.ThreadGoalSetParams) }, func(c *p.Client, v any) error {
			_, e := c.Thread.GoalSet(context.Background(), *v.(*p.ThreadGoalSetParams))
			return e
		}},
	}
	for _, key := range []string{"sha", "branch", "originUrl"} {
		routes = append(routes, nullableRequestRoute{"metadata/" + key, "thread/metadata/update", key, true, "nonempty", func() any { return new(p.ThreadMetadataUpdateParams) }, func(c *p.Client, v any) error {
			_, e := c.Thread.MetadataUpdate(context.Background(), *v.(*p.ThreadMetadataUpdateParams))
			return e
		}})
	}
	return routes
}

func TestNullableRequestPresence(t *testing.T) {
	for _, r := range nullableRequestRoutes() {
		for _, state := range []string{"absent", "null", "value"} {
			t.Run(r.name+"/"+state, func(t *testing.T) {
				body := map[string]any{"threadId": "thread"}
				if r.method == "thread/start" {
					body = map[string]any{}
				}
				if r.method == "turn/start" {
					body["input"] = []any{}
				}
				target := body
				if r.nested {
					target = map[string]any{}
					body["gitInfo"] = target
				}
				if state == "null" {
					target[r.field] = nil
				}
				if state == "value" {
					target[r.field] = r.value
				}
				raw, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				params := r.params()
				if err := json.Unmarshal(raw, params); err != nil {
					t.Fatal(err)
				}
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				var result any = validThreadLifecycleResponse(validThreadPayload("thread"))
				switch r.method {
				case "thread/metadata/update":
					result = map[string]any{"thread": validThreadPayload("thread")}
				case "turn/start":
					result = map[string]any{"turn": map[string]any{"id": "turn", "status": "inProgress", "items": []any{}}}
				case "thread/goal/set":
					result = map[string]any{"goal": map[string]any{"threadId": "thread", "objective": "goal", "status": "active", "createdAt": 0, "updatedAt": 0, "timeUsedSeconds": 0, "tokensUsed": 0}}
				}
				if err := mock.SetResponseData(r.method, result); err != nil {
					t.Fatal(err)
				}
				if err := r.call(c, params); err != nil {
					t.Fatal("public call:", err)
				}
				if len(mock.SentRequests) != 1 {
					t.Fatal("request not captured")
				}
				var sent map[string]any
				if err := json.Unmarshal(mock.SentRequests[0].Params, &sent); err != nil {
					t.Fatal(err)
				}
				if r.nested {
					sent = sent["gitInfo"].(map[string]any)
				}
				got, has := sent[r.field]
				want, wantHas := target[r.field]
				if has != wantHas || !reflect.DeepEqual(got, want) {
					t.Errorf("presence changed: input=%s request=%s", raw, mock.SentRequests[0].Params)
				}
			})
		}
	}
}

func captureNullableRequest(t *testing.T, r nullableRequestRoute, params any) map[string]json.RawMessage {
	t.Helper()
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	var result any = validThreadLifecycleResponse(validThreadPayload("thread"))
	switch r.method {
	case "thread/metadata/update":
		result = map[string]any{"thread": validThreadPayload("thread")}
	case "turn/start":
		result = map[string]any{"turn": map[string]any{"id": "turn", "status": "inProgress", "items": []any{}}}
	case "thread/goal/set":
		result = map[string]any{"goal": map[string]any{"threadId": "thread", "objective": "goal", "status": "active", "createdAt": 0, "updatedAt": 0, "timeUsedSeconds": 0, "tokensUsed": 0}}
	}
	if err := mock.SetResponseData(r.method, result); err != nil {
		t.Fatal(err)
	}
	if err := r.call(client, params); err != nil {
		t.Fatal(err)
	}
	if len(mock.SentRequests) != 1 {
		t.Fatal("request not sent once")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(mock.GetSentRequest(0).Params, &fields); err != nil {
		t.Fatal(err)
	}
	if r.nested {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(fields["gitInfo"], &nested); err != nil {
			t.Fatal(err)
		}
		return nested
	}
	return fields
}

func nullableRequestBody(r nullableRequestRoute, fields string) []byte {
	prefix := `"threadId":"thread"`
	if r.method == "thread/start" {
		prefix = `"future":"kept"`
	}
	if r.method == "turn/start" {
		prefix += `,"input":[]`
	}
	if r.nested {
		return []byte(`{` + prefix + `,"gitInfo":{` + fields + `}}`)
	}
	if fields != "" {
		prefix += `,` + fields
	}
	return []byte(`{` + prefix + `}`)
}

func TestNullableRequestConstructedStates(t *testing.T) {
	for _, r := range nullableRequestRoutes() {
		for _, state := range []string{"absent", "null", "value"} {
			t.Run(r.name+"/"+state, func(t *testing.T) {
				params := r.params()
				tier := p.OptionalNullable[p.ServiceTier]{Present: state != "absent"}
				budget := p.OptionalNullable[int64]{Present: state != "absent"}
				text := p.OptionalNullable[string]{Present: state != "absent"}
				if state == "value" {
					tier.Value = p.Ptr(p.ServiceTier(""))
					budget.Value = p.Ptr(int64(0))
					text.Value = p.Ptr("replacement")
				}
				switch v := params.(type) {
				case *p.ThreadStartParams:
					v.ServiceTier = tier
				case *p.ThreadForkParams:
					v.ThreadID = "thread"
					v.ServiceTier = tier
				case *p.ThreadResumeParams:
					v.ThreadID = "thread"
					v.ServiceTier = tier
				case *p.TurnStartParams:
					v.ThreadID = "thread"
					v.Input = []p.UserInput{}
					v.ServiceTier = tier
				case *p.ThreadGoalSetParams:
					v.ThreadID = "thread"
					v.TokenBudget = budget
				case *p.ThreadMetadataUpdateParams:
					v.ThreadID = "thread"
					v.GitInfo = &p.ThreadMetadataGitInfoUpdateParams{}
					switch r.field {
					case "sha":
						v.GitInfo.SHA = text
					case "branch":
						v.GitInfo.Branch = text
					case "originUrl":
						v.GitInfo.OriginURL = text
					}
				}
				sent := captureNullableRequest(t, r, params)
				want := ""
				if state == "null" {
					want = "null"
				}
				if state == "value" {
					want = `"replacement"`
					if r.field == "serviceTier" {
						want = `""`
					}
					if r.field == "tokenBudget" {
						want = "0"
					}
				}
				if string(sent[r.field]) != want {
					t.Fatalf("constructed state lost: %s want %s", sent[r.field], want)
				}
			})
		}
	}
}

func TestNullableRequestHistories(t *testing.T) {
	for _, r := range nullableRequestRoutes() {
		value, err := json.Marshal(r.value)
		if err != nil {
			t.Fatal(err)
		}
		member := `"` + r.field + `":`
		escaped := `"\u00` + fmt.Sprintf("%02x", r.field[0]) + r.field[1:] + `":`
		for _, tc := range []struct{ name, fields, want string }{
			{"value then null", member + string(value) + `,` + member + `null`, "null"},
			{"null then value", member + `null,` + member + string(value), string(value)},
			{"null reset and value", member + string(value) + `,` + member + `null,` + member + string(value), string(value)},
			{"escaped null", member + string(value) + `,` + escaped + `null`, "null"},
			{"native alias null", member + string(value) + `,"` + strings.ToUpper(r.field) + `":null`, "null"},
		} {
			t.Run(r.name+"/"+tc.name, func(t *testing.T) {
				params := r.params()
				if err := json.Unmarshal(nullableRequestBody(r, tc.fields), params); err != nil {
					t.Fatal(err)
				}
				sent := captureNullableRequest(t, r, params)
				if string(sent[r.field]) != tc.want {
					t.Fatalf("history lost: %s want %s", sent[r.field], tc.want)
				}
			})
		}
		t.Run(r.name+"/reused receiver", func(t *testing.T) {
			params := r.params()
			for _, raw := range []string{member + string(value), member + `null`} {
				if err := json.Unmarshal(nullableRequestBody(r, raw), params); err != nil {
					t.Fatal(err)
				}
			}
			if got := captureNullableRequest(t, r, params); string(got[r.field]) != "null" {
				t.Fatal("reused receiver restored old value")
			}
			if err := json.Unmarshal(nullableRequestBody(r, ""), params); err != nil {
				t.Fatal(err)
			}
			got := string(captureNullableRequest(t, r, params)[r.field])
			// TurnStart's existing decoder replaces the whole record; plain records
			// retain omitted members during receiver reuse.
			want := "null"
			if r.method == "turn/start" {
				want = ""
			}
			if got != want {
				t.Fatalf("omission changed owner receiver behavior: %s want %s", got, want)
			}
		})
	}
}

func TestNullableRequestExactBudgets(t *testing.T) {
	var route nullableRequestRoute
	for _, r := range nullableRequestRoutes() {
		if r.field == "tokenBudget" {
			route = r
		}
	}
	for _, number := range []string{"0", "-1", "9223372036854775807", "-9223372036854775808"} {
		t.Run(number, func(t *testing.T) {
			params := route.params()
			if err := json.Unmarshal(nullableRequestBody(route, `"tokenBudget":`+number), params); err != nil {
				t.Fatal(err)
			}
			sent := captureNullableRequest(t, route, params)
			if string(sent["tokenBudget"]) != number {
				t.Fatal("budget lost integer precision")
			}
		})
	}
}

func TestNullableGitObjectHistories(t *testing.T) {
	for _, r := range nullableRequestRoutes() {
		if !r.nested {
			continue
		}
		member := `"` + r.field + `":`
		for _, tc := range []struct{ name, objects, want string }{
			{"merged null", `{"` + r.field + `":"old"},"gitInfo":{` + member + `null}`, "null"},
			{"reset parent", `{"` + r.field + `":"old"},"gitInfo":null,"gitInfo":{}`, ""},
			{"preserve null in partial merge", `{` + member + `null},"gitInfo":{"future":"kept"}`, "null"},
		} {
			t.Run(r.field+"/"+tc.name, func(t *testing.T) {
				params := r.params()
				raw := []byte(`{"threadId":"thread","gitInfo":` + tc.objects + `}`)
				if err := json.Unmarshal(raw, params); err != nil {
					t.Fatal(err)
				}
				if got := string(captureNullableRequest(t, r, params)[r.field]); got != tc.want {
					t.Fatalf("nested merge/reset changed presence: %s want %s", got, tc.want)
				}
			})
		}
	}
}

func FuzzNullableRequestHistories(f *testing.F) {
	for _, history := range [][]byte{{}, {0}, {1}, {2}, {2, 1}, {1, 2}, {2, 1, 0}, {1, 0, 2, 0, 1}} {
		f.Add(history)
	}
	f.Fuzz(func(t *testing.T, history []byte) {
		if len(history) > 32 {
			history = history[:32]
		}
		for _, r := range nullableRequestRoutes() {
			value, err := json.Marshal(r.value)
			if err != nil {
				t.Fatal(err)
			}
			var members []string
			want := ""
			for _, op := range history {
				switch op % 3 {
				case 0:
					continue
				case 1:
					want = "null"
				case 2:
					want = string(value)
				}
				members = append(members, `"`+r.field+`":`+want)
			}
			params := r.params()
			if err := json.Unmarshal(nullableRequestBody(r, strings.Join(members, ",")), params); err != nil {
				t.Fatal(err)
			}
			got := string(captureNullableRequest(t, r, params)[r.field])
			if got != want {
				t.Fatalf("last explicit state lost: field=%s history=%v got=%s want=%s", r.field, history, got, want)
			}
		}
	})
}
