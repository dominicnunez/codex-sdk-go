package transport

import (
	"encoding/json"
	"strings"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

// Delivery priority must not let a later event overtake an earlier callback
// for the same state owner. Only classified methods participate; unknown
// methods retain the best-effort policy.
func orderedNotificationKey(notif Notification) string {
	if notif.Method != protocol.NotifyItemCompleted && notif.Method != protocol.NotifyTurnCompleted &&
		!isProtectedNotificationMethod(notif.Method) && !isStreamingNotificationMethod(notif.Method) && !isCriticalNotificationMethod(notif.Method) {
		return ""
	}
	// Read the identity owned by the method's schema. Unknown extra fields
	// must not redirect an event into another owner's queue.
	kind := "thread"
	field := "threadId"
	switch notif.Method {
	case protocol.NotifyThreadStarted:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(notif.Params, &fields); err != nil {
			return ""
		}
		// Thread's own decoder uses the standard field matching rules for id.
		// The containing notification requires an exact "thread" property.
		var carrier struct {
			ID *string `json:"id"`
		}
		if err := json.Unmarshal(fields["thread"], &carrier); err != nil || carrier.ID == nil || *carrier.ID == "" {
			return ""
		}
		return "thread:" + *carrier.ID
	case protocol.NotifyProcessOutputDelta, protocol.NotifyProcessExited:
		kind, field = "process", "processHandle"
	case protocol.NotifyCommandExecOutputDelta:
		kind, field = "command", "processId"
	case protocol.NotifyFuzzyFileSearchSessionUpdated, protocol.NotifyFuzzyFileSearchSessionCompleted:
		kind, field = "search", "sessionId"
	default:
		if strings.HasPrefix(notif.Method, "account/") {
			return "account"
		}
		if !isThreadNotificationMethod(notif.Method) {
			return "method:" + notif.Method
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(notif.Params, &fields); err != nil {
		return ""
	}
	var id string
	if err := json.Unmarshal(fields[field], &id); err != nil || id == "" {
		return ""
	}
	return kind + ":" + id
}

func isThreadNotificationMethod(method string) bool {
	switch method {
	case protocol.NotifyError, protocol.NotifyServerRequestResolved, protocol.NotifyModelRerouted,
		protocol.NotifyHookStarted, protocol.NotifyHookCompleted:
		return true
	default:
		return strings.HasPrefix(method, "thread/") || strings.HasPrefix(method, "turn/") || strings.HasPrefix(method, "item/")
	}
}
