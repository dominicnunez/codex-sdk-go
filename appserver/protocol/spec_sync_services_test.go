package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestSyncFeedbackSaveService(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("feedback/save", p.Response{Result: json.RawMessage(`{"savedPath":"relative/archive.zip","threadId":"t","promptHash":"hash"}`)})
	got, err := client.Feedback.Save(context.Background(), p.FeedbackSaveParams{Classification: "", SavePath: "opaque/archive.zip", IncludeLogs: ptr(false), ExtraLogFiles: &[]string{}})
	if err != nil || got.SavedPath != "relative/archive.zip" || got.ThreadID != "t" || got.PromptHash == nil || *got.PromptHash != "hash" {
		t.Fatalf("saved result = %+v, %v", got, err)
	}
	sent := mock.GetSentRequest(0)
	if sent == nil || sent.Method != "feedback/save" {
		t.Fatal("save did not dispatch")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(sent.Params, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["savePath"]) != `"opaque/archive.zip"` || string(fields["classification"]) != `""` || string(fields["includeLogs"]) != "false" || string(fields["extraLogFiles"]) != "[]" {
		t.Fatalf("params changed: %s", sent.Params)
	}
	for _, raw := range []string{`{}`, `{"savedPath":null,"threadId":"t"}`, `{"savedPath":7,"threadId":"t"}`, `{"savedPath":"p","threadId":"t","SAVEDPATH":null}`, `{"savedPath":"p","threadId":"t","THREADID":null}`, `{"savedPath":"p","threadId":null}`} {
		mock.SetResponse("feedback/save", p.Response{Result: json.RawMessage(raw)})
		got, err := client.Feedback.Save(context.Background(), p.FeedbackSaveParams{})
		if err == nil || !reflect.DeepEqual(got, p.FeedbackSaveResponse{}) {
			t.Fatalf("invalid save result = %+v, %v", got, err)
		}
	}
	mock.SetResponse("feedback/save", p.Response{Result: json.RawMessage(`{"savedPath":"","threadId":""}`)})
	if _, err := client.Feedback.Save(context.Background(), p.FeedbackSaveParams{}); err != nil {
		t.Fatalf("recovery: %v", err)
	}
}

func TestSyncConfigOtelService(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	for _, raw := range []string{`{"requirements":{"otel":{"exporter":"none","log_user_prompt":false,"tool_result":{"max_bytes":0}}}}`, `{"requirements":{"otel":{"exporter":{"otlp-grpc":{"endpoint":"opaque","headers":{}}}}}}`} {
		mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(raw)})
		got, err := client.Config.ReadRequirements(context.Background())
		if err != nil || got.Requirements == nil || got.Requirements.Otel == nil || got.Requirements.Otel.Exporter == nil {
			t.Fatalf("telemetry requirements lost: %+v, %v", got, err)
		}
	}
	for _, raw := range []string{`{"requirements":{"otel":{"exporter":"future"}}}`, `{"requirements":{"otel":{"tool_result":{"max_bytes":0,"MAX_BYTES":null}}}}`, `{"requirements":{"otel":{"exporter":{"otlp-http":{"endpoint":"e","headers":{},"protocol":"future"}}}}}`} {
		mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(raw)})
		got, err := client.Config.ReadRequirements(context.Background())
		if err == nil || !reflect.DeepEqual(got, p.ConfigRequirementsReadResponse{}) {
			t.Fatalf("invalid requirements published: %+v, %v", got, err)
		}
	}
}

func TestSyncAccountAndUsageServices(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	for _, raw := range []string{`{"type":"gcp"}`, `{"type":"chatgpt","email":null,"planType":"plus","workspaceName":"workspace"}`} {
		mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":` + raw + `}`)})
		got, err := client.Account.Get(context.Background(), p.GetAccountParams{})
		if err != nil || got.Account == nil {
			t.Fatalf("account = %+v, %v", got, err)
		}
		out, err := json.Marshal(got.Account)
		if err != nil {
			t.Fatal(err)
		}
		var expected, actual map[string]any
		if err := json.Unmarshal([]byte(raw), &expected); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out, &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("account lost metadata: %s", out)
		}
		if string(out) == `{"type":"gcp"}` {
			if _, ok := got.Account.Value.(*p.GcpAccount); !ok {
				t.Fatal("gcp is not typed")
			}
		}
	}
	mock.SetResponse("account/usage/read", p.Response{Result: json.RawMessage(`{"summary":{},"threadUsage":{"estimatedUsageCreditsMicros":0,"groups":[],"threadId":"t","nativeUsageUsdMicros":0}}`)})
	got, err := client.Account.GetTokenUsage(context.Background(), nil)
	if err != nil || got.ThreadUsage == nil || got.ThreadUsage.NativeUsageUSDMicros == nil || *got.ThreadUsage.NativeUsageUSDMicros != 0 {
		t.Fatalf("native usage lost: %+v, %v", got, err)
	}
}

