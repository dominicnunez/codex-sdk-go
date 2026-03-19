# Misreads

> Findings where the audit misread the code or described behavior that doesn't occur.
> Managed by sfk willie. Follow the entry format below.
>
> Entry format:
> ### Plain language description
> **Location:** `file/path:line` — optional context
> **Date:** YYYY-MM-DD
> **Reason:** Explanation (can be multiple lines)

### ChatgptAuthTokensRefreshParams described as carrying auth tokens that need redaction tests

**Location:** `credential_redact_test.go` — missing test claim
**Date:** 2026-03-01

**Reason:** The audit claims `ChatgptAuthTokensRefreshParams` is "the request type that carries auth
tokens" and needs redaction tests. This is factually wrong. `ChatgptAuthTokensRefreshParams` contains
only `Reason` (a string enum) and `PreviousAccountID` (optional string) — neither is a credential.
The type carries the *reason* for a token refresh request (e.g. "expired"), not the actual tokens.
The *response* type (`ChatgptAuthTokensRefreshResponse`) carries the new `AccessToken` and already
has `MarshalJSON` redaction with full test coverage in `credential_redact_test.go`. There is nothing
to redact on the params type.
