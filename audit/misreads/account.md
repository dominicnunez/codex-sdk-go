### Account login decode failures already include the RPC method name

**Location:** `account.go:614` — `AccountService.Login`

**Reason:** The current implementation already wraps `UnmarshalLoginAccountResponse` failures with
`fmt.Errorf("%s: %w", methodAccountLoginStart, err)`, so malformed login results surface as
`account/login/start: ...` at the service boundary. The stale report line no longer matches the
checked-in code, and the login tests assert the method-prefixed error text.

### Formatting nil login parameter pointers does not panic in fmt paths

**Location:** `account.go:215`, `account.go:285` — `String` methods used by `Format`
**Date:** 2026-03-03

**Reason:** The finding says formatting a nil pointer (for example `%v`) will panic and crash.
That behavior does not occur in Go's `fmt` package for nil pointers that implement formatter/stringer:
`fmt.Sprintf("%v", (*ApiKeyLoginAccountParams)(nil))` and
`fmt.Sprintf("%v", (*ChatgptAuthTokensLoginAccountParams)(nil))` both render `<nil>`.
So the specific reported crash path via formatting is a misread.

### AccountWrapper nil receiver check is reachable via pointer field

**Location:** `account.go:96-97` — AccountWrapper.MarshalJSON pointer receiver
**Date:** 2026-02-27

**Reason:** The audit claims `AccountWrapper` is "used as a value type in struct fields" and
therefore the `a == nil` check on the pointer receiver is unreachable dead code. This is incorrect.
`GetAccountResponse` at `account.go:16` declares the field as `*AccountWrapper` (pointer), not
`AccountWrapper` (value). When this pointer field is nil, `json.Marshal` calls
`(*AccountWrapper)(nil).MarshalJSON()`, making the nil check both reachable and correct. The
pointer receiver is appropriate here precisely because the field is a pointer type.
