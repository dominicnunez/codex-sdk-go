### TestErrorCodeConstants verifies constants against their literal definitions

**Location:** `230-250`

**Reason:** The test compares `ErrCodeParseError` against `-32700`, etc. These values are defined
as constants, so the test is tautological — it can only fail if someone changes the constant but not
the test. This is intentional documentation-as-test: the test serves as executable documentation that
the constants match the JSON-RPC 2.0 spec values. The alternative (deleting the test) loses the
documentation value with no practical benefit.
