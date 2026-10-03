package protocol

import (
	"context"
	"fmt"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

type ThreadPredictionResult interface{ isThreadPredictionResult() }
type ThreadPredictionResultWrapper struct{ Value ThreadPredictionResult }
type CompletedThreadPredictionResult struct {
	Text *string `json:"text,omitempty"`
}
type FailedThreadPredictionResult struct{}

func (*CompletedThreadPredictionResult) isThreadPredictionResult() {}
func (*FailedThreadPredictionResult) isThreadPredictionResult()    {}
func (v CompletedThreadPredictionResult) MarshalJSON() ([]byte, error) {
	type wire CompletedThreadPredictionResult
	return jsonencode.Marshal(struct {
		Type string `json:"type"`
		wire
	}{"completed", wire(v)})
}
func (v FailedThreadPredictionResult) MarshalJSON() ([]byte, error) {
	return []byte(`{"type":"failed"}`), nil
}
func (w *ThreadPredictionResultWrapper) UnmarshalJSON(data []byte) error {
	tag, err := decodeRequiredObjectTypeField(data, "ThreadPredictionResult")
	if err != nil {
		return err
	}
	switch tag {
	case "completed":
		var v CompletedThreadPredictionResult
		if err := jsondecode.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	case "failed":
		var v FailedThreadPredictionResult
		if err := jsondecode.Unmarshal(data, &v); err != nil {
			return err
		}
		w.Value = &v
	default:
		return fmt.Errorf("unknown ThreadPredictionResult type %s", quotedValueDiagnostic(tag))
	}
	return nil
}
func (w ThreadPredictionResultWrapper) MarshalJSON() ([]byte, error) {
	if isNilInterfaceValue(w.Value) {
		return nil, fmt.Errorf("missing ThreadPredictionResult")
	}
	return jsonencode.Marshal(w.Value)
}

type ThreadPredictionUpdatedNotification struct {
	Result       ThreadPredictionResultWrapper `json:"result"`
	SourceTurnID string                        `json:"sourceTurnId"`
	ThreadID     string                        `json:"threadId"`
}

func (n *ThreadPredictionUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadPredictionUpdatedNotification
	var v wire
	if err := unmarshalInboundObject(data, &v, []string{"result", "sourceTurnId", "threadId"}, []string{"result", "sourceTurnId", "threadId"}); err != nil {
		return err
	}
	*n = ThreadPredictionUpdatedNotification(v)
	return nil
}
func (c *Client) OnThreadPredictionUpdated(handler func(ThreadPredictionUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadPredictionUpdated, nil)
		return
	}
	c.OnNotification(notifyThreadPredictionUpdated, func(_ context.Context, notif Notification) {
		var v ThreadPredictionUpdatedNotification
		if err := jsondecode.Unmarshal(notif.Params, &v); err != nil {
			c.reportHandlerError(notifyThreadPredictionUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadPredictionUpdated, err))
			return
		}
		handler(v)
	})
}
