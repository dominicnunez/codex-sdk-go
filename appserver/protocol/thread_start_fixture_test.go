package protocol

import (
	"context"
	"encoding/json"
	"fmt"
)

type startStateTransport struct {
	handler  NotificationHandler
	before   func(context.Context, Request)
	result   json.RawMessage
	failure  error
	provider *Error
}

func startStateThread(id, name string) string {
	return fmt.Sprintf(`{"id":%q,"name":%q,"projectId":null,"sessionId":"session","cliVersion":"test","createdAt":1,"cwd":"/tmp","modelProvider":"openai","preview":"initial","source":"exec","status":{"type":"idle"},"turns":[],"updatedAt":1,"ephemeral":true}`, id, name)
}

func startStateResponse(id string) json.RawMessage {
	return json.RawMessage(`{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"openai","sandbox":{"type":"readOnly"},"disabledPluginIds":["initial"],"thread":` + startStateThread(id, "initial") + `}`)
}

func (tr *startStateTransport) Send(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if tr.before != nil {
		tr.before(ctx, req)
	}
	return Response{ID: req.ID, Result: tr.result, Error: tr.provider}, tr.failure
}

func (*startStateTransport) Notify(context.Context, Notification) error { return nil }
func (*startStateTransport) OnRequest(RequestHandler)                   {}
func (tr *startStateTransport) OnNotify(h NotificationHandler)          { tr.handler = h }
func (*startStateTransport) Close() error                               { return nil }

func (tr *startStateTransport) started(id, name string) {
	tr.handler(context.Background(), Notification{Method: notifyThreadStarted, Params: json.RawMessage(`{"thread":` + startStateThread(id, name) + `}`)})
}

func (tr *startStateTransport) closed(id string) {
	tr.handler(context.Background(), Notification{Method: notifyThreadClosed, Params: json.RawMessage(fmt.Sprintf(`{"threadId":%q}`, id))})
}

func (tr *startStateTransport) name(id, name string) {
	tr.handler(context.Background(), Notification{Method: notifyThreadNameUpdated, Params: json.RawMessage(fmt.Sprintf(`{"threadId":%q,"threadName":%q}`, id, name))})
}

func (tr *startStateTransport) settings(id string, plugins []string) {
	data, _ := json.Marshal(plugins)
	tr.handler(context.Background(), Notification{Method: notifyThreadSettingsUpdated, Params: json.RawMessage(fmt.Sprintf(`{"threadId":%q,"threadSettings":{"approvalPolicy":"never","approvalsReviewer":"user","cwd":"/tmp","model":"model","modelProvider":"openai","sandboxPolicy":{"type":"readOnly"},"collaborationMode":{"mode":"default","settings":{"model":"model"}},"disabledPluginIds":%s}}`, id, data))})
}
