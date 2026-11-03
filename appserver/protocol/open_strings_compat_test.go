package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestOpenLifecycleTierValues(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/resume", "thread/fork"} {
		for _, tc := range []struct {
			name    string
			value   any
			present bool
		}{
			{"absent", nil, false}, {"null", nil, true}, {"empty", "", true}, {"custom", "priority", true}, {"spelling", " Tier/é\n", true}, {"wrong kind", 42, true},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				fixture := validThreadLifecycleResponse(validThreadPayload("thread"))
				if tc.present {
					fixture["serviceTier"] = tc.value
				}
				if err := mock.SetResponseData(method, fixture); err != nil {
					t.Fatal(err)
				}
				var tier *p.ServiceTier
				var err error
				var zero bool
				switch method {
				case "thread/start":
					var r p.ThreadStartResponse
					r, err = c.Thread.Start(context.Background(), p.ThreadStartParams{})
					tier = r.ServiceTier
					zero = reflect.DeepEqual(r, p.ThreadStartResponse{})
				case "thread/resume":
					var r p.ThreadResumeResponse
					r, err = c.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "thread"})
					tier = r.ServiceTier
					zero = reflect.DeepEqual(r, p.ThreadResumeResponse{})
				case "thread/fork":
					var r p.ThreadForkResponse
					r, err = c.Thread.Fork(context.Background(), p.ThreadForkParams{ThreadID: "parent"})
					tier = r.ServiceTier
					zero = reflect.DeepEqual(r, p.ThreadForkResponse{})
				}
				if tc.name == "wrong kind" {
					if err == nil || !zero {
						t.Fatal("malformed tier published lifecycle success")
					}
					if _, ok := c.ThreadStateSnapshot("thread"); ok {
						t.Fatal("malformed response cached")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if tc.value == nil {
					if tier != nil {
						t.Fatal("absent/null tier became present")
					}
				} else if tier == nil || string(*tier) != tc.value.(string) {
					t.Fatalf("tier changed: %v", tier)
				}
			})
		}
	}
}

func TestOpenThreadSourceConsumers(t *testing.T) {
	for _, route := range []string{"read", "started"} {
		for _, value := range []any{nil, "", "guardian_review", "custom-client", " source/é\n", 42} {
			t.Run(route+"/"+stringValueName(value), func(t *testing.T) {
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				c.CacheThreadState(p.Thread{ID: "thread", Preview: "before"})
				var observed, returned p.Thread
				updates, callbacks := 0, 0
				remove := c.AddThreadStateListener("thread", func(thread p.Thread) { observed = thread; updates++ }, nil)
				defer remove()
				initialUpdates := updates
				thread := validThreadPayload("thread")
				thread["threadSource"] = value
				body, err := json.Marshal(map[string]any{"thread": thread})
				if err != nil {
					t.Fatal(err)
				}
				if route == "read" {
					mock.SetResponse("thread/read", p.Response{Result: body})
					r, e := c.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "thread"})
					returned = r.Thread
					err = e
					if value == 42 && !reflect.DeepEqual(r, p.ThreadReadResponse{}) {
						t.Fatal("malformed source published read result")
					}
				} else {
					c.OnThreadStarted(func(n p.ThreadStartedNotification) { returned = n.Thread; callbacks++ })
					mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/started", Params: body})
				}
				cached, ok := c.ThreadStateSnapshot("thread")
				if !ok {
					t.Fatal("cache missing")
				}
				if value == 42 {
					if (route == "read" && err == nil) || callbacks != 0 || updates != initialUpdates || cached.Preview != "before" {
						t.Fatalf("malformed source admitted: %v updates=%d callbacks=%d", err, updates, callbacks)
					}
					return
				}
				if err != nil || updates != initialUpdates+1 || (route == "started" && callbacks != 1) {
					t.Fatalf("source rejected by consumer: %v updates=%d callbacks=%d", err, updates, callbacks)
				}
				for _, got := range []p.Thread{returned, observed, cached} {
					if value == nil {
						if got.ThreadSource != nil {
							t.Fatal("null source became present")
						}
					} else if got.ThreadSource == nil || string(*got.ThreadSource) != value.(string) {
						t.Fatal("consumer lost source")
					}
				}
				if returned.ThreadSource != nil {
					*returned.ThreadSource = "changed"
					*observed.ThreadSource = "changed too"
					*cached.ThreadSource = "snapshot changed"
					fresh, _ := c.ThreadStateSnapshot("thread")
					if fresh.ThreadSource == nil || string(*fresh.ThreadSource) != value.(string) {
						t.Fatal("source pointer aliases cache")
					}
				}
			})
		}
	}
}

