package deepcopy

import (
	"encoding/json"
	"testing"
)

func TestDefinedPointerCompositions(t *testing.T) {
	for _, kind := range []string{"record", "slice", "array", "map"} {
		t.Run(kind, func(t *testing.T) {
			original := "original"
			input := definedStringPointer(&original)
			var got definedStringPointer
			switch kind {
			case "record":
				type record struct{ Value definedStringPointer }
				source := record{input}
				if _, e := json.Marshal(source); e != nil {
					t.Fatal(e)
				}
				got = Value(source).Value
			case "slice":
				source := []definedStringPointer{input}
				if _, e := json.Marshal(source); e != nil {
					t.Fatal(e)
				}
				got = Value(source)[0]
			case "array":
				source := [1]definedStringPointer{input}
				if _, e := json.Marshal(source); e != nil {
					t.Fatal(e)
				}
				got = Value(source)[0]
			case "map":
				source := map[string]definedStringPointer{"value": input}
				if _, e := json.Marshal(source); e != nil {
					t.Fatal(e)
				}
				got = Value(source)["value"]
			}
			if got == nil || *got != "original" {
				t.Fatal("copy lost value")
			}
			*got = "copied"
			if original != "original" {
				t.Fatal("typed composition aliases original")
			}
		})
	}
	t.Run("repeated-record-reference", func(t *testing.T) {
		type record struct{ First, Second definedStringPointer }
		original := "original"
		input := definedStringPointer(&original)
		copied := Value(record{input, input})
		if copied.First == input || copied.First != copied.Second {
			t.Fatal("reference identity or isolation changed")
		}
		*copied.First = "changed"
		if *copied.Second != "changed" || original != "original" {
			t.Fatal("shared copy storage is incorrect")
		}
	})
}
