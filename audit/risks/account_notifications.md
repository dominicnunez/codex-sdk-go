# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Notification handlers silently swallow unmarshal errors

**Location:** `account_notifications.go`, `turn_notifications.go`, and 25 other handlers
**Date:** 2026-02-27

**Reason:** Adding error surfacing requires either an `OnNotificationError` callback on Client
(new public API surface + all 27 handlers need plumbing) or changing handler signatures to return
errors (breaking change). The silent-drop behavior is consistent with JSON-RPC 2.0 notification
semantics where the server doesn't expect acknowledgment. Malformed notifications from the server
indicate a protocol-level bug that would manifest in other ways. The risk of silent data loss is
low relative to the API churn required to surface these errors.
