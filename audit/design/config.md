### ReasoningSummaryWrapper accepts any string without validation

**Location:** `config.go:62-69` — ReasoningSummaryWrapper.UnmarshalJSON
**Date:** 2026-02-27

**Reason:** Same forward-compatibility design as SessionSourceWrapper. The server may add new
reasoning summary modes. Rejecting unknown strings would break the SDK on server upgrades.
The type already constrains the value to be a string (rejecting non-string JSON), which is
the meaningful validation boundary.
