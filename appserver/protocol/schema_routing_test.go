package protocol_test

import (
	"context"
	"encoding/json"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSyncedRealtimeBranches(t *testing.T) {
	for _, body := range []string{
		`{"type":"frame","channels":{"voice_transcript":{"role":"user","text":""}}}`,
		`{"type":"turnInputAccepted","handoff_id":"h","input":"","input_item_id":"i","mode":"steer","source_event_ids":[],"turn_id":"u"}`,
		`{"type":"sessionInputSaved","input":"","input_item_id":"i","reason":"chunk","source_event_ids":[]}`,
		`{"type":"transcriptSegment","role":"user","text":""}`,
	} {
		body = strings.TrimSuffix(body, "}") + `,"id":"i","realtimeSessionId":"s","observedAtMs":-1}`
		var v p.ThreadRealtimeItemWrapper
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Fatal(err)
		}
		if _, unknown := v.Value.(*p.UnknownThreadRealtimeItem); unknown {
			t.Fatalf("known branch became unknown: %s", body)
		}
		out, err := json.Marshal(v)
		if err != nil || !strings.Contains(string(out), `"observedAtMs":-1`) {
			t.Fatalf("lost timestamp: %s %v", out, err)
		}
	}
	for _, body := range []string{`{"type":"frame"}`, `{"type":"frame","channels":{}}`, `{"type":"turnInputAccepted","source_event_ids":[null]}`, `{"type":"sessionInputSaved"}`} {
		var v p.ThreadRealtimeItemWrapper
		if err := json.Unmarshal([]byte(strings.TrimSuffix(body, "}")+`,"id":"i","realtimeSessionId":"s"}`), &v); err == nil {
			t.Fatalf("accepted incomplete branch: %s", body)
		}
	}
}

func TestSyncedMcpSelectors(t *testing.T) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	defer c.Close()
	for _, tc := range []struct {
		method, body, response string
		call                   func([]byte) error
	}{
		{"mcpServer/tool/call", `{"threadId":"t","server":"s","tool":"tool","environmentId":""}`, `{"content":[]}`, func(b []byte) error {
			var v p.McpServerToolCallParams
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			_, err := c.Mcp.ToolCall(context.Background(), v)
			return err
		}},
		{"mcpServer/resource/read", `{"server":"s","uri":"r","environmentId":""}`, `{"contents":[]}`, func(b []byte) error {
			var v p.McpResourceReadParams
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			_, err := c.Mcp.ResourceRead(context.Background(), v)
			return err
		}},
		{"mcpServer/oauth/login", `{"name":"s","environmentId":""}`, `{"authorizationUrl":"opaque:u"}`, func(b []byte) error {
			var v p.McpServerOauthLoginParams
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			_, err := c.Mcp.OauthLogin(context.Background(), v)
			return err
		}},
		{"mcpServerStatus/list", `{"environmentId":""}`, `{"data":[]}`, func(b []byte) error {
			var v p.ListMcpServerStatusParams
			if err := json.Unmarshal(b, &v); err != nil {
				return err
			}
			_, err := c.Mcp.ListServerStatus(context.Background(), v)
			return err
		}},
	} {
		tr.SetResponse(tc.method, p.Response{Result: json.RawMessage(tc.response)})
		if err := tc.call([]byte(tc.body)); err != nil {
			t.Fatal(err)
		}
		if got := string(tr.GetSentRequest(tr.CallCount() - 1).Params); !strings.Contains(got, `"environmentId":""`) {
			t.Errorf("%s lost selector: %s", tc.method, got)
		}
	}
}

func TestIncludedThreadSelection(t *testing.T) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	defer c.Close()
	tr.SetResponse("thread/list", p.Response{Result: json.RawMessage(`{"data":[]}`)})
	for _, raw := range []string{`null`, `[]`, `["t",""]`} {
		body := []byte(`{"includedThreadIds":` + raw + `}`)
		var legacy p.ThreadListParams
		var full p.ThreadListParamsJSON
		if err := json.Unmarshal(body, &legacy); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &full); err != nil {
			t.Fatal(err)
		}
		_, err := c.Thread.List(context.Background(), legacy)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(tr.GetSentRequest(tr.CallCount() - 1).Params); !strings.Contains(got, `"includedThreadIds":`+raw) {
			t.Errorf("legacy lost selection %s", got)
		}
		_, err = c.Thread.ListJSON(context.Background(), full)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(tr.GetSentRequest(tr.CallCount() - 1).Params); !strings.Contains(got, `"includedThreadIds":`+raw) {
			t.Errorf("full lost selection %s", got)
		}
	}
}

