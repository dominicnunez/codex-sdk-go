### Realtime notification payloads are already validated before handlers run

**Location:** `realtime.go:25` — realtime notification unmarshaling

**Reason:** The current tree already validates `thread/realtime/closed`,
`thread/realtime/error`, and `thread/realtime/itemAdded` with custom
`UnmarshalJSON` methods that call `unmarshalInboundObject`. Malformed payloads
do not reach application handlers. The regression coverage in
`realtime_test.go` also includes missing-required-field handler error tests for
these notifications.

### Comment about silently ignoring unmarshal errors claimed to be copy-pasted across 25+ handlers

**Location:** `streaming.go:100`, `thread_notifications.go:18`, `turn_notifications.go:18`, `account_notifications.go:17`, `realtime.go:18` — notification handlers
**Date:** 2026-02-28

**Reason:** The audit claims the comment `// Silently ignore unmarshal errors (notification is malformed)`
appears in "every notification handler" and is "copy-pasted across 25+ handler methods." This is
factually wrong. The comment exists in exactly one location: `streaming.go:100`. The other file:line
references (`thread_notifications.go:18`, `turn_notifications.go:18`, `account_notifications.go:17`,
`realtime.go:18`) point to struct type definitions, not comments or handler methods. Most notification
handlers silently return on unmarshal error without any comment at all — but the finding's claim that
a specific comment is duplicated across 25+ methods does not match the code.
