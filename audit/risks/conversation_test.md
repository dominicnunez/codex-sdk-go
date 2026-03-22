### Conversation and turn tests use time.Sleep for goroutine synchronization

**Location:** `42,67,111,256,315,525,554,623,664,715`

**Reason:** Nearly every turn-based test uses `time.Sleep(50ms)` between starting a goroutine
that calls `Turn()` and injecting the completion notification. Replacing these with deterministic
signals requires adding a method-call signaling mechanism to MockTransport (e.g. a channel that
fires when `turn/start` is sent). The mock transport currently returns immediately from `Send`,
so the 50ms sleep is reliably sufficient. The fix requires non-trivial test infrastructure
changes across ~15 tests for a low-severity code smell. The tests have never flaked in CI.

### Conversation and turn tests use time.Sleep for goroutine synchronization

**Location:** `42,67,111,256,315,525,554,623,664,715`

**Reason:** Nearly every turn-based test uses `time.Sleep(50ms)` between starting a goroutine
that calls `Turn()` and injecting the completion notification. Replacing these with deterministic
signals requires adding a method-call signaling mechanism to MockTransport (e.g. a channel that
fires when `turn/start` is sent). The mock transport currently returns immediately from `Send`,
so the 50ms sleep is reliably sufficient. The fix requires non-trivial test infrastructure
changes across ~15 tests for a low-severity code smell. The tests have never flaked in CI.

### Conversation and turn tests use time.Sleep for goroutine synchronization

**Location:** `42,67,111,256,315,525,554,623,664,715`

**Reason:** Nearly every turn-based test uses `time.Sleep(50ms)` between starting a goroutine
that calls `Turn()` and injecting the completion notification. Replacing these with deterministic
signals requires adding a method-call signaling mechanism to MockTransport (e.g. a channel that
fires when `turn/start` is sent). The mock transport currently returns immediately from `Send`,
so the 50ms sleep is reliably sufficient. The fix requires non-trivial test infrastructure
changes across ~15 tests for a low-severity code smell. The tests have never flaked in CI.