func TestSyncedAccountAndTelemetry(t *testing.T) {
	var n p.AccountUpdatedNotification
	if err := json.Unmarshal([]byte(`{"reason":"workspaceRoutingReady"}`), &n); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(n)
	if err != nil || !strings.Contains(string(b), `"reason":"workspaceRoutingReady"`) {
		t.Errorf("reason lost: %s %v", b, err)
	}
	for _, raw := range []string{`1`, `18446744073709551615`} {
		var o p.OtelRequirements
		if err := json.Unmarshal([]byte(`{"headers_helper":"opaque command","headers_helper_refresh_interval_ms":`+raw+`}`), &o); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(o)
		if err != nil || !strings.Contains(string(b), `"headers_helper":"opaque command"`) || !strings.Contains(string(b), `"headers_helper_refresh_interval_ms":`+raw) {
			t.Errorf("helper lost: %s %v", b, err)
		}
	}
	var o p.OtelRequirements
	if err := json.Unmarshal([]byte(`{"headers_helper_refresh_interval_ms":0}`), &o); err == nil {
		t.Error("accepted zero refresh interval")
	}
}

func TestDefaultProviderSelection(t *testing.T) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	defer c.Close()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := tr.SetResponseData("modelProvider/default/set", map[string]any{"filePath": path, "status": "ok", "version": "v"}); err != nil {
		t.Fatal(err)
	}
	r, err := c.ModelProvider.DefaultSet(context.Background(), p.ModelProviderDefaultSetParams{ProviderID: ""})
	if err != nil || r.Version != "v" || r.FilePath == "" {
		t.Fatalf("response=%+v err=%v", r, err)
	}
	if got := string(tr.GetSentRequest(0).Params); got != `{"providerId":""}` {
		t.Fatalf("provider selection=%s", got)
	}
}

func TestSearchPartialCallbacks(t *testing.T) {
	tr := NewMockTransport()
	errors, calls := 0, 0
	c := p.NewClient(tr, p.WithHandlerErrorCallback(func(_ string, _ error) { errors++ }))
	defer c.Close()
	c.OnThreadSearchPartial(func(n p.ThreadSearchPartialNotification) {
		calls++
		if len(n.Data) > 0 {
			n.Data[0].Snippet = "mutated"
		}
	})
	c.AddThreadSearchPartialListener(func(n p.ThreadSearchPartialNotification) {
		calls++
		if len(n.Data) != 0 && n.Data[0].Snippet != "match" {
			t.Error("recipient mutation leaked")
		}
	})
	for _, id := range []any{"", int64(0), int64(-1), int64(9223372036854775807)} {
		body, err := json.Marshal(map[string]any{"requestId": id, "data": []any{map[string]any{"snippet": "match", "thread": validProcessThreadPayload("t")}}})
		if err != nil {
			t.Fatal(err)
		}
		tr.InjectServerNotification(context.Background(), p.Notification{Method: p.NotifyThreadSearchPartial, Params: body})
	}
	if calls != 8 || errors != 0 {
		t.Fatalf("valid callbacks=%d errors=%d", calls, errors)
	}
	if _, ok := c.ThreadStateSnapshot("t"); ok {
		t.Error("provisional row populated authoritative thread cache")
	}
	for _, body := range []string{`null`, `{}`, `{"data":[],"requestId":null}`, `{"data":[null],"requestId":1}`, `{"data":[{"snippet":"s"}],"requestId":1}`, `{"data":[],"requestId":1.2}`} {
		tr.InjectServerNotification(context.Background(), p.Notification{Method: p.NotifyThreadSearchPartial, Params: json.RawMessage(body)})
	}
	if calls != 8 || errors != 12 {
		t.Fatalf("malformed callbacks=%d errors=%d", calls, errors)
	}
}

