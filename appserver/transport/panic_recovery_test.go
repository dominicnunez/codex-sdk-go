package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestStdioPanicReportingDoubleFault(t *testing.T) {
	for _, mode := range []string{"ordered", "fallback", "request"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStdioPanicReportingHelper$")
			cmd.Env = append(os.Environ(), "CODEX_SDK_PANIC_REPORT_HELPER="+mode)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("double-fault %s worker stopped: %v\n%s", mode, err, output)
			}
		})
	}
}

func TestStdioPanicReportingHelper(t *testing.T) {
	mode := os.Getenv("CODEX_SDK_PANIC_REPORT_HELPER")
	if mode == "" {
		return
	}
	r, w := io.Pipe()
	outR, outW := io.Pipe()
	writer := io.Discard
	if mode == "request" {
		writer = outW
	}
	tr := NewStdioTransport(r, writer)
	t.Cleanup(func() { _ = tr.Close(); _ = w.Close(); _ = outR.Close(); _ = outW.Close() })
	reported := make(chan struct{}, 1)
	tr.OnPanic(func(any) { reported <- struct{}{}; panic("report callback failed") })
	if mode == "request" {
		tr.OnRequest(func(_ context.Context, req Request) (Response, error) {
			if req.Method == "first" {
				panic("request callback failed")
			}
			return Response{Result: json.RawMessage(`{"ok":true}`)}, nil
		})
		enc, dec := json.NewEncoder(w), bufio.NewReader(outR)
		if err := enc.Encode(Request{JSONRPC: "2.0", ID: RequestID{Value: int64(1)}, Method: "first"}); err != nil {
			t.Fatal(err)
		}
		var response Response
		line, readErr := dec.ReadBytes('\n')
		if err := json.Unmarshal(line, &response); err != nil || readErr != nil || response.Error == nil || response.Error.Code != ErrCodeInternalError {
			t.Fatalf("first request response=%+v err=%v", response, err)
		}
		waitAuditSignal(t, reported)
		if err := enc.Encode(Request{JSONRPC: "2.0", ID: RequestID{Value: int64(2)}, Method: "second"}); err != nil {
			t.Fatal(err)
		}
		response = Response{}
		line, readErr = dec.ReadBytes('\n')
		if err := json.Unmarshal(line, &response); err != nil || readErr != nil || string(response.Result) != `{"ok":true}` {
			t.Fatalf("second request response=%+v err=%v", response, err)
		}
		return
	}
	seen := make(chan string, 2)
	tr.OnNotify(func(_ context.Context, n Notification) {
		if strings.Contains(string(n.Params), "first") {
			panic("notification callback failed")
		}
		seen <- n.Method
	})
	method := "audit/unknown"
	if mode == "ordered" {
		method = "item/agentMessage/delta"
	}
	writeAuditNotification(t, w, Notification{Method: method, Params: json.RawMessage(`{"threadId":"a","delta":"first"}`)})
	waitAuditSignal(t, reported)
	writeAuditNotification(t, w, Notification{Method: method, Params: json.RawMessage(`{"threadId":"a","delta":"second"}`)})
	writeAuditNotification(t, w, Notification{Method: "configWarning", Params: json.RawMessage(`{}`)})
	got := map[string]bool{}
	for range 2 {
		select {
		case m := <-seen:
			got[m] = true
		case <-time.After(2 * time.Second):
			t.Fatal("same-owner or unrelated dispatch did not recover")
		}
	}
	if !got[method] || !got["configWarning"] {
		t.Fatalf("post-panic delivery=%v", got)
	}
	waitNotificationBytes(t, tr, 0)
}
