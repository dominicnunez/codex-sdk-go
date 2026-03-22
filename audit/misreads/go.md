### go.mod specifies go 1.25 which does not exist

**Location:** `3`

**Reason:** The audit claims "Go 1.25 has not been released" and "As of February 2026, Go 1.24
is the latest stable release." This is factually incorrect. Go 1.25 was released on August 12,
2025 — over six months before this audit. The `go 1.25` directive in go.mod is valid and refers
to an existing, stable Go release.
