package protocol_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestFunctionAudioAlternatives(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"type":"input_audio","file_id":"f"}`, true},
		{`{"type":"input_audio","file_id":""}`, true},
		{`{"type":"input_audio","audio_url":""}`, true},
		{`{"type":"input_audio","audio_url":false,"file_id":"f"}`, true},
		{`{"type":"input_audio","audio_url":"u","file_id":[]}`, true},
		{`{"type":"input_audio","audio_url":"u","file_id":"f"}`, true},
		{`{"type":"input_audio"}`, false},
		{`{"type":"input_audio","audio_url":null,"file_id":null}`, false},
		{`{"type":"input_audio","audio_url":3,"file_id":false}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var got p.FunctionCallOutputContentItemWrapper
			err := json.Unmarshal([]byte(tc.body), &got)
			if (err == nil) != tc.valid {
				t.Fatalf("admission=%v", err)
			}
			if !tc.valid {
				return
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var again p.FunctionCallOutputContentItemWrapper
			if err = json.Unmarshal(data, &again); err != nil {
				t.Fatalf("roundtrip %s: %v", data, err)
			}
		})
	}
}

func TestNewSchemaRequests(t *testing.T) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	tr.SetResponse("model/list", p.Response{Result: json.RawMessage(`{"data":[]}`)})
	if _, err := c.Model.List(ctx, p.ModelListParams{ModelProvider: p.Ptr("")}); err != nil {
		t.Fatal(err)
	}
	assertLast := func(fragment string) {
		t.Helper()
		if got := string(tr.GetSentRequest(tr.CallCount() - 1).Params); !strings.Contains(got, fragment) {
			t.Fatalf("request=%s want %s", got, fragment)
		}
	}
	assertLast(`"modelProvider":""`)
	tr.SetResponse("thread/items/list", p.Response{Result: json.RawMessage(`{"data":[],"nextCursor":null}`)})
	for _, raw := range []string{`null`, `{}`, `{"future":false}`} {
		var params p.ThreadItemsListParams
		if err := json.Unmarshal([]byte(`{"threadId":"t","turnId":"u","deferOutput":`+raw+`}`), &params); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Thread.ItemsList(ctx, params); err != nil {
			t.Fatal(err)
		}
		assertLast(`"deferOutput":` + raw)
	}
	tr.SetResponse("mcpServer/resource/read", p.Response{Result: json.RawMessage(`{"contents":[]}`)})
	for _, raw := range []string{`null`, `{}`, `{"opaque":9007199254740993}`} {
		var params p.McpResourceReadParams
		if err := json.Unmarshal([]byte(`{"server":"s","uri":"opaque:r","_meta":`+raw+`}`), &params); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Mcp.ResourceRead(ctx, params); err != nil {
			t.Fatal(err)
		}
		assertLast(`"_meta":` + raw)
	}
}

