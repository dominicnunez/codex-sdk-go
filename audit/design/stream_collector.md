### Stream summary takes a full snapshot on each call

**Location:** `151`

**Reason:** `Summary()` intentionally returns a deep copy so callers can mutate the returned
value without racing or corrupting collector state. Eliminating full-copy work would require
changing the API contract (for example immutable views or incremental subscriptions), which is
an architectural shift disproportionate to this low-severity finding. The current behavior is
correct by design and prioritized for data isolation and thread safety.
