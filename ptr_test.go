package codex_test

import (
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestPtr(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		p := codex.Ptr("hello")
		if *p != "hello" {
			t.Errorf("Ptr(\"hello\") = %q, want \"hello\"", *p)
		}
	})

	t.Run("int", func(t *testing.T) {
		p := codex.Ptr(42)
		if *p != 42 {
			t.Errorf("Ptr(42) = %d, want 42", *p)
		}
	})

	t.Run("bool", func(t *testing.T) {
		p := codex.Ptr(false)
		if *p != false {
			t.Errorf("Ptr(false) = %v, want false", *p)
		}
	})

	t.Run("zero values", func(t *testing.T) {
		if *codex.Ptr(0) != 0 {
			t.Error("Ptr(0) != 0")
		}
		if *codex.Ptr("") != "" {
			t.Error(`Ptr("") != ""`)
		}
	})

	t.Run("returns distinct pointers", func(t *testing.T) {
		a := codex.Ptr("same")
		b := codex.Ptr("same")
		if a == b {
			t.Error("Ptr returned same pointer for different calls")
		}
	})
}