func TestGatewayDeviceFlowAndSecuritySetup(t *testing.T) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	tr.SetResponse("account/gatewayOAuth/read", p.Response{Result: json.RawMessage(`{"providerId":"p","providerName":"P","required":true,"supportsDeviceAuth":true}`)})
	read, err := c.Account.GatewayOAuthReadWithParams(ctx, &p.GatewayOAuthReadParams{ModelProvider: p.Ptr("p")})
	if err != nil || !read.SupportsDeviceAuth {
		t.Fatalf("read=%+v err=%v", read, err)
	}
	if got := string(tr.GetSentRequest(0).Params); got != `{"modelProvider":"p"}` {
		t.Fatalf("read params=%s", got)
	}
	tr.SetResponse("account/gatewayOAuth/login", p.Response{Result: json.RawMessage(`{"prompt":{"loginId":"l","userCode":"code","verificationUrl":"opaque:url"}}`)})
	login, err := c.Account.GatewayOAuthLoginWithParams(ctx, &p.GatewayOAuthLoginParams{DeviceAuth: p.Ptr(true), ModelProvider: p.Ptr("p")})
	if err != nil || login.Prompt == nil || login.Prompt.LoginID != "l" || login.Prompt.UserCode != "code" || login.Prompt.VerificationURL != "opaque:url" {
		t.Fatalf("login=%+v err=%v", login, err)
	}
	if got := string(tr.GetSentRequest(1).Params); !strings.Contains(got, `"deviceAuth":true`) || !strings.Contains(got, `"modelProvider":"p"`) {
		t.Fatalf("login params=%s", got)
	}
	// Existing convenience methods still send null params and retain the prompt.
	login, err = c.Account.GatewayOAuthLogin(ctx)
	if err != nil || login.Prompt == nil {
		t.Fatalf("legacy login=%+v %v", login, err)
	}
	var changed p.GatewayOAuthChangedNotification
	c.OnGatewayOAuthChanged(func(n p.GatewayOAuthChangedNotification) { changed = n })
	calls := 0
	c.AddGatewayOAuthChangedListener(func(n p.GatewayOAuthChangedNotification) {
		calls++
		if n.LoginID == nil || *n.LoginID != "l" {
			t.Errorf("listener=%+v", n)
		}
	})
	tr.InjectServerNotification(ctx, p.Notification{Method: p.NotifyGatewayOAuthChanged, Params: json.RawMessage(`{"providerId":"p","status":"started","loginId":"l"}`)})
	if calls != 1 || changed.LoginID == nil || *changed.LoginID != "l" {
		t.Fatalf("changed=%+v calls=%d", changed, calls)
	}
	tr.SetResponse("account/securitySetup/read", p.Response{Result: json.RawMessage(`{"notice":{"accountId":"a","userId":"u","title":"T","description":"D","action":{"label":"L","url":"opaque:action"}}}`)})
	notice, err := c.Account.SecuritySetupRead(ctx)
	if err != nil || notice.Notice == nil || notice.Notice.UserID != "u" || notice.Notice.Action.URL != "opaque:action" {
		t.Fatalf("notice=%+v err=%v", notice, err)
	}
	tr.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"account":null,"requiresOpenaiAuth":false,"userId":""}`)})
	account, err := c.Account.Get(ctx, p.GetAccountParams{})
	if err != nil || account.UserID == nil || *account.UserID != "" {
		t.Fatalf("account=%+v %v", account, err)
	}
}

func TestAgentModerationAdmission(t *testing.T) {
	for _, tc := range []struct {
		meta  string
		valid bool
	}{
		{`null`, true}, {`{"moderated":false}`, true},
		{`{"moderated":true,"display":{"action":"future","checkTrustedContacts":false,"cyberTrustedAccess":true,"blockType":""}}`, true},
		{`{}`, false}, {`{"moderated":null}`, false}, {`{"moderated":true,"display":{}}`, false},
		{`{"moderated":true,"display":{"action":null}}`, false},
	} {
		t.Run(tc.meta, func(t *testing.T) {
			var got p.ThreadItemWrapper
			err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"hello","moderation":`+tc.meta+`}`), &got)
			if (err == nil) != tc.valid {
				t.Fatalf("admission=%v", err)
			}
			if !tc.valid {
				return
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if tc.meta != "null" && !strings.Contains(string(data), `"moderated":`) {
				t.Fatalf("lost moderation: %s", data)
			}
		})
	}
}

func TestMcpRequestMetadataRejectsNonObjects(t *testing.T) {
	for _, raw := range []string{`[]`, `false`, `"x"`, `1`, `{`} {
		tr := NewMockTransport()
		c := p.NewClient(tr)
		t.Cleanup(func() { _ = c.Close() })
		_, err := c.Mcp.ResourceRead(context.Background(), p.McpResourceReadParams{Server: "s", URI: "r", Meta: json.RawMessage(raw)})
		if err == nil || tr.CallCount() != 0 {
			t.Fatalf("meta=%s err=%v calls=%d", raw, err, tr.CallCount())
		}
	}
}

