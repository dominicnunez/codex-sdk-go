package protocol_test

import (
	"go/version"
	"runtime"
	"strings"
	"testing"
)

// TestGoVersion verifies the runtime Go version meets the module's minimum Go directive.
func TestGoVersion(t *testing.T) {
	minimum := moduleGoVersion(t)
	actual := runtime.Version()
	if version.Compare(strings.TrimPrefix(actual, "devel "), "go"+minimum) < 0 {
		t.Fatalf("Go %s required, running %s", minimum, actual)
	}

	t.Logf("Go version: %s", actual)
}
