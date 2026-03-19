### TestErrorCodeConstants described as tautological

**Location:** `jsonrpc_test.go:230-249` — table-driven test comparing constants to literals
**Date:** 2026-03-01

**Reason:** This is a duplicate of the known exception "TestErrorCodeConstants verifies constants
against their literal definitions" which explains the test serves as executable documentation
that the constants match the JSON-RPC 2.0 spec values.
