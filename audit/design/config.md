### ReasoningSummaryWrapper accepts any string without validation

**Location:** `62-69`

**Reason:** Same forward-compatibility design as SessionSourceWrapper. The server may add new
reasoning summary modes. Rejecting unknown strings would break the SDK on server upgrades.
The type already constrains the value to be a string (rejecting non-string JSON), which is
the meaningful validation boundary.
