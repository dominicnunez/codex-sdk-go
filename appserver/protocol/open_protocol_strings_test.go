package protocol_test

import (
	"context"
	"encoding/json"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func callOpenTierRequest(t *testing.T, method string, tier *p.ServiceTier) error {
	t.Helper()
	mock := NewMockTransport()
	c := p.NewClient(mock)
	t.Cleanup(func() { _ = c.Close() })
	var result any = validThreadLifecycleResponse(validThreadPayload("thread"))
	if method == "turn/start" {
		result = map[string]any{"turn": map[string]any{"id": "turn", "status": "inProgress", "items": []any{}}}
	}
	if err := mock.SetResponseData(method, result); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var err error
	switch method {
	case "thread/start":
		_, err = c.Thread.Start(ctx, p.ThreadStartParams{ServiceTier: tier})
	case "thread/fork":
		_, err = c.Thread.Fork(ctx, p.ThreadForkParams{ThreadID: "thread", ServiceTier: tier})
	case "thread/resume":
		_, err = c.Thread.Resume(ctx, p.ThreadResumeParams{ThreadID: "thread", ServiceTier: tier})
	case "turn/start":
		_, err = c.Turn.Start(ctx, p.TurnStartParams{ThreadID: "thread", Input: []p.UserInput{}, ServiceTier: tier})
	}
	if err != nil {
		return err
	}
	if len(mock.SentRequests) != 1 {
		t.Fatal("request not dispatched exactly once")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(mock.GetSentRequest(0).Params, &fields); err != nil {
		t.Fatal(err)
	}
	if tier == nil {
		if _, ok := fields["serviceTier"]; ok {
			t.Fatal("nil tier became present")
		}
	} else {
		var selected string
		if err := json.Unmarshal(fields["serviceTier"], &selected); err != nil || selected != string(*tier) {
			t.Fatalf("outbound tier changed: %s %v", fields["serviceTier"], err)
		}
	}
	return nil
}

func TestOpenServiceTierAdmission(t *testing.T) {
	for _, method := range []string{"thread/start", "thread/fork", "thread/resume", "turn/start"} {
		t.Run(method, func(t *testing.T) {
			if err := callOpenTierRequest(t, method, nil); err != nil {
				t.Fatal(err)
			}
			if err := callOpenTierRequest(t, method, p.Ptr(p.ServiceTierFast)); err != nil {
				t.Fatal("known control:", err)
			}
			for _, v := range []p.ServiceTier{"priority", "default", ""} {
				if err := callOpenTierRequest(t, method, &v); err != nil {
					t.Errorf("schema string %q rejected: %v", v, err)
				}
			}
		})
	}
	for _, method := range []string{"thread/start", "thread/fork", "thread/resume"} {
		t.Run("response/"+method, func(t *testing.T) {
			mock := NewMockTransport()
			c := p.NewClient(mock)
			t.Cleanup(func() { _ = c.Close() })
			response := validThreadLifecycleResponse(validThreadPayload("thread"))
			response["serviceTier"] = "priority"
			if err := mock.SetResponseData(method, response); err != nil {
				t.Fatal(err)
			}
			var err error
			switch method {
			case "thread/start":
				_, err = c.Thread.Start(context.Background(), p.ThreadStartParams{})
			case "thread/fork":
				_, err = c.Thread.Fork(context.Background(), p.ThreadForkParams{ThreadID: "thread"})
			case "thread/resume":
				_, err = c.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "thread"})
			}
			if err != nil {
				t.Errorf("schema-valid returned tier rejected: %v", err)
			}
		})
	}
}

func TestOpenThreadSourceAdmission(t *testing.T) {
	for _, value := range []p.ThreadSource{"user", "guardian_review", "custom-client", ""} {
		t.Run(string(value), func(t *testing.T) {
			var decoded p.ThreadSource
			raw, _ := json.Marshal(string(value))
			err := json.Unmarshal(raw, &decoded)
			if err != nil || decoded != value {
				t.Errorf("schema string decode: %v", err)
			}
			_, err = json.Marshal(value)
			if err != nil {
				t.Errorf("schema string encode: %v", err)
			}
			for _, route := range []string{"thread/start", "thread/fork", "thread/read"} {
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				thread := validThreadPayload("thread")
				thread["threadSource"] = string(value)
				var response any = validThreadLifecycleResponse(thread)
				if route == "thread/read" {
					response = map[string]any{"thread": thread}
				}
				if err := mock.SetResponseData(route, response); err != nil {
					t.Fatal(err)
				}
				var err error
				var returned p.Thread
				switch route {
				case "thread/start":
					var r p.ThreadStartResponse
					r, err = c.Thread.Start(context.Background(), p.ThreadStartParams{ThreadSource: &value})
					returned = r.Thread
				case "thread/fork":
					var r p.ThreadForkResponse
					r, err = c.Thread.Fork(context.Background(), p.ThreadForkParams{ThreadID: "thread", ThreadSource: &value})
					returned = r.Thread
				case "thread/read":
					var r p.ThreadReadResponse
					r, err = c.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "thread"})
					returned = r.Thread
				}
				if err != nil || returned.ThreadSource == nil || *returned.ThreadSource != value {
					t.Fatalf("%s rejected or changed schema string: %v", route, err)
				}
				if route != "thread/read" {
					var fields map[string]json.RawMessage
					if err := json.Unmarshal(mock.GetSentRequest(0).Params, &fields); err != nil {
						t.Fatal(err)
					}
					if string(fields["threadSource"]) != string(raw) {
						t.Fatal("outbound source changed")
					}
				}
			}
		})
	}
}
