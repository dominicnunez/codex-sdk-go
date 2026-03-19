# Audit Format

This directory centralizes audit findings by referenced file.

## Categories

### Design

Findings that describe behavior which is correct by design.

### Misreads

Findings where the audit misread the code or described behavior that does not occur.

### Risks

Real findings consciously accepted because the architectural cost, external constraints, or implementation effort are disproportionate to the severity.

## Entry Format

Each per-file audit document consists only of finding entries. Entries use this format:

```md
### Plain language description

**Location:** `file/path:line` — optional context

**Reason:** Explanation (can be multiple lines)
```

If a finding references multiple files in its `**Location:**` line, the same
entry is duplicated into each corresponding per-file bucket so auditors can
compare one code file against its matching audit file without cross-referencing
other buckets.
