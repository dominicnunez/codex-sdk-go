# Design

> Findings that describe behavior which is correct by design.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### BinaryPath relies on design-by-contract without path validation

**Location:** `process.go:149-153` — BinaryPath resolution in StartProcess
**Date:** 2026-03-01

**Reason:** `ProcessOptions.BinaryPath` is documented as "must be a trusted value — passed directly
to exec.CommandContext." The SDK is a library where callers control all inputs. Adding path
sanitization (no null bytes, no shell metacharacters) would be speculative defense against caller
misuse that exec.CommandContext already rejects with clear errors. The design-by-contract boundary
is at the public API: callers who set BinaryPath from untrusted input have a bug in their code,
not in the SDK. This matches Go standard library conventions where exec.Command accepts any string
and fails at execution time.