func TestRealtimePublicationAndIsolation(t *testing.T) {
	for _, branch := range []string{`"type":"realtimeSessionStarted"`, `"type":"realtimeSessionClosed","outcome":"ended"`, `"type":"transcriptSegment","role":"assistant","text":""`, `"type":"bemItemPromoted","item_id":"i","turn_id":"u","presentation":{"type":"wholeItem"}`, `"type":"frame","channels":{"voice_transcript":{"role":"user","text":""}}`, `"type":"turnInputAccepted","handoff_id":"","input":"","input_item_id":"","mode":"start","source_event_ids":["a"],"turn_id":""`, `"type":"sessionInputSaved","input":"","input_item_id":"","reason":"sessionEnd","source_event_ids":[]`} {
		tr := NewMockTransport()
		calls, errors := 0, 0
		c := p.NewClient(tr, p.WithHandlerErrorCallback(func(_ string, _ error) { errors++ }))
		defer c.Close()
		check := func(item p.ThreadRealtimeItemWrapper) {
			calls++
			b, err := json.Marshal(item)
			if err != nil || !strings.Contains(string(b), `"observedAtMs":0`) {
				t.Errorf("timestamp lost: %s %v", b, err)
			}
		}
		c.OnThreadRealtimeItemStarted(func(n p.ThreadRealtimeItemStartedNotification) { check(n.Item) })
		c.AddThreadRealtimeItemStartedListener(func(n p.ThreadRealtimeItemStartedNotification) { check(n.Item) })
		c.OnThreadRealtimeItemCompleted(func(n p.ThreadRealtimeItemCompletedNotification) { check(n.Item) })
		c.AddThreadRealtimeItemCompletedListener(func(n p.ThreadRealtimeItemCompletedNotification) { check(n.Item) })
		for _, method := range []string{"thread/realtime/item/started", "thread/realtime/item/completed"} {
			tr.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","item":{"id":"i","realtimeSessionId":"s","observedAtMs":0,` + branch + `}}`)})
		}
		if calls != 4 || errors != 0 {
			t.Fatalf("callbacks=%d errors=%d", calls, errors)
		}
		for _, method := range []string{"thread/realtime/item/started", "thread/realtime/item/completed"} {
			tr.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","item":{"type":"frame","id":"i","realtimeSessionId":"s","channels":null}}`)})
		}
		if calls != 4 || errors != 4 {
			t.Fatalf("malformed callback published=%d errors=%d", calls, errors)
		}
	}
}