func TestNestedSchemaRecordMerges(t *testing.T) {
	for _, tc := range []struct {
		members string
		valid   bool
	}{
		{`"moderation":{"moderated":false},"MODERATION":{"display":{"action":"future"}}`, true},
		{`"moderation":{"moderated":true,"display":{"action":"future"}},"moderation":{"display":{"blockType":"opaque"}}`, true},
		{`"moderation":{"moderated":true},"moderation":null`, true},
		{`"moderation":{"moderated":true},"moderation":null,"moderation":{}`, false},
		{`"moderation":{"moderated":null},"moderation":{"moderated":false}`, false},
		{`"moderation":{"moderated":false,"display":{"action":null}},"moderation":{"display":{"action":"future"}}`, false},
		{`"moderation":{"moderated":false,"display":{"action":"future"}},"moderation":{"display":null}`, true},
		{`"moderation":{"moderated":false,"display":{"action":"future"}},"moderation":{"display":null},"moderation":{"display":{}}`, false},
	} {
		var item p.ThreadItemWrapper
		err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"t",`+tc.members+`}`), &item)
		if (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.members, err)
		}
	}
	for _, body := range []string{`{"prompt":{}}`, `{"prompt":{"loginId":"l","userCode":null,"verificationUrl":"u"}}`, `{"prompt":{"loginId":"l","userCode":"c"},"prompt":null,"prompt":{"verificationUrl":"u"}}`} {
		if _, err := gatewayResult(body); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	r, err := gatewayResult(`{"prompt":{"loginId":"","userCode":""},"PROMPT":{"verificationUrl":""}}`)
	if err != nil || r.Prompt == nil {
		t.Fatalf("prompt merge=%+v %v", r, err)
	}
	for _, body := range []string{`{"notice":{}}`, `{"notice":{"accountId":"a","userId":"u","title":"t","description":"d","action":null}}`, `{"notice":{"accountId":"a","userId":"u","title":"t","description":"d","action":{"label":"l"}}}`} {
		if _, err := securityResult(body); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	notice, err := securityResult(`{"notice":{"accountId":"","userId":"","title":"","description":"","action":{"label":""}},"notice":{"action":{"url":""}}}`)
	if err != nil || notice.Notice == nil {
		t.Fatalf("notice merge=%+v %v", notice, err)
	}
	for _, body := range []string{`{}`, `{"notice":null}`} {
		notice, err := securityResult(body)
		if err != nil || notice.Notice != nil {
			t.Fatalf("absent notice: %+v %v", notice, err)
		}
	}
}

func TestAgentModerationPublicRoutesAndOwnership(t *testing.T) {
	for _, valid := range []bool{true, false} {
		tr := NewMockTransport()
		calls, failures := 0, 0
		c := p.NewClient(tr, p.WithHandlerErrorCallback(func(_ string, _ error) { failures++ }))
		t.Cleanup(func() { _ = c.Close() })
		moderation := `{"moderated":false,"display":{"action":"future","checkTrustedContacts":false}}`
		if !valid {
			moderation = `{"display":{"action":"future"}}`
		}
		item := json.RawMessage(`{"type":"agentMessage","id":"i","text":"t","moderation":` + moderation + `}`)
		thread := validProcessThreadPayload("t")
		thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []json.RawMessage{item}}}
		if err := tr.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
			t.Fatal(err)
		}
		response, err := c.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
		if (err == nil) != valid {
			t.Fatalf("read=%v", err)
		}
		if valid {
			snapshot, ok := c.ThreadStateSnapshot("t")
			if !ok {
				t.Fatal("missing cached response")
			}
			get := func(x p.Thread) *p.AgentMessageModeration {
				return x.Turns[0].Items[0].Value.(*p.AgentMessageThreadItem).Moderation
			}
			get(response.Thread).Display.Action = "response mutation"
			if get(snapshot).Display.Action != "future" {
				t.Fatal("response mutation leaked")
			}
			get(snapshot).Display.CheckTrustedContacts = p.Ptr(true)
			next, _ := c.ThreadStateSnapshot("t")
			if *get(next).Display.CheckTrustedContacts {
				t.Fatal("snapshot mutation leaked")
			}
		}
		check := func(x p.ThreadItemWrapper) {
			calls++
			m := x.Value.(*p.AgentMessageThreadItem).Moderation
			if m == nil || m.Moderated || m.Display == nil || m.Display.Action != "future" {
				t.Errorf("callback=%+v", m)
			}
		}
		c.OnItemStarted(func(n p.ItemStartedNotification) { check(n.Item) })
		c.AddItemStartedListener(func(n p.ItemStartedNotification) { check(n.Item) })
		c.OnItemCompleted(func(n p.ItemCompletedNotification) { check(n.Item) })
		c.AddItemCompletedListener(func(n p.ItemCompletedNotification) { check(n.Item) })
		for _, method := range []string{p.NotifyItemStarted, p.NotifyItemCompleted} {
			tr.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":0,"completedAtMs":0,"item":` + string(item) + `}`)})
		}
		if valid && calls != 4 || !valid && (calls != 0 || failures != 4) {
			t.Fatalf("valid=%v calls=%d failures=%d", valid, calls, failures)
		}
	}
}

