package diagnostic

import (
	"fmt"
	"strings"
	"testing"
)

func TestDisplayPreservesShortValues(t *testing.T) {
	for _, value := range []string{"", "short", "\x00\n日本語", "\xff", strings.Repeat("x", 256)} {
		if got := Display(value); got != fmt.Sprint(value) {
			t.Fatal("short display changed")
		}
	}
	for _, value := range []string{strings.Repeat("x", 257), strings.Repeat("\x00", 1<<20), strings.Repeat("\xff", 1<<20)} {
		if got := Display(value); len(got) > 1600 || !strings.Contains(got, "bytes omitted") {
			t.Fatal("large display is not bounded")
		}
	}
}