func TestTelemetryIntervalOccurrences(t *testing.T) {
	for _, raw := range []string{`0`, `-1`, `1.2`, `18446744073709551616`, `"1"`} {
		for _, suffix := range []string{"", `,"headers_helper_refresh_interval_ms":1`} {
			var o p.OtelRequirements
			if err := json.Unmarshal([]byte(`{"headers_helper_refresh_interval_ms":`+raw+suffix+`}`), &o); err == nil {
				t.Fatalf("accepted invalid interval occurrence %s", raw+suffix)
			}
		}
	}
	for _, raw := range []string{`null`, `1`, `18446744073709551615`} {
		var o p.OtelRequirements
		if err := json.Unmarshal([]byte(`{"headers_helper":"","headers_helper_refresh_interval_ms":`+raw+`}`), &o); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewOwnersIgnoreNonSchemaAliases(t *testing.T) {
	for _, body := range []string{
		`{"id":"i","realtimeSessionId":"s","type":"frame","channels":{"voice_transcript":{"role":"user","text":"ok"}},"CHANNELS":{"voice_transcript":null}}`,
		`{"id":"i","realtimeSessionId":"s","type":"sessionInputSaved","input":"","input_item_id":"","reason":"chunk","source_event_ids":[],"SOURCE_EVENT_IDS":[null]}`,
	} {
		var v p.ThreadRealtimeItemWrapper
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Errorf("ignored field affected admission: %v", err)
		}
	}
	var n p.ThreadSearchPartialNotification
	if err := json.Unmarshal([]byte(`{"requestId":1,"data":[],"DATA":[null]}`), &n); err != nil {
		t.Errorf("ignored data alias affected admission: %v", err)
	}
}

func TestAccountReasonAliases(t *testing.T) {
	for _, body := range []string{`{"REASON":"future"}`, `{"REASON":"future","reason":"unknown"}`, `{"rea\u0073on":"future","reason":null}`} {
		var n p.AccountUpdatedNotification
		if err := json.Unmarshal([]byte(body), &n); err == nil {
			t.Errorf("invalid recognized reason accepted: %s", body)
		}
	}
	for _, body := range []string{`{}`, `{"reason":null}`, `{"REASON":"unknown"}`, `{"reason":"workspaceRoutingReady"}`} {
		var n p.AccountUpdatedNotification
		if err := json.Unmarshal([]byte(body), &n); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFrameNestedMergeAndAliases(t *testing.T) {
	for _, body := range []string{
		`"channels":{"VOICE_TRANSCRIPT":{"ROLE":"user","TEXT":""}}`,
		`"channels":{"voice_transcript":{"role":"user"}},"channels":{"voice_transcript":{"text":""}}`,
	} {
		var item p.ThreadRealtimeItemWrapper
		if err := json.Unmarshal([]byte(`{"id":"i","realtimeSessionId":"s","type":"frame",`+body+`}`), &item); err != nil {
			t.Errorf("native nested merge: %v", err)
		}
	}
}

func TestMcpEnvironmentOutputs(t *testing.T) {
	tr := NewMockTransport()
	calls := 0
	c := p.NewClient(tr)
	defer c.Close()
	// Catalog records and both notification APIs retain the same opaque scope.
	tr.SetResponse("mcpServerStatus/list", p.Response{Result: json.RawMessage(`{"data":[{"authStatus":"notLoggedIn","name":"s","resourceTemplates":[],"resources":[],"tools":{},"environmentId":"relative/../opaque"}]}`)})
	r, err := c.Mcp.ListServerStatus(context.Background(), p.ListMcpServerStatusParams{})
	if err != nil || len(r.Data) != 1 || r.Data[0].EnvironmentID == nil || *r.Data[0].EnvironmentID != "relative/../opaque" {
		t.Fatalf("catalog=%+v %v", r, err)
	}
	c.OnMcpServerStatusUpdated(func(n p.McpServerStatusUpdatedNotification) {
		calls++
		if n.EnvironmentID == nil || *n.EnvironmentID != "" {
			t.Error("notification scope lost")
		}
	})
	c.AddMcpServerStatusUpdatedListener(func(n p.McpServerStatusUpdatedNotification) {
		calls++
		if n.EnvironmentID == nil || *n.EnvironmentID != "" {
			t.Error("listener scope lost")
		}
	})
	tr.InjectServerNotification(context.Background(), p.Notification{Method: "mcpServer/startupStatus/updated", Params: json.RawMessage(`{"name":"s","status":"ready","environmentId":""}`)})
	if calls != 2 {
		t.Fatalf("status callbacks=%d", calls)
	}
	var item p.ThreadItemWrapper
	if err := json.Unmarshal([]byte(`{"type":"mcpToolCall","id":"i","server":"s","tool":"tool","status":"completed","arguments":{},"environmentId":"relative/../opaque"}`), &item); err != nil {
		t.Fatal(err)
	}
	if v := item.Value.(*p.McpToolCallThreadItem); v.EnvironmentID == nil || *v.EnvironmentID != "relative/../opaque" {
		t.Fatal("history scope lost")
	}
	c.SetApprovalHandlers(p.ApprovalHandlers{OnMcpServerElicitationRequest: func(_ context.Context, n p.McpServerElicitationRequestParams) (p.McpServerElicitationRequestResponse, error) {
		calls++
		if n.EnvironmentID == nil || *n.EnvironmentID != "" {
			t.Error("elicitation scope lost")
		}
		return p.McpServerElicitationRequestResponse{Action: p.McpServerElicitationActionCancel}, nil
	}})
	_, err = tr.InjectServerRequest(context.Background(), p.Request{ID: p.RequestID{Value: "e"}, Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"serverName":"s","threadId":"t","mode":"form","message":"m","requestedSchema":{"type":"object","properties":{}},"environmentId":""}`)})
	if err != nil || calls != 3 {
		t.Fatalf("elicitation callbacks=%d %v", calls, err)
	}
}

func TestSearchRowsReuseBackingStorage(t *testing.T) {
	row := func(id string) string {
		b, err := json.Marshal(map[string]any{"snippet": id, "thread": validProcessThreadPayload(id)})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	a, b := row("a"), row("b")
	threadA, err := json.Marshal(validProcessThreadPayload("a"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		arrays string
		valid  bool
	}{
		{`[` + a + `],"data":[{"snippet":"next"}]`, true},
		{`[` + a + `],"data":[{"thread":` + string(threadA) + `}]`, true},
		{`[` + a + `,` + b + `],"data":[{"snippet":"short"}],"data":[{},{}]`, true},
		{`[` + a + `],"data":[],"data":[{}]`, false},
		{`[` + a + `],"data":[null],"data":[` + a + `]`, false},
		{`[` + a + `],"data":[{"snippet":null}],"data":[` + a + `]`, false},
	} {
		body := []byte(`{"requestId":1,"data":` + tc.arrays + `}`)
		if !json.Valid(body) {
			t.Fatalf("invalid fixture %s", body)
		}
		type native p.ThreadSearchPartialNotification
		var expected native
		if err := json.Unmarshal(body, &expected); err != nil {
			t.Fatal(err)
		}
		var got p.ThreadSearchPartialNotification
		err := json.Unmarshal(body, &got)
		if (err == nil) != tc.valid {
			t.Errorf("reuse valid=%v err=%v body=%s", tc.valid, err, body)
		}
		if err == nil && !reflect.DeepEqual(got, p.ThreadSearchPartialNotification(expected)) {
			t.Errorf("native row reuse changed")
		}
	}
}

func FuzzSearchPartialRows(f *testing.F) {
	for i := range 11 {
		f.Add([]byte{byte(i)})
	}
	f.Add([]byte{0, 1, 3, 8, 1, 3})
	f.Add([]byte{8, 1, 9})
	f.Add([]byte{0, 4, 3})
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 24 {
			ops = ops[:24]
		}
		thread, err := json.Marshal(validProcessThreadPayload("t"))
		if err != nil {
			t.Fatal(err)
		}
		full := `{"snippet":"match","thread":` + string(thread) + `}`
		choices := []string{`[` + full + `]`, `[{"snippet":"next"}]`, `[{"thread":` + string(thread) + `}]`, `[{}]`, `[]`, `[null]`, `[{"snippet":null}]`, `[{"thread":null}]`, `[` + full + `,` + full + `]`, `[` + full + `,{}]`, `null`}
		body := `{"requestId":1`
		forbidden := false
		for _, op := range ops {
			n := int(op) % len(choices)
			body += `,"data":` + choices[n]
			forbidden = forbidden || n == 5 || n == 6 || n == 7 || n == 10
		}
		body += "}"
		// Native pointers independently model required presence across backing reuse;
		// explicit forbidden events come from the generator, not production helpers.
		var model struct {
			Data []struct {
				Snippet *string
				Thread  *p.Thread
			}
		}
		if err := json.Unmarshal([]byte(body), &model); err != nil {
			t.Fatal(err)
		}
		valid := len(ops) > 0 && !forbidden
		for _, row := range model.Data {
			valid = valid && row.Snippet != nil && row.Thread != nil
		}
		var got p.ThreadSearchPartialNotification
		err = json.Unmarshal([]byte(body), &got)
		if (err == nil) != valid {
			t.Fatalf("valid=%v err=%v ops=%v", valid, err, ops)
		}
		if err != nil {
			return
		}
		if len(got.Data) != len(model.Data) {
			t.Fatal("row count changed")
		}
		for i, row := range got.Data {
			if row.Snippet != *model.Data[i].Snippet || row.Thread.ID != model.Data[i].Thread.ID {
				t.Fatal("native merged row changed")
			}
		}
	})
}
