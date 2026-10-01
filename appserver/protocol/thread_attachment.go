package protocol

import (
	"context"
	"encoding/json"
	"fmt"
)

type ThreadAttachmentAddParams struct {
	AttachmentType string          `json:"attachmentType"`
	IdentityKey    string          `json:"identityKey"`
	Payload        json.RawMessage `json:"payload"`
	ThreadID       string          `json:"threadId"`
}

func (p ThreadAttachmentAddParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("threadId", p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateRequiredNonEmptyStringField("attachmentType", p.AttachmentType); err != nil {
		return nil, err
	}
	if err := validateRequiredNonEmptyStringField("identityKey", p.IdentityKey); err != nil {
		return nil, err
	}
	return p, nil
}

type ThreadAttachment struct {
	AttachmentType string          `json:"attachmentType"`
	CreatedAt      int64           `json:"createdAt"`
	ID             string          `json:"id"`
	IdentityKey    string          `json:"identityKey"`
	Payload        json.RawMessage `json:"payload"`
}

func (a *ThreadAttachment) UnmarshalJSON(data []byte) error {
	type wire ThreadAttachment
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded,
		[]string{"attachmentType", "createdAt", "id", "identityKey", "payload"},
		[]string{"attachmentType", "createdAt", "id", "identityKey"}); err != nil {
		return err
	}
	*a = ThreadAttachment(decoded)
	return nil
}

type ThreadAttachmentAddOutcome string

const (
	ThreadAttachmentAddOutcomeCreated  ThreadAttachmentAddOutcome = "created"
	ThreadAttachmentAddOutcomeExisting ThreadAttachmentAddOutcome = "existing"
)

var validThreadAttachmentAddOutcomes = map[ThreadAttachmentAddOutcome]struct{}{
	ThreadAttachmentAddOutcomeCreated: {}, ThreadAttachmentAddOutcomeExisting: {},
}

func (v ThreadAttachmentAddOutcome) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ThreadAttachmentAddOutcome", v, validThreadAttachmentAddOutcomes)
}
func (v *ThreadAttachmentAddOutcome) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ThreadAttachmentAddOutcome", validThreadAttachmentAddOutcomes, v)
}

type ThreadAttachmentAddResponse struct {
	Attachment ThreadAttachment           `json:"attachment"`
	Outcome    ThreadAttachmentAddOutcome `json:"outcome"`
}

func (r *ThreadAttachmentAddResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadAttachmentAddResponse
	var decoded wire
	if err := unmarshalResponseObject(data, &decoded, []string{"attachment", "outcome"}, []string{"attachment", "outcome"}); err != nil {
		return err
	}
	*r = ThreadAttachmentAddResponse(decoded)
	return nil
}

type ThreadAttachmentListParams struct {
	Cursor   *string `json:"cursor,omitempty"`
	Limit    *uint32 `json:"limit,omitempty"`
	ThreadID string  `json:"threadId"`
}

func (p ThreadAttachmentListParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("threadId", p.ThreadID); err != nil {
		return nil, err
	}
	return p, nil
}

type ThreadAttachmentListResponse struct {
	Data       []ThreadAttachment `json:"data"`
	NextCursor *string            `json:"nextCursor,omitempty"`
}

func (r *ThreadAttachmentListResponse) UnmarshalJSON(data []byte) error {
	type wire ThreadAttachmentListResponse
	var decoded wire
	if err := unmarshalResponseObject(data, &decoded, []string{"data"}, []string{"data"}); err != nil {
		return err
	}
	*r = ThreadAttachmentListResponse(decoded)
	return nil
}

type ThreadAttachmentRemoveParams struct {
	AttachmentType string `json:"attachmentType"`
	IdentityKey    string `json:"identityKey"`
	ThreadID       string `json:"threadId"`
}

func (p ThreadAttachmentRemoveParams) prepareRequest() (interface{}, error) {
	if err := validateRequiredNonEmptyStringField("threadId", p.ThreadID); err != nil {
		return nil, err
	}
	if err := validateRequiredNonEmptyStringField("attachmentType", p.AttachmentType); err != nil {
		return nil, err
	}
	if err := validateRequiredNonEmptyStringField("identityKey", p.IdentityKey); err != nil {
		return nil, err
	}
	return p, nil
}

type ThreadAttachmentRemoveResponse struct{}

type ThreadAttachmentOperation string

const (
	ThreadAttachmentOperationCreated ThreadAttachmentOperation = "created"
	ThreadAttachmentOperationDeleted ThreadAttachmentOperation = "deleted"
)

var validThreadAttachmentOperations = map[ThreadAttachmentOperation]struct{}{
	ThreadAttachmentOperationCreated: {}, ThreadAttachmentOperationDeleted: {},
}

func (v ThreadAttachmentOperation) MarshalJSON() ([]byte, error) {
	return marshalEnumString("ThreadAttachmentOperation", v, validThreadAttachmentOperations)
}
func (v *ThreadAttachmentOperation) UnmarshalJSON(data []byte) error {
	return unmarshalEnumString(data, "ThreadAttachmentOperation", validThreadAttachmentOperations, v)
}

type ThreadAttachmentUpdatedNotification struct {
	AttachmentID   string                    `json:"attachmentId"`
	AttachmentType string                    `json:"attachmentType"`
	IdentityKey    string                    `json:"identityKey"`
	Operation      ThreadAttachmentOperation `json:"operation"`
	ThreadID       string                    `json:"threadId"`
}

func (n *ThreadAttachmentUpdatedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadAttachmentUpdatedNotification
	var decoded wire
	required := []string{"attachmentId", "attachmentType", "identityKey", "operation", "threadId"}
	if err := unmarshalInboundObject(data, &decoded, required, required); err != nil {
		return err
	}
	*n = ThreadAttachmentUpdatedNotification(decoded)
	return nil
}

func (s *ThreadService) AttachmentAdd(ctx context.Context, params ThreadAttachmentAddParams) (ThreadAttachmentAddResponse, error) {
	var response ThreadAttachmentAddResponse
	if err := s.client.sendRequest(ctx, methodThreadAttachmentAdd, params, &response); err != nil {
		return ThreadAttachmentAddResponse{}, err
	}
	return response, nil
}
func (s *ThreadService) AttachmentList(ctx context.Context, params ThreadAttachmentListParams) (ThreadAttachmentListResponse, error) {
	var response ThreadAttachmentListResponse
	if err := s.client.sendRequest(ctx, methodThreadAttachmentList, params, &response); err != nil {
		return ThreadAttachmentListResponse{}, err
	}
	return response, nil
}
func (s *ThreadService) AttachmentRemove(ctx context.Context, params ThreadAttachmentRemoveParams) (ThreadAttachmentRemoveResponse, error) {
	if err := s.client.sendEmptyObjectRequest(ctx, methodThreadAttachmentRemove, params); err != nil {
		return ThreadAttachmentRemoveResponse{}, err
	}
	return ThreadAttachmentRemoveResponse{}, nil
}

func (c *Client) OnThreadAttachmentUpdated(handler func(ThreadAttachmentUpdatedNotification)) {
	if handler == nil {
		c.OnNotification(notifyThreadAttachmentUpdated, nil)
		return
	}
	c.OnNotification(notifyThreadAttachmentUpdated, func(_ context.Context, notif Notification) {
		var value ThreadAttachmentUpdatedNotification
		if err := json.Unmarshal(notif.Params, &value); err != nil {
			c.reportHandlerError(notifyThreadAttachmentUpdated, fmt.Errorf("unmarshal %s: %w", notifyThreadAttachmentUpdated, err))
			return
		}
		handler(value)
	})
}