func TestSyncProcessDrainCallbacks(t *testing.T) {
	for _, add := range []bool{false, true} {
		mock := NewMockTransport()
		var seen []*bool
		errors := 0
		client := p.NewClient(mock, p.WithHandlerErrorCallback(func(_ string, _ error) { errors++ }))
		t.Cleanup(func() { _ = client.Close() })
		handler := func(n p.ProcessExitedNotification) { seen = append(seen, n.OutputDrainTimedOut) }
		if add {
			client.AddProcessExitedListener(handler)
		} else {
			client.OnProcessExited(handler)
		}
		prefix := `{"exitCode":0,"processHandle":"p","stderr":"","stderrCapReached":false,"stdout":"","stdoutCapReached":false`
		for _, field := range []string{"", `,"outputDrainTimedOut":null`, `,"outputDrainTimedOut":false`, `,"outputDrainTimedOut":true`, `,"outputDrainTimedOut":"bad"`} {
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "process/exited", Params: json.RawMessage(prefix + field + `}`)})
		}
		if len(seen) != 4 || errors != 1 || seen[0] != nil || seen[1] != nil || seen[2] == nil || *seen[2] || seen[3] == nil || !*seen[3] {
			t.Fatalf("drain callbacks add=%v: %+v errors=%d", add, seen, errors)
		}
	}
}

func TestSyncOpaqueHookCallbacks(t *testing.T) {
	for _, method := range []string{"hook/started", "hook/completed"} {
		for _, add := range []bool{false, true} {
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			var paths []string
			if method == "hook/started" {
				h := func(n p.HookStartedNotification) { paths = append(paths, n.Run.SourcePath) }
				if add {
					client.AddHookStartedListener(h)
				} else {
					client.OnHookStarted(h)
				}
			} else {
				h := func(n p.HookCompletedNotification) { paths = append(paths, n.Run.SourcePath) }
				if add {
					client.AddHookCompletedListener(h)
				} else {
					client.OnHookCompleted(h)
				}
			}
			for _, path := range []string{"", "relative/hook", "/tmp/a/../hook"} {
				raw, err := json.Marshal(p.HookRunSummary{Entries: []p.HookOutputEntry{}, EventName: "sessionStart", ExecutionMode: "sync", HandlerType: "command", Scope: "thread", SourcePath: path, Status: "completed"})
				if err != nil {
					t.Fatal(err)
				}
				mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","run":` + string(raw) + `}`)})
			}
			if !reflect.DeepEqual(paths, []string{"", "relative/hook", "/tmp/a/../hook"}) {
				t.Fatalf("hook paths lost method=%s add=%v: %+v", method, add, paths)
			}
		}
	}
}

func TestSyncLifecyclePublicRoutes(t *testing.T) {
	item := `{"type":"plan","id":"i","text":"plan","startedAtMs":0,"completedAtMs":-1}`
	check := func(t *testing.T, item p.ThreadItemWrapper) {
		t.Helper()
		v, ok := item.Value.(*p.PlanThreadItem)
		if !ok || v.StartedAtMs == nil || *v.StartedAtMs != 0 || v.CompletedAtMs == nil || *v.CompletedAtMs != -1 {
			t.Fatalf("lifecycle lost: %+v", item.Value)
		}
	}
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	thread := threadWithCitationItem(t, "t", item)
	mock.SetResponse("thread/read", p.Response{Result: json.RawMessage(`{"thread":` + string(thread) + `}`)})
	got, err := client.Thread.Read(context.Background(), p.ThreadReadParams{ThreadID: "t"})
	if err != nil || len(got.Thread.Turns) != 1 {
		t.Fatalf("thread read: %+v %v", got, err)
	}
	check(t, got.Thread.Turns[0].Items[0])
	mock.SetResponse("thread/items/list", p.Response{Result: json.RawMessage(`{"data":[{"turnId":"u","item":` + item + `}]}`)})
	history, err := client.Thread.ItemsList(context.Background(), p.ThreadItemsListParams{ThreadID: "t"})
	if err != nil || len(history.Data) != 1 {
		t.Fatalf("history: %+v %v", history, err)
	}
	check(t, history.Data[0].Item)
	// Per-item lifecycle fields remain separate from the notification envelope's time.
	for _, completed := range []bool{false, true} {
		seen := []p.ThreadItemWrapper{}
		handler := func(v p.ThreadItemWrapper) { seen = append(seen, v) }
		method := "item/started"
		if completed {
			method = "item/completed"
			client.OnItemCompleted(func(n p.ItemCompletedNotification) { handler(n.Item) })
			client.AddItemCompletedListener(func(n p.ItemCompletedNotification) { handler(n.Item) })
		} else {
			client.OnItemStarted(func(n p.ItemStartedNotification) { handler(n.Item) })
			client.AddItemStartedListener(func(n p.ItemStartedNotification) { handler(n.Item) })
		}
		mock.InjectServerNotification(context.Background(), p.Notification{Method: method, Params: json.RawMessage(`{"threadId":"t","turnId":"u","startedAtMs":99,"completedAtMs":100,"item":` + item + `}`)})
		if len(seen) != 2 {
			t.Fatalf("notification %s delivered %d", method, len(seen))
		}
		for _, v := range seen {
			check(t, v)
		}
	}
}

