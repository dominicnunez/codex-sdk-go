### The client test suite no longer locks in forged transport metadata as a TransportError

**Location:** `257`

**Reason:** The checked-in test now verifies the opposite behavior. The test at
`client_test.go:257-291` is `TestClientSendForgedTransportFailureResponseReturnsRPCError`,
and it asserts that a forged wire payload remains an `RPCError` with the original
error code, message, and data intact.
