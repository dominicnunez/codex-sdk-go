package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func FuzzOptionalStringArrayNulls(f *testing.F) {
	f.Add([]byte("plain"), uint8(0), false)
	f.Add([]byte{0, 1, 2, 3, 4}, uint8(1), true)
	f.Add([]byte("escaped\n\"\\null"), uint8(2), true)
	f.Fuzz(func(t *testing.T, data []byte, spelling uint8, duplicate bool) {
		if len(data) > 512 {
			return
		}
		names := []string{"summary", "SUMMARY", `\u0073ummary`, `\u017fummary`}
		makeList := func(reverse bool) []byte {
			items := make([]json.RawMessage, 0, len(data))
			for i := range data {
				index := i
				if reverse {
					index = len(data) - 1 - i
				}
				switch data[index] % 5 {
				case 0:
					items = append(items, json.RawMessage(`null`))
				case 1:
					items = append(items, json.RawMessage(`{"nested":null}`))
				case 2:
					items = append(items, json.RawMessage(`[null]`))
				default:
					value, err := json.Marshal(string(data[index:]))
					if err != nil {
						t.Fatal(err)
					}
					items = append(items, value)
				}
			}
			encoded, err := json.Marshal(items)
			if err != nil {
				t.Fatal(err)
			}
			return encoded
		}
		first, last := makeList(false), makeList(true)
		if spelling&8 != 0 {
			first = []byte(`null`)
		}
		if spelling&16 != 0 {
			last = []byte(`null`)
		}
		body := `{"ignored":{"summary":null},"` + names[int(spelling)%len(names)] + `":` + string(first)
		want := optionalArrayHasNull(first)
		if duplicate {
			body += `,"summary":` + string(last)
			want = want || optionalArrayHasNull(last)
		}
		body += `}`
		if !json.Valid([]byte(body)) {
			t.Fatal("generator produced invalid JSON")
		}
		err := validateOptionalStringArrays([]byte(body), "summary")
		if (err != nil) != want {
			t.Fatalf("body=%s error=%v wantNullFailure=%v", body, err, want)
		}
		// Valid strings containing the token must not be treated as null.
		quoted, err := json.Marshal(strings.Repeat("null", 1+len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if err := validateOptionalStringArrays([]byte(`{"summary":[`+string(quoted)+`]}`), "summary"); err != nil {
			t.Fatal(err)
		}
	})
}

func optionalArrayHasNull(raw []byte) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	for _, item := range items {
		if bytes.Equal(bytes.TrimSpace(item), []byte("null")) {
			return true
		}
	}
	return false
}
