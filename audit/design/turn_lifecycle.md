### executeStreamedTurn emits TurnCompleted before the terminal error event

**Location:** `206-211`

**Reason:** When a turn completes with an error, `executeStreamedTurn` emits `TurnCompleted{Turn: completed.Turn}`
followed by a stream error. The `TurnCompleted` event carries the full `Turn` struct including `Turn.Error`,
so consumers can inspect the error on receipt. This is intentional: stream consumers see the complete turn
data first, then the terminal error that closes the stream. Skipping `TurnCompleted` on error would deprive
consumers of the turn data (items, metadata) that may be needed for error reporting or partial results.
The non-streaming `executeTurn` returns only an error because its caller already has the turn data from
the response — different API shape, same information available.

### Duplicate turn/completed notifications silently dropped via default branch

**Location:** `64-67`

**Reason:** The `done`/`turnDone` channel has capacity 1. If a duplicate `turn/completed` notification
arrives, the `default` branch drops it silently. This is correct defensive behavior: the channel signals
"at least one completion" and consuming code proceeds on the first signal. Reporting the duplicate via
`reportHandlerError` would add observability for a server bug, but the SDK's notification handlers are
not the right place to diagnose server-side protocol violations — that belongs in server-side telemetry.
The drop is safe because the first notification already contains the authoritative turn data.

### Duplicate turn/completed notifications silently dropped via default branch

**Location:** `197-200`

**Reason:** The `done`/`turnDone` channel has capacity 1. If a duplicate `turn/completed` notification
arrives, the `default` branch drops it silently. This is correct defensive behavior: the channel signals
"at least one completion" and consuming code proceeds on the first signal. Reporting the duplicate via
`reportHandlerError` would add observability for a server bug, but the SDK's notification handlers are
not the right place to diagnose server-side protocol violations — that belongs in server-side telemetry.
The drop is safe because the first notification already contains the authoritative turn data.
