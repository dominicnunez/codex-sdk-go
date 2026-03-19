# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.

### ExecArgs safety does not depend on duplicate-flag parser precedence

**Location:** `process.go:41` — `ProcessOptions.ExecArgs` validation and `process.go:181` — `buildArgs`

**Reason:** The current implementation rejects the `--` end-of-options marker and all typed safety
flags from `ExecArgs` before the CLI is ever spawned. That means `ExecArgs` cannot supply a second
`--model`, `--sandbox`, `--full-auto`, or `--config` flag for the parser to resolve, so the safety
boundary does not depend on a last-wins CLI contract. The remaining ordering test only verifies the
argv shape emitted after validation, and the related comments were tightened to reflect that design.
