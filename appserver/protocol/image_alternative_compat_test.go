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

func TestImageAlternativeHistory(t *testing.T) {
	for _, tool := range []bool{false, true} {
		name, urlName, fileName := "image", "url", "fileId"
		if tool {
			name, urlName, fileName = "input_image", "image_url", "file_id"
		}
		for _, tc := range []struct {
			fields    string
			url, file *string
			valid     bool
		}{
			{`"U":"","F":7,"F":"later"`, p.Ptr(""), nil, true},
			{`"U":false,"U":"later","F":"file"`, nil, p.Ptr("file"), true},
			{`"U":"old","U":null,"F":"file"`, nil, p.Ptr("file"), true},
			{`"U":"","F":"old","F":null`, p.Ptr(""), nil, true},
			{`"U":"old","U":null`, nil, nil, false},
			{`"U":false,"U":"later"`, nil, nil, false},
			{`"U":"","detail":"bad"`, nil, nil, false},
			{`"U":"","detail":17`, nil, nil, false},
			{`"U":"","detail":null`, p.Ptr(""), nil, true},
		} {
			raw := `{"type":"` + name + `",` + strings.NewReplacer(`"U"`, `"`+urlName+`"`, `"F"`, `"`+fileName+`"`).Replace(tc.fields) + `}`
			oldURL, oldFile, oldDetail := p.Ptr("prior-url"), p.Ptr("prior-file"), p.Ptr(p.ImageDetailLow)
			user := p.ImageUserInput{URL: oldURL, FileID: oldFile, Detail: oldDetail}
			output := p.InputImageFunctionCallOutputContentItem{ImageURL: oldURL, FileID: oldFile, Detail: oldDetail}
			var err error
			var url, file *string
			if tool {
				err = output.UnmarshalJSON([]byte(raw))
				url, file = output.ImageURL, output.FileID
			} else {
				err = user.UnmarshalJSON([]byte(raw))
				url, file = user.URL, user.FileID
			}
			if tc.valid {
				if err != nil || !reflect.DeepEqual(url, tc.url) || !reflect.DeepEqual(file, tc.file) {
					t.Fatalf("history %s: %v URL=%v file=%v", raw, err, url, file)
				}
			} else if err == nil || url != oldURL || file != oldFile || *oldURL != "prior-url" || *oldFile != "prior-file" || user.Detail != oldDetail || output.Detail != oldDetail {
				t.Fatalf("failed receiver changed: %s %v", raw, err)
			}
		}
	}
}

func FuzzImageAlternatives(f *testing.F) {
	for u := uint8(0); u < 7; u++ {
		for v := uint8(0); v < 7; v++ {
			f.Add(u, v, "opaque", false)
			f.Add(u, v, "", true)
		}
	}
	f.Fuzz(func(t *testing.T, u, v uint8, text string, tool bool) {
		if len(text) > 512 {
			text = text[:512]
		}
		quoted, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		choices := []string{string(quoted), `""`, `null`, `false`, `17`, `{}`, `[]`}
		urlRaw, fileRaw := choices[int(u)%len(choices)], choices[int(v)%len(choices)]
		name, urlName, fileName := "image", "url", "fileId"
		if tool {
			name, urlName, fileName = "input_image", "image_url", "file_id"
		}
		raw := fmt.Sprintf(`{"type":%q,%q:%s,%q:%s}`, name, urlName, urlRaw, fileName, fileRaw)
		var value any
		if tool {
			out := new(p.InputImageFunctionCallOutputContentItem)
			err = json.Unmarshal([]byte(raw), out)
			value = out
		} else {
			out := new(p.ImageUserInput)
			err = json.Unmarshal([]byte(raw), out)
			value = out
		}
		valid := int(u)%len(choices) < 2 || int(v)%len(choices) < 2
		if !valid {
			if err == nil {
				t.Fatal("neither alternative valid but decode succeeded")
			}
			return
		}
		if err != nil {
			t.Fatalf("valid independent alternative rejected: %s %v", raw, err)
		}
		checkImageAlternatives(t, value, raw, tool)
		encoded, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		var round any
		if tool {
			out := new(p.InputImageFunctionCallOutputContentItem)
			e = json.Unmarshal(encoded, out)
			round = out
		} else {
			out := new(p.ImageUserInput)
			e = json.Unmarshal(encoded, out)
			round = out
		}
		if e != nil || !reflect.DeepEqual(value, round) {
			t.Fatalf("roundtrip changed selected image: %v", e)
		}
	})
}

func TestImageAlternativeAliases(t *testing.T) {
	for _, tool := range []bool{false, true} {
		name, urlName, fileName := "image", "url", "fileId"
		if tool {
			name, urlName, fileName = "input_image", "image_url", "file_id"
		}
		for _, escaped := range []bool{false, true} {
			urlKey, fileKey := strings.ToUpper(urlName), strings.ToUpper(fileName)
			if escaped {
				urlKey = fmt.Sprintf(`\u%04x%s`, urlName[0], urlName[1:])
				fileKey = fmt.Sprintf(`\u%04x%s`, fileName[0], fileName[1:])
			}
			raw := `{"type":"` + name + `","` + urlKey + `":"","` + fileKey + `":17}`
			if tool {
				var value p.InputImageFunctionCallOutputContentItem
				if err := json.Unmarshal([]byte(raw), &value); err != nil || value.ImageURL == nil || *value.ImageURL != "" || value.FileID != nil {
					t.Fatalf("tool alias mismatch: %s %v", raw, err)
				}
			} else {
				var value p.ImageUserInput
				if err := json.Unmarshal([]byte(raw), &value); err != nil || value.URL == nil || *value.URL != "" || value.FileID != nil {
					t.Fatalf("user alias mismatch: %s %v", raw, err)
				}
			}
		}
	}
}

func TestImageAlternativeNativeErrors(t *testing.T) {
	for _, tool := range []bool{false, true} {
		for _, shared := range []bool{false, true} {
			raw := `{"url":17,"fileId":false}`
			if shared {
				raw = `{"url":"valid","detail":17}`
			}
			var reference any = &struct {
				URL    string         `json:"url"`
				FileID *string        `json:"fileId"`
				Detail *p.ImageDetail `json:"detail"`
			}{}
			var err error
			if tool {
				raw = strings.NewReplacer(`"url"`, `"image_url"`, `"fileId"`, `"file_id"`).Replace(raw)
				reference = &struct {
					ImageURL string         `json:"image_url"`
					FileID   *string        `json:"file_id"`
					Detail   *p.ImageDetail `json:"detail"`
				}{}
				var value p.InputImageFunctionCallOutputContentItem
				err = value.UnmarshalJSON([]byte(raw))
			} else {
				var value p.ImageUserInput
				err = value.UnmarshalJSON([]byte(raw))
			}
			oldErr := json.Unmarshal([]byte(raw), reference)
			var before, after *json.UnmarshalTypeError
			if !errors.As(oldErr, &before) || !errors.As(err, &after) || before.Value != after.Value || before.Type != after.Type || before.Offset != after.Offset || before.Field != after.Field {
				t.Fatalf("native error changed: %v / %v", oldErr, err)
			}
		}
	}
}
