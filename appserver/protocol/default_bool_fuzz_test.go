package protocol_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type aliasBooleanProbe struct{ Null bool }

func (b *aliasBooleanProbe) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		b.Null = true
		return nil
	}
	var value bool
	return json.Unmarshal(raw, &value)
}

func FuzzNativeDefaultBooleanAliases(f *testing.F) {
	for strategy := uint8(0); strategy < 6; strategy++ {
		for value := uint8(0); value < 3; value++ {
			f.Add("key", strategy, value, false)
			f.Add("日本語", strategy, value, true)
		}
	}
	f.Fuzz(func(t *testing.T, text string, strategy, value uint8, duplicate bool) {
		if len(text) > 256 {
			text = text[:256]
		}
		for _, tc := range nativeDefaultBools {
			key := tc.field
			switch strategy % 6 {
			case 1:
				key = strings.ToUpper(key)
			case 2:
				key = strings.ToLower(key)
			case 3:
				if strings.Contains(key, "s") {
					key = strings.Replace(key, "s", "ſ", 1)
				} else if strings.Contains(key, "k") {
					key = strings.Replace(key, "k", "K", 1)
				}
			case 5:
				key = "extra_" + text
			}
			encodedKey, err := json.Marshal(key)
			if err != nil {
				t.Fatal(err)
			}
			if strategy%6 == 4 {
				encodedKey = []byte(fmt.Sprintf(`"\u%04x%s"`, key[0], key[1:]))
			}
			rawValue := []string{"false", "true", "null"}[value%3]
			members := string(encodedKey) + ":" + rawValue
			if duplicate {
				members += fmt.Sprintf(`,%q:true`, tc.field)
			}
			// The native reference invokes a custom field observer at every matching
			// occurrence. Its field matching is independent of jsonobject helpers.
			refType := reflect.StructOf([]reflect.StructField{{Name: "Flag", Type: reflect.TypeFor[aliasBooleanProbe](), Tag: reflect.StructTag(fmt.Sprintf(`json:%q`, tc.field))}})
			reference := reflect.New(refType)
			if err := json.Unmarshal([]byte("{"+members+"}"), reference.Interface()); err != nil {
				t.Fatal(err)
			}
			sawNull := reference.Elem().Field(0).Interface().(aliasBooleanProbe).Null
			err = json.Unmarshal([]byte(defaultBoolBody(tc.owner, ","+members)), defaultBoolTarget(tc.owner))
			if (err != nil) != sawNull || (sawNull && !errors.Is(err, p.ErrNullResultField)) {
				t.Fatalf("native field matching disagreement: owner=%s field=%s key=%q value=%s duplicate=%v sawNull=%v error=%v", tc.owner, tc.field, key, rawValue, duplicate, sawNull, err)
			}
		}
	})
}
