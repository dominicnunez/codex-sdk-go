### Thread rollback does not violate the source-of-truth schema by allowing zero turns

**Location:** `request_paths.go:557` — `ThreadRollbackParams.prepareRequest()`

**Reason:** This finding relies on treating `numTurns >= 1` as a protocol requirement, but the
repo’s source of truth says otherwise. `specs/v2/ThreadRollbackParams.json` defines `numTurns` with
`minimum: 0.0`, and the project `AGENTS.md` explicitly says the JSON schemas in `specs/` are the
source of truth for the protocol surface. In this codebase, allowing `NumTurns: 0` matches the
checked-in schema rather than violating it, so the audit overstates a bug that is not supported by
the authoritative spec files.
