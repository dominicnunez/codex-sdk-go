# Risks

> Real findings consciously accepted — architectural cost, external constraints, disproportionate effort.

### Unix process-group shutdown cannot safely re-identify a recycled group leader PID

**Location:** `process_tree_unix.go:29` — process-group interrupt/kill path

**Reason:** Portable Unix APIs do not expose a stable handle for a process group after the original
leader exits. The current implementation can only address the group by PGID, and a complete fix
would require platform-specific primitives such as Linux pidfds or a different supervisor model
that tracks descendants directly. Simply skipping the final group kill after the leader exits would
avoid the reuse race but would also leak surviving child processes, which is a worse shutdown
regression for this SDK.
