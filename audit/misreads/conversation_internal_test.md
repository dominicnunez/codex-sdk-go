### Clone fallback comments no longer describe dropped unknown variants

**Location:** `conversation.go:312`, `conversation_internal_test.go:455` — clone fallback semantics

**Reason:** The current comment says the fallback preserves unexpected
in-memory values with a reflective deep clone when the JSON round-trip path does
not work, which matches the implementation and tests. The regression test
`TestCloneFallbacksPreserveUncloneableValues` explicitly verifies that these
fallback helpers preserve uncloneable values instead of dropping them.