func TestSyncThreadHookService(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	hook := issue74HookPayload(`,"handlerType":"command","command":"hook","async":false,"environmentId":"env"`)
	mock.SetResponse("hooks/list", p.Response{Result: json.RawMessage(`{"data":[{"cwd":"opaque","errors":[],"warnings":[],"hooks":[` + hook + `]}]}`)})
	got, err := client.Hooks.List(context.Background(), p.HooksListParams{ThreadID: ptr("t")})
	if err != nil || len(got.Data) != 1 || len(got.Data[0].Hooks) != 1 {
		t.Fatalf("thread hooks: %+v, %v", got, err)
	}
	env := got.Data[0].Hooks[0].EnvironmentID
	if env == nil || *env != "env" {
		t.Fatalf("environment lost: %+v", got)
	}
	request := mock.GetSentRequest(0)
	if request == nil || string(request.Params) != `{"threadId":"t"}` {
		t.Fatalf("thread hook params: %+v", request)
	}
}

func TestSyncDetachedReviewNeverDispatches(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	_, err := client.Review.Start(context.Background(), p.ReviewStartParams{ThreadID: "t", Target: p.ReviewTargetWrapper{Value: &p.UncommittedChangesReviewTarget{}}, Delivery: ptr(p.ReviewDeliveryDetached)})
	if err == nil || len(mock.SentRequests) != 0 {
		t.Fatalf("obsolete delivery dispatched: %v, %+v", err, mock.SentRequests)
	}
}

func TestSyncGcpNullAliasHasNoPublicResult(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	t.Cleanup(func() { _ = client.Close() })
	mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":true,"account":{"type":"gcp","TYPE":null}}`)})
	got, err := client.Account.Get(context.Background(), p.GetAccountParams{})
	if err == nil || !reflect.DeepEqual(got, p.GetAccountResponse{}) {
		t.Fatalf("invalid GCP account published: %+v, %v", got, err)
	}
	mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":false,"account":{"type":"gcp"}}`)})
	got, err = client.Account.Get(context.Background(), p.GetAccountParams{})
	if err != nil || got.Account == nil {
		t.Fatalf("recovery: %+v, %v", got, err)
	}
	if _, ok := got.Account.Value.(*p.GcpAccount); !ok {
		t.Fatalf("wrong variant: %T", got.Account.Value)
	}
}

func TestSyncGcpRejectsHiddenDiscriminators(t *testing.T) {
	for _, raw := range []string{`{"type":"bogus","TYPE":"gcp"}`, `{"type":"gcp","type":"bogus","TYPE":"gcp"}`, `{"type":"bogus","ty\u0070e":"gcp"}`} {
		direct := p.GcpAccount{Type: "old"}
		if err := json.Unmarshal([]byte(raw), &direct); err == nil || direct.Type != "old" {
			t.Errorf("invalid discriminator admitted directly: %s", raw)
		}
		prior := &p.ApiKeyAccount{}
		wrapper := p.AccountWrapper{Value: prior}
		if err := json.Unmarshal([]byte(raw), &wrapper); err == nil || wrapper.Value != prior {
			t.Errorf("invalid discriminator replaced wrapper: %s", raw)
		}
		mock := NewMockTransport()
		client := p.NewClient(mock)
		t.Cleanup(func() { _ = client.Close() })
		mock.SetResponse("account/read", p.Response{Result: json.RawMessage(`{"requiresOpenaiAuth":true,"account":` + raw + `}`)})
		got, err := client.Account.Get(context.Background(), p.GetAccountParams{})
		if err == nil || !reflect.DeepEqual(got, p.GetAccountResponse{}) {
			t.Errorf("invalid discriminator published by service: %+v %v", got, err)
		}
	}
}
