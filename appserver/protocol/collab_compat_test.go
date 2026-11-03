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

func TestCollabPriorErrorsAndPlainContracts(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	defer client.Close()
	body := validThreadLifecycleResponse(validProcessThreadPayload("collab-thread"))
	body["model"] = ""
	body["collaborationMode"] = json.RawMessage(`{"settings":{}}`)
	if err := mock.SetResponseData("thread/resume", body); err != nil {
		t.Fatal(err)
	}
	result, err := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
	if err == nil || err.Error() != "thread/resume: missing model" || !reflect.ValueOf(result).IsZero() {
		t.Fatalf("prior lifecycle semantic error/stage changed: %v", err)
	}
	body["model"] = "model"
	body["collaborationMode"] = json.RawMessage(`{"mode":"future","settings":{}}`)
	if err := mock.SetResponseData("thread/resume", body); err != nil {
		t.Fatal(err)
	}
	_, err = client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
	if err == nil || !strings.Contains(err.Error(), `invalid mode "future"`) || !strings.HasPrefix(err.Error(), "unmarshal response result for thread/resume:") {
		t.Fatalf("native mode error/stage displaced: %v", err)
	}
	body["collaborationMode"] = json.RawMessage(`{"settings":{"model":42}}`)
	if err := mock.SetResponseData("thread/resume", body); err != nil {
		t.Fatal(err)
	}
	_, err = client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) || !strings.HasSuffix(mismatch.Field, "settings.model") {
		t.Fatalf("native model kind/context displaced: %v", err)
	}

	for _, value := range []any{p.CollaborationMode{}, &p.CollaborationMode{}, p.CollaborationModeSettings{}, &p.CollaborationModeSettings{}} {
		if _, ok := value.(json.Unmarshaler); ok {
			t.Fatal("plain collaboration record acquired decoder")
		}
		if _, ok := value.(json.Marshaler); ok {
			t.Fatal("plain collaboration record acquired encoder")
		}
	}
	var named struct {
		Mode  p.CollaborationMode `json:"collaborationMode"`
		After string              `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"collaborationMode":{},"after":"visited"}`), &named); err != nil || named.After != "visited" {
		t.Fatal("plain named envelope acquired admission restrictions")
	}
	var embedded struct {
		p.CollaborationMode
		After string `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"mode":"default","settings":{},"after":"visited"}`), &embedded); err != nil || embedded.After != "visited" {
		t.Fatal("plain anonymous envelope acquired admission restrictions")
	}
	var settings struct {
		p.CollaborationModeSettings
		After string `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"model":null,"after":"visited"}`), &settings); err != nil || settings.After != "visited" {
		t.Fatal("plain settings embedding changed")
	}
}

func TestCollabSettingsReceiverAndEarlierError(t *testing.T) {
	prior := p.Ptr("prior")
	value := p.ThreadSettings{Model: "prior", CollaborationMode: p.CollaborationMode{Mode: p.ModeKindDefault, Settings: p.CollaborationModeSettings{Model: "prior", DeveloperInstructions: prior}}}
	before := value
	var envelope struct {
		Settings json.RawMessage `json:"threadSettings"`
	}
	if err := json.Unmarshal(collabSettingsBody(`{"mode":"default","settings":{}}`), &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope.Settings, &value); err == nil || !reflect.DeepEqual(value, before) || value.CollaborationMode.Settings.DeveloperInstructions != prior {
		t.Fatal("new settings rejection changed prior values/references")
	}
	badPath := strings.Replace(string(envelope.Settings), `"cwd":"/tmp"`, `"cwd":"relative"`, 1)
	if err := json.Unmarshal([]byte(badPath), &value); err == nil || !strings.Contains(err.Error(), "threadSettings.cwd") {
		t.Fatalf("old settings path error lost priority: %v", err)
	}
	var outer struct {
		Before   int              `json:"before"`
		Settings p.ThreadSettings `json:"threadSettings"`
		After    string           `json:"after"`
	}
	raw := `{"before":"bad","threadSettings":` + string(envelope.Settings) + `,"after":"visited"}`
	err := json.Unmarshal([]byte(raw), &outer)
	var mismatch *json.UnmarshalTypeError
	if !errors.As(err, &mismatch) || mismatch.Field != "before" || outer.After != "visited" {
		t.Fatalf("earlier outer error/traversal changed: %v", err)
	}
}

func TestCollabConstructedRequestUnchanged(t *testing.T) {
	mock := NewMockTransport()
	client := p.NewClient(mock)
	defer client.Close()
	mock.SetResponse("turn/start", p.Response{Result: json.RawMessage(`{"turn":{"id":"turn","status":"inProgress","items":[]}}`)})
	mode := p.CollaborationMode{Mode: p.ModeKindPlan, Settings: p.CollaborationModeSettings{Model: "", DeveloperInstructions: p.Ptr("instructions"), ReasoningEffort: p.Ptr(p.ReasoningEffortHigh)}}
	if _, err := client.Turn.Start(context.Background(), p.TurnStartParams{ThreadID: "collab-thread", Input: []p.UserInput{}, CollaborationMode: &mode}); err != nil {
		t.Fatal(err)
	}
	request := mock.GetSentRequest(0)
	var payload struct {
		Mode p.CollaborationMode `json:"collaborationMode"`
	}
	if err := json.Unmarshal(request.Params, &payload); err != nil || !reflect.DeepEqual(payload.Mode, mode) {
		t.Fatalf("constructed request changed: %v %+v", err, payload.Mode)
	}
}

func TestCollabAdmissionNativeErrorLocations(t *testing.T) {
	for _, raw := range []string{
		`{"mode":"default","settings":{"model":42}}`,
		`{"mode":"default","settings":{"MODEL":{}}}`,
		`{"mode":"default","settings":{"model":"m","reasoning_effort":42}}`,
	} {
		body := validThreadLifecycleResponse(validProcessThreadPayload("collab-thread"))
		body["collaborationMode"] = json.RawMessage(raw)
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		// The unchanged public response decoder is the previous service's
		// native JSON boundary; compare every structured location component.
		var reference p.ThreadResumeResponse
		wantErr := json.Unmarshal(encoded, &reference)
		mock := NewMockTransport()
		client := p.NewClient(mock)
		mock.SetResponse("thread/resume", p.Response{Result: encoded})
		_, gotErr := client.Thread.Resume(context.Background(), p.ThreadResumeParams{ThreadID: "collab-thread"})
		client.Close()
		var want, got *json.UnmarshalTypeError
		if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) || !reflect.DeepEqual(got, want) {
			t.Fatalf("native error location changed: got=%+v want=%+v errors=%v/%v", got, want, gotErr, wantErr)
		}
	}
}
