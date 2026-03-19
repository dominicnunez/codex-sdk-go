# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### Clone fallback comments no longer describe dropped unknown variants

**Location:** `conversation.go:312`, `conversation_internal_test.go:455` — clone fallback semantics

**Reason:** The current comment says the fallback preserves unexpected
in-memory values with a reflective deep clone when the JSON round-trip path does
not work, which matches the implementation and tests. The regression test
`TestCloneFallbacksPreserveUncloneableValues` explicitly verifies that these
fallback helpers preserve uncloneable values instead of dropping them.
