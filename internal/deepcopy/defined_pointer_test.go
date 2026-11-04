package deepcopy

import (
	"encoding/json"
	"testing"
)

type definedStringPointer *string

func TestDefinedPointerCopy(t *testing.T) {
	t.Run("ordinary-pointer", func(t *testing.T) {
		source := "original"
		copied := Value(&source)
		*copied = "changed"
		if source != "original" {
			t.Fatal("ordinary pointer copy aliases input")
		}
	})
	t.Run("defined-pointer", func(t *testing.T) {
		source := "original"
		var input definedStringPointer = &source
		if _, e := json.Marshal(input); e != nil {
			t.Fatal(e)
		}
		copied := Value(input)
		*copied = "changed"
		if source != "original" {
			t.Fatal("defined pointer copy aliases original input")
		}
	})
	t.Run("nil-defined-pointer", func(t *testing.T) {
		var source definedStringPointer
		if copied := Value(source); copied != nil {
			t.Fatal("nil pointer became nonnil")
		}
	})
}
