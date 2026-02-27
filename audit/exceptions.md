# Audit Exceptions

> Items validated as false positives or accepted as won't-fix.
> Managed by willie audit loop. Do not edit format manually.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

## False Positives

<!-- Findings where the audit misread the code or described behavior that doesn't occur -->

## Won't Fix

<!-- Real findings not worth fixing — architectural cost, external constraints, etc. -->

### StdioTransport.Close does not stop the reader goroutine

**Location:** `stdio.go:140-157` — Close() and readLoop()
**Date:** 2026-02-26

**Reason:** Fixing this requires changing the public API from `io.Reader` to `io.ReadCloser`,
which is a breaking change for all callers. The primary use case is `os.Stdin`, where the reader
goroutine terminates naturally with the process. In library contexts, callers control the underlying
reader and can close it themselves to unblock the scanner. The goroutine leak only matters for
long-running processes that create and discard many StdioTransport instances — a usage pattern
this SDK doesn't target.

### Write goroutine in Send can leak on context cancellation

**Location:** `stdio.go:86-102` — Send's write goroutine
**Date:** 2026-02-27

**Reason:** The goroutine runs `writeMessage`, which acquires `writeMu` and calls
`io.Writer.Write`. Go's `io.Writer` interface has no context or deadline support, so
there's no way to interrupt a blocked write without closing the underlying writer.
Fixing this requires either changing the public API to accept `io.WriteCloser` (breaking),
or wrapping all writes in a deadline-aware layer that introduces complexity disproportionate
to the risk. In practice, the writer is stdout to a child process — writes block only if the
process is hung, at which point the entire SDK session is stuck regardless.

### Notification handlers silently swallow unmarshal errors

**Location:** `account_notifications.go`, `turn_notifications.go`, and 25 other handlers
**Date:** 2026-02-27

**Reason:** Adding error surfacing requires either an `OnNotificationError` callback on Client
(new public API surface + all 27 handlers need plumbing) or changing handler signatures to return
errors (breaking change). The silent-drop behavior is consistent with JSON-RPC 2.0 notification
semantics where the server doesn't expect acknowledgment. Malformed notifications from the server
indicate a protocol-level bug that would manifest in other ways. The risk of silent data loss is
low relative to the API churn required to surface these errors.

## Intentional Design Decisions

<!-- Findings that describe behavior which is correct by design -->

### Notification handler registration silently overwrites previous handlers

**Location:** `client.go:138-142` — OnNotification
**Date:** 2026-02-27

**Reason:** One handler per method is the intentional dispatch model. The Client routes each
notification method to exactly one handler — the same pattern used by `http.HandleFunc` and
similar Go standard library APIs. Supporting multiple handlers adds complexity (slice management,
ordering semantics, error aggregation) without clear benefit. The `OnNotification` doc comment
states "Only one handler can be registered per method; subsequent calls replace the previous
handler." This is documented behavior, not a bug.