func TestFunctionAudioPublicRoutes(t *testing.T) {
	for _, valid := range []bool{true, false} {
		tr := NewMockTransport()
		c := p.NewClient(tr)
		t.Cleanup(func() { _ = c.Close() })
		var content p.FunctionCallOutputContentItemWrapper
		body := `{"type":"input_audio","audio_url":false,"file_id":"file"}`
		if err := json.Unmarshal([]byte(body), &content); err != nil {
			t.Fatal(err)
		}
		tr.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"u","items":[],"status":"inProgress"}}`)})
		if !valid {
			content.Value = &p.InputAudioFunctionCallOutputContentItem{}
		}
		_, err := c.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "t", Input: []p.UserInput{}, ToolOutput: &p.TurnToolOutput{Name: "tool", Output: p.FunctionCallOutputBody{Content: []p.FunctionCallOutputContentItemWrapper{content}}}})
		if (err == nil) != valid || !valid && tr.CallCount() != 0 {
			t.Fatalf("start valid=%v err=%v calls=%d", valid, err, tr.CallCount())
		}
		if valid && !strings.Contains(string(tr.GetSentRequest(0).Params), `"file_id":"file"`) {
			t.Fatal("lost file request")
		}
		audio := `{"type":"input_audio","file_id":"file"}`
		if !valid {
			audio = `{"type":"input_audio","file_id":null}`
		}
		item := `{"type":"functionCallOutput","id":"i","name":"tool","output":[` + audio + `]}`
		thread := validProcessThreadPayload("t")
		thread["turns"] = []any{map[string]any{"id": "u", "status": "completed", "items": []json.RawMessage{json.RawMessage(item)}}}
		if err := tr.SetResponseData("thread/read", map[string]any{"thread": thread}); err != nil {
			t.Fatal(err)
		}
		response, err := c.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
		if (err == nil) != valid {
			t.Fatalf("history valid=%v err=%v", valid, err)
		}
		if valid {
			v := response.Thread.Turns[0].Items[0].Value.(*p.FunctionCallOutputThreadItem).Output.Content[0].Value.(*p.InputAudioFunctionCallOutputContentItem)
			if v.FileID == nil || *v.FileID != "file" {
				t.Fatalf("history=%+v", v)
			}
		}
	}
}

func FuzzFunctionAudioAlternatives(f *testing.F) {
	for a := range 7 {
		for b := range 7 {
			f.Add(uint8(a), uint8(b), "opaque:audio")
		}
	}
	f.Fuzz(func(t *testing.T, a, b uint8, text string) {
		choices := []any{nil, 0, true, []any{}, map[string]any{}, "", text}
		url, file := choices[int(a)%len(choices)], choices[int(b)%len(choices)]
		body, err := json.Marshal(map[string]any{"type": "input_audio", "audio_url": url, "file_id": file})
		if err != nil {
			t.Fatal(err)
		}
		_, u := url.(string)
		_, v := file.(string)
		var got p.FunctionCallOutputContentItemWrapper
		err = json.Unmarshal(body, &got)
		if (err == nil) != (u || v) {
			t.Fatalf("anyOf %s: %v", body, err)
		}
		if err != nil {
			return
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		var original map[string]json.RawMessage
		if err := json.Unmarshal(body, &original); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"audio_url", "file_id"} {
			var s string
			if string(original[name]) != "null" && json.Unmarshal(original[name], &s) == nil && string(fields[name]) != string(original[name]) {
				t.Fatalf("lost alternative: %s -> %s", body, data)
			}
		}
	})
}

func TestPlainModerationRecordsKeepNativeDecoding(t *testing.T) {
	// Requiredness belongs to item admission, not public nested records or an
	// application's anonymous envelope. Unknown actions are preserved verbatim.
	var envelope struct {
		p.AgentMessageModeration
		Extra string `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"extra":"kept","display":{"action":"future"}}`), &envelope); err != nil || envelope.Extra != "kept" || envelope.Display.Action != "future" {
		t.Fatalf("envelope=%+v err=%v", envelope, err)
	}
}

func TestOpaqueInferenceHookError(t *testing.T) {
	var e p.TurnError
	if err := json.Unmarshal([]byte(`{"message":"denied","codexErrorInfo":"inferenceHookDenied"}`), &e); err != nil || string(e.CodexErrorInfo) != `"inferenceHookDenied"` {
		t.Fatalf("error=%+v %v", e, err)
	}
}

func FuzzAgentModerationMerges(f *testing.F) {
	for i := range 9 {
		f.Add([]byte{byte(i)})
	}
	f.Add([]byte{1, 2, 3, 4, 1})
	f.Add([]byte{5, 1})
	f.Add([]byte{1, 6, 7, 8})
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 32 {
			ops = ops[:32]
		}
		fragments := []string{
			`"moderation":null`,
			`"moderation":{"moderated":false}`,
			`"MODERATION":{"display":{"action":"future"}}`,
			`"moderation":{"display":{"blockType":""}}`,
			`"moderation":{}`,
			`"moderation":{"moderated":null}`,
			`"moderation":{"display":null}`,
			`"moderation":{"display":{"action":null}}`,
			`"moderation":{"moderated":true,"display":{"action":"","cyberTrustedAccess":false}}`,
		}
		body := `{"type":"agentMessage","id":"i","text":"t"`
		forbidden := false
		for _, op := range ops {
			n := int(op) % len(fragments)
			body += "," + fragments[n]
			forbidden = forbidden || n == 5 || n == 7
		}
		body += "}"
		// Pointer fields are an independent native-presence oracle for the final
		// merged object. The generator separately records forbidden null events.
		var model struct {
			Moderation *struct {
				Moderated *bool
				Display   *struct{ Action *string }
			}
		}
		if err := json.Unmarshal([]byte(body), &model); err != nil {
			t.Fatal(err)
		}
		valid := !forbidden && (model.Moderation == nil || model.Moderation.Moderated != nil && (model.Moderation.Display == nil || model.Moderation.Display.Action != nil))
		var got p.ThreadItemWrapper
		err := json.Unmarshal([]byte(body), &got)
		if (err == nil) != valid {
			t.Fatalf("admission=%v valid=%v body=%s", err, valid, body)
		}
		if err != nil {
			return
		}
		m := got.Value.(*p.AgentMessageThreadItem).Moderation
		if (m == nil) != (model.Moderation == nil) {
			t.Fatal("presence changed")
		}
		if m != nil && m.Moderated != *model.Moderation.Moderated {
			t.Fatal("boolean changed")
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var round p.ThreadItemWrapper
		if err := json.Unmarshal(data, &round); err != nil {
			t.Fatalf("roundtrip=%s: %v", data, err)
		}
	})
}

func TestPlainGatewayResponseComposition(t *testing.T) {
	var envelope struct {
		p.GatewayOAuthLoginResponse
		Extra string `json:"extra"`
	}
	if err := json.Unmarshal([]byte(`{"extra":"kept","prompt":{"loginId":"l","userCode":"c","verificationUrl":"u"}}`), &envelope); err != nil || envelope.Extra != "kept" || envelope.Prompt == nil {
		t.Fatalf("envelope=%+v err=%v", envelope, err)
	}
}

func gatewayResult(body string) (p.GatewayOAuthLoginResponse, error) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	defer c.Close()
	tr.SetResponse("account/gatewayOAuth/login", p.Response{Result: json.RawMessage(body)})
	return c.Account.GatewayOAuthLogin(context.Background())
}
func securityResult(body string) (p.GetAccountSecuritySetupResponse, error) {
	tr := NewMockTransport()
	c := p.NewClient(tr)
	defer c.Close()
	tr.SetResponse("account/securitySetup/read", p.Response{Result: json.RawMessage(body)})
	return c.Account.SecuritySetupRead(context.Background())
}
