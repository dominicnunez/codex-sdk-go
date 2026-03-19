# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Handler errors in handleApproval are invisible to SDK consumers

**Location:** `client.go:273-274` — handleApproval error return path
**Date:** 2026-02-27

**Reason:** When a user-supplied approval handler returns an error, it propagates to `handleRequest`
in `stdio.go` which replaces it with a generic `"internal handler error"` response on the wire.
The original error is never surfaced to the SDK consumer. Adding observability (e.g. an
`OnHandlerError` callback on `Client`) requires new public API surface. This is the same pattern
as the existing "notification handlers silently swallow unmarshal errors" and "writeMessage errors
silently discarded" exceptions — surfacing internal errors from goroutine-dispatched handlers
requires API additions disproportionate to the severity. Consumers who need observability can
wrap their handler functions with their own error logging before passing them to the SDK.

### handleApproval includes server-controlled method name in internal error strings

**Location:** `client.go:274,279,284` — error wrapping in handleApproval
**Date:** 2026-02-27

**Reason:** The `req.Method` string from the server is included in error messages, but these
errors never cross a trust boundary. `handleRequest` in `stdio.go` replaces all handler errors
with a generic `"internal handler error"` before sending the JSON-RPC response. The internal
error strings are not logged, stored, or exposed. This is a defense-in-depth observation with
no active vulnerability. Adding truncation/sanitization to internal error formatting adds
complexity without mitigating any concrete risk.

### Internal listener sequence counter can theoretically wrap around and collide

**Location:** `client.go:217` — internalListenerSeq uint64 increment
**Date:** 2026-02-28

**Reason:** `internalListenerSeq` is incremented without overflow checking. After 2^64
increments it wraps to 0 and subsequent IDs could collide with still-registered listeners.
However, 2^64 operations is unreachable in any realistic runtime — at 1 billion increments
per second it would take ~584 years. Adding overflow detection or a different ID scheme
is disproportionate to the near-zero probability of occurrence.
