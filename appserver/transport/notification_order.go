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
		raw, _, ok := selectJSONObjectField(notif.Params, "thread")
		if !ok {
			return ""
		}
		// Thread.id follows standard field matching, including aliases and
		// sticky type errors, while the containing "thread" is exact.
		var selected json.RawMessage
		invalid := false
		if !walkJSONObjectFields(raw, true, func(key, value []byte) {
			if jsonFieldMatchesFolded(key, "id") {
				selected = value
				if value[0] != '"' && string(value) != "null" {
					invalid = true
				}
			}
		}) || invalid {
			return ""
		}
		var id *string
		if err := json.Unmarshal(selected, &id); err != nil || id == nil {
			return ""
		}
		return "thread:" + *id
	case protocol.NotifyProcessOutputDelta, protocol.NotifyProcessExited:
		kind, field = "process", "processHandle"
	case protocol.NotifyCommandExecOutputDelta:
		kind, field = "command", "processId"
	case protocol.NotifyFuzzyFileSearchSessionUpdated, protocol.NotifyFuzzyFileSearchSessionCompleted:
		kind, field = "search", "sessionId"
	case "externalAgentConfig/import/progress", "externalAgentConfig/import/completed":
		kind, field = "import", "importId"
	case "project/changed":
		kind, field = "project", "projectId"
	case "fs/changed":
		kind, field = "watch", "watchId"
	case "mcpServer/event/stream/notification":
		kind, field = "subscription", "subscriptionId"
	default:
		if strings.HasPrefix(notif.Method, "account/") {
			return "account"
		}
		if !isThreadNotificationMethod(notif.Method) {
			return "method:" + notif.Method
		}
	}
	raw, exists, ok := selectJSONObjectField(notif.Params, field)
	if !ok {
		return ""
	}
	var id *string
	if !exists && hasOptionalThreadOwner(notif.Method) {
		return "method:" + notif.Method
	}
	if err := json.Unmarshal(raw, &id); err != nil {
		return ""
	}
	if id == nil {
		if hasOptionalThreadOwner(notif.Method) {
			return "method:" + notif.Method
		}
		return ""
	}
	// A schema string may be empty. Distinguish it from missing/null instead
	// of allowing a valid owner to bypass ordered delivery.
	return kind + ":" + *id
}

func isThreadNotificationMethod(method string) bool {
	switch method {
	case protocol.NotifyError, protocol.NotifyServerRequestResolved, protocol.NotifyModelRerouted,
		protocol.NotifyHookStarted, protocol.NotifyHookCompleted, protocol.NotifyMcpServerOauthLoginCompleted,
		"mcpServer/startupStatus/updated", "warning", "guardianWarning",
		"autoApprovalReview/strictReviewRequired", "model/verification", "model/safetyBuffering/updated",
		"modelProvider/authRecoveryStarted", "modelProvider/authRecoveryCompleted":
		return true
	default:
		return strings.HasPrefix(method, "thread/") || strings.HasPrefix(method, "turn/") || strings.HasPrefix(method, "item/")
	}
}

func hasOptionalThreadOwner(method string) bool {
	switch method {
	case protocol.NotifyMcpServerOauthLoginCompleted, "mcpServer/startupStatus/updated", "warning":
		return true
	default:
		return false
	}
}