func stringValueName(value any) string {
	if value == nil {
		return "null"
	}
	if text, ok := value.(string); ok {
		return text
	}
	return "wrong kind"
}

func TestOpenStringCodecErrorsAndMethods(t *testing.T) {
	for _, field := range []string{"serviceTier", "threadSource"} {
		for _, raw := range []string{"null", "42", "true", "{}", "[]", `"unterminated`} {
			t.Run(field+"/"+raw, func(t *testing.T) {
				var dst any
				if field == "serviceTier" {
					v := p.ServiceTier("before")
					dst = &v
				} else {
					v := p.ThreadSource("before")
					dst = &v
				}
				before, _ := json.Marshal(dst)
				err := json.Unmarshal([]byte(raw), dst)
				after, _ := json.Marshal(dst)
				if err == nil || string(before) != string(after) {
					t.Fatalf("bad string changed receiver: %v", err)
				}
				switch raw {
				case "null":
					if !strings.Contains(err.Error(), field) {
						t.Fatal("null field context lost")
					}
				case `"unterminated`:
					var syntax *json.SyntaxError
					if !errors.As(err, &syntax) {
						t.Fatal("syntax cause lost")
					}
				default:
					var native *json.UnmarshalTypeError
					if !errors.As(err, &native) {
						t.Fatal("native kind cause lost")
					}
				}
				var directErr error
				switch typed := dst.(type) {
				case *p.ServiceTier:
					directErr = typed.UnmarshalJSON([]byte(raw))
				case *p.ThreadSource:
					directErr = typed.UnmarshalJSON([]byte(raw))
				}
				encoded, _ := json.Marshal(dst)
				if directErr == nil || directErr.Error() != err.Error() || string(encoded) != string(before) {
					t.Fatal("direct JSON method changed error or receiver")
				}
			})
		}
	}
	for _, v := range []any{p.ServiceTier(""), p.ThreadSource(""), p.Ptr(p.ServiceTier("")), p.Ptr(p.ThreadSource(""))} {
		if _, ok := v.(json.Marshaler); !ok {
			t.Fatal("public marshal method removed")
		}
	}
	for _, v := range []any{p.Ptr(p.ServiceTier("")), p.Ptr(p.ThreadSource(""))} {
		if _, ok := v.(json.Unmarshaler); !ok {
			t.Fatal("public decoder method removed")
		}
	}
	var optional struct {
		Tier   *p.ServiceTier
		Source *p.ThreadSource
	}
	if err := json.Unmarshal([]byte(`{"Tier":null,"Source":null}`), &optional); err != nil || optional.Tier != nil || optional.Source != nil {
		t.Fatal("nullable carrier behavior changed")
	}
	for _, v := range []any{(*p.ServiceTier)(nil), (*p.ThreadSource)(nil)} {
		raw, err := json.Marshal(v)
		if err != nil || string(raw) != "null" {
			t.Fatal("nil pointer encoding changed")
		}
	}
	// These remain closed protocol enums and required owner checks.
	for _, v := range []any{p.ThreadSourceKind("custom-client"), p.ApprovalsReviewer("custom-client"), p.Personality("custom-client")} {
		if _, err := json.Marshal(v); err == nil {
			t.Fatalf("closed enum weakened: %T", v)
		}
	}
}

func FuzzOpenProtocolStrings(f *testing.F) {
	for _, value := range []string{"", "priority", "guardian_review", "custom-client", "fast", "\x00é\n\\\"", string([]byte{0xff})} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		expected, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var normalized string
		if err := json.Unmarshal(expected, &normalized); err != nil {
			t.Fatal(err)
		}
		for _, typed := range []any{p.ServiceTier(value), p.ThreadSource(value)} {
			encoded, err := json.Marshal(typed)
			if err != nil || string(encoded) != string(expected) {
				t.Fatalf("open string encoding differs from JSON string: %T %v", typed, err)
			}
		}
		var tier p.ServiceTier
		var source p.ThreadSource
		if err := json.Unmarshal(expected, &tier); err != nil || string(tier) != normalized {
			t.Fatal("tier rejected/changed standard JSON string")
		}
		if err := json.Unmarshal(expected, &source); err != nil || string(source) != normalized {
			t.Fatal("source rejected/changed standard JSON string")
		}
	})
}
