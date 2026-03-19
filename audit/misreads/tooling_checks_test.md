### CI already enforces module tidiness

**Location:** `.github/workflows/ci.yml:43`, `tooling_checks_test.go:45` — workflow gate and tooling test

**Reason:** The current GitHub Actions workflow already runs `go mod tidy -diff`
in the main CI job, and the tooling test still verifies the same rule under the
optional tooling lane. The finding no longer matches the checked-in workflow.
