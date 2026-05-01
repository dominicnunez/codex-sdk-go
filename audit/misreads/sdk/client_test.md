### Unknown notification test relies on fixed sleep for a negative assertion

**Line:** `118` — `TestClientUnknownNotification`

**Reason:** `MockTransport.InjectServerNotification` calls the registered notification handler synchronously, and `Client.handleNotification` invokes matching listeners synchronously before returning. The sleep is unnecessary, but the current test does not depend on scheduler timing to flush notification dispatch.

There is also a deterministic same-behavior test in `sdk/dispatch_test.go`, so the described asynchronous-dispatch gap does not occur in the current code path.
