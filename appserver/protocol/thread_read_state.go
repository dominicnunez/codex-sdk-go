package protocol

import (
	"errors"

	"github.com/dominicnunez/codex-sdk-go/internal/jsondecode"
	"github.com/dominicnunez/codex-sdk-go/internal/jsonencode"
)

// ThreadReadState records the first unread position and its opaque revision.
type ThreadReadState struct {
	Revision    string                                        `json:"revision"`
	FirstUnread OptionalNullable[ThreadUnreadPositionWrapper] `json:"firstUnread,omitzero"`
}

func (s *ThreadReadState) UnmarshalJSON(data []byte) error {
	type wire ThreadReadState
	var decoded wire
	if err := unmarshalInboundObject(data, &decoded, []string{"revision"}, []string{"revision"}); err != nil {
		return err
	}
	*s = ThreadReadState(decoded)
	return nil
}

// ThreadUnreadPosition is either the start of a thread or a completed turn.
type ThreadUnreadPosition interface{ threadUnreadPosition() }

// ThreadStartThreadUnreadPosition marks the entire thread unread.
type ThreadStartThreadUnreadPosition struct{}

func (ThreadStartThreadUnreadPosition) threadUnreadPosition() {}

// TurnThreadUnreadPosition identifies the earliest result needing attention.
type TurnThreadUnreadPosition struct {
	TurnID string `json:"turnId"`
}

func (TurnThreadUnreadPosition) threadUnreadPosition() {}

type ThreadUnreadPositionWrapper struct{ Value ThreadUnreadPosition }

func (w *ThreadUnreadPositionWrapper) UnmarshalJSON(data []byte) error {
	tag, err := decodeRequiredObjectTypeField(data, "thread unread position")
	if err != nil {
		return err
	}
	switch tag {
	case "threadStart":
		w.Value = ThreadStartThreadUnreadPosition{}
	case "turn":
		var decoded TurnThreadUnreadPosition
		if err := validateRequiredTaggedObjectFields(data, "turnId"); err != nil {
			return err
		}
		if err := jsondecode.Unmarshal(data, &decoded); err != nil {
			return err
		}
		w.Value = decoded
	default:
		return errors.New("unknown thread unread position type")
	}
	return nil
}

func (w ThreadUnreadPositionWrapper) MarshalJSON() ([]byte, error) {
	switch v := w.Value.(type) {
	case ThreadStartThreadUnreadPosition:
		return []byte(`{"type":"threadStart"}`), nil
	case *ThreadStartThreadUnreadPosition:
		if v != nil {
			return []byte(`{"type":"threadStart"}`), nil
		}
	case TurnThreadUnreadPosition:
		return marshalUnreadTurn(v)
	case *TurnThreadUnreadPosition:
		if v != nil {
			return marshalUnreadTurn(*v)
		}
	}
	return nil, errors.New("missing or unsupported thread unread position")
}

func marshalUnreadTurn(v TurnThreadUnreadPosition) ([]byte, error) {
	return jsonencode.Marshal(struct {
		Type   string `json:"type"`
		TurnID string `json:"turnId"`
	}{Type: "turn", TurnID: v.TurnID})
}

// ThreadReadStateChangedNotification reports a durable local thread's read state.
type ThreadReadStateChangedNotification struct {
	ThreadID  string          `json:"threadId"`
	ReadState ThreadReadState `json:"readState"`
}

func (n *ThreadReadStateChangedNotification) UnmarshalJSON(data []byte) error {
	type wire ThreadReadStateChangedNotification
	decoded, err := decodeOwnedNotification[wire](data, []string{"threadId", "readState"})
	if err != nil {
		return err
	}
	*n = ThreadReadStateChangedNotification(decoded)
	return nil
}

// OnThreadReadStateChanged sets the handler for thread/readState/changed.
func (c *Client) OnThreadReadStateChanged(h func(ThreadReadStateChangedNotification)) {
	setTypedNotificationHandler(c, notifyThreadReadStateChanged, h)
}

// AddThreadReadStateChangedListener adds a listener for thread/readState/changed.
func (c *Client) AddThreadReadStateChangedListener(h func(ThreadReadStateChangedNotification)) func() {
	return addTypedNotificationListener(c, notifyThreadReadStateChanged, h)
}
