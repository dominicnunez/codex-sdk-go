# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### go.mod specifies go 1.25 which does not exist

**Location:** `go.mod:3` — go directive version
**Date:** 2026-02-27

**Reason:** The audit claims "Go 1.25 has not been released" and "As of February 2026, Go 1.24
is the latest stable release." This is factually incorrect. Go 1.25 was released on August 12,
2025 — over six months before this audit. The `go 1.25` directive in go.mod is valid and refers
to an existing, stable Go release.
