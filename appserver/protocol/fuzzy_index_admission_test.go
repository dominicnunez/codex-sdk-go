package protocol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestFuzzyIndexPublicAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"omitted", ``, true},
		{"max", `[4294967295]`, true}, {"duplicates", `[1,2],"indices":[]`, true},
		{"reset", `[2],"INDICES":null`, true},
		{"folded", `[0],"INDICES":[null]`, false},
		{"unicode folded", `[],"indice\u017f":[null]`, false},
		{"escaped", `[],"ind\u0069ces":[null]`, false},
		{"hidden reset", `[0,null,2],"indices":null`, false},
		{"wrong array", `{}`, false}, {"negative", `[-1]`, false},
		{"overflow", `[4294967296]`, false}, {"fraction", `[1.5]`, false},
		{"string", `["0"]`, false}, {"boolean", `[false]`, false},
		{"object", `[{}]`, false}, {"nested", `[[0]]`, false},
		{"null-array", `null`, true}, {"empty", `[]`, true}, {"zero", `[0]`, true}, {"values", `[0,2]`, true},
		{"null-item", `[null]`, false}, {"hidden-null-item", `[null],"indices":[0]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockTransport()
			reports := 0
			client := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, err error) {
				if origin != "fuzzyFileSearch/sessionUpdated" || err == nil {
					t.Errorf("bad diagnostic: %s %v", origin, err)
				}
				reports++
			}))
			t.Cleanup(func() { _ = client.Close() })
			file := fuzzyIndexRecord(tc.raw)
			type native p.FuzzyFileSearchResult
			var expected native
			if tc.valid {
				if err := json.Unmarshal([]byte(file), &expected); err != nil {
					t.Fatal(err)
				}
			}
			want := p.FuzzyFileSearchResult(expected)
			mock.SetResponse("fuzzyFileSearch", p.Response{Result: json.RawMessage(`{"files":[` + file + `]}`)})
			result, err := client.FuzzyFileSearch.Search(context.Background(), p.FuzzyFileSearchParams{Query: "p", Roots: []string{"/"}})
			on, add := 0, 0
			client.OnFuzzyFileSearchSessionUpdated(func(n p.FuzzyFileSearchSessionUpdatedNotification) {
				on++
				if len(n.Files) != 1 || !reflect.DeepEqual(n.Files[0], want) {
					t.Error("On result changed")
				}
			})
			remove := client.AddFuzzyFileSearchSessionUpdatedListener(func(n p.FuzzyFileSearchSessionUpdatedNotification) {
				add++
				if len(n.Files) != 1 || !reflect.DeepEqual(n.Files[0], want) {
					t.Error("Add result changed")
				}
			})
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "fuzzyFileSearch/sessionUpdated", Params: json.RawMessage(`{"sessionId":"s","query":"p","files":[` + file + `]}`)})
			if tc.valid {
				if err != nil || on != 1 || add != 1 || reports != 0 || len(result.Files) != 1 || !reflect.DeepEqual(result.Files[0], want) {
					t.Fatalf("control err=%v On=%d Add=%d", err, on, add)
				}
			} else {
				if err == nil || !reflect.DeepEqual(result, p.FuzzyFileSearchResponse{}) {
					t.Error("malformed indices published as successful search")
				}
				if on != 0 || add != 0 || reports != 2 {
					t.Errorf("malformed indices delivered: On=%d Add=%d", on, add)
				}

				good := fuzzyIndexRecord(`[0,4294967295]`)
				if err := json.Unmarshal([]byte(good), &want); err != nil {
					t.Fatal(err)
				}
				mock.SetResponse("fuzzyFileSearch", p.Response{Result: json.RawMessage(`{"files":[` + good + `]}`)})
				recovered, recoveryErr := client.FuzzyFileSearch.Search(context.Background(), p.FuzzyFileSearchParams{Roots: []string{"/"}})
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "fuzzyFileSearch/sessionUpdated", Params: json.RawMessage(`{"sessionId":"s","query":"p","files":[` + good + `]}`)})
				if recoveryErr != nil || len(recovered.Files) != 1 || !reflect.DeepEqual(recovered.Files[0], want) || on != 1 || add != 1 || reports != 2 {
					t.Fatal("same-client recovery failed")
				}
			}
		})
	}
}

func fuzzyIndexRecord(indices string) string {
	raw := `{"path":"relative/../p","file_name":"","root":"opaque","score":0,"match_type":"file"`
	if indices != "" {
		raw += `,"indices":` + indices
	}
	return raw + `}`
}

func TestFuzzyIndexReceiverAndErrors(t *testing.T) {
	prior := []uint32{7, 8}
	value := p.FuzzyFileSearchResult{Path: "prior", Indices: &prior}
	before := value
	if err := value.UnmarshalJSON([]byte(fuzzyIndexRecord(`[null],"indices":[0]`))); err == nil {
		t.Fatal("null accepted")
	}
	if !reflect.DeepEqual(value, before) || value.Indices != before.Indices || &(*value.Indices)[0] != &prior[0] {
		t.Fatal("receiver/reference changed")
	}
	for _, indices := range []string{"", `null`, `[]`, `[0,4294967295]`} {
		if err := value.UnmarshalJSON([]byte(fuzzyIndexRecord(indices))); err != nil {
			t.Fatal(err)
		}
		if value.Path != "relative/../p" || value.Root != "opaque" || value.FileName != "" {
			t.Fatal("opaque strings changed")
		}
		if indices == "" || indices == "null" {
			if value.Indices != nil {
				t.Fatal("optional value retained")
			}
		} else if value.Indices == nil || *value.Indices == nil {
			t.Fatal("present array lost")
		}
	}
	// The old required-field and native wire errors precede the new guard.
	for _, change := range [][2]string{{`"path":"relative/../p",`, ``}, {`"score":0`, `"score":"bad"`}, {`"match_type":"file"`, `"match_type":"bad"`}, {`"indices":[0]`, `"indices":[0],"indices":{}`}} {
		control := strings.Replace(fuzzyIndexRecord(`[0]`), change[0], change[1], 1)
		mixed := strings.Replace(control, `"indices":[0]`, `"indices":[null]`, 1)
		a, b := value.UnmarshalJSON([]byte(control)), value.UnmarshalJSON([]byte(mixed))
		if a == nil || b == nil || a.Error() != b.Error() {
			t.Fatalf("old error changed: %v / %v", a, b)
		}
	}
	var response p.FuzzyFileSearchResponse
	var notification p.FuzzyFileSearchSessionUpdatedNotification
	good := fuzzyIndexRecord(`[7]`)
	if err := json.Unmarshal([]byte(`{"files":[`+good+`]}`), &response); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"sessionId":"s","query":"q","files":[`+good+`]}`), &notification); err != nil {
		t.Fatal(err)
	}
	oldResponse, oldNotification := response, notification
	bad := fuzzyIndexRecord(`[null]`)
	if err := response.UnmarshalJSON([]byte(`{"files":[` + bad + `]}`)); err == nil {
		t.Fatal("response accepted")
	}
	if err := notification.UnmarshalJSON([]byte(`{"sessionId":"new","query":"new","files":[` + bad + `]}`)); err == nil {
		t.Fatal("notification accepted")
	}
	if !reflect.DeepEqual(response, oldResponse) || !reflect.DeepEqual(notification, oldNotification) || &response.Files[0] != &oldResponse.Files[0] || &notification.Files[0] != &oldNotification.Files[0] {
		t.Fatal("outer owner changed on failure")
	}
}

// The generator constructs valid uint32 arrays independently; inserting a null
// must fail even if a later alias resets the array. Valid duplicate histories
// must preserve the final native value and optional/null/empty distinctions.
func FuzzFuzzyIndexAdmission(f *testing.F) {
	for _, n := range []uint32{0, 1, 4294967295} {
		for mode := uint8(0); mode < 6; mode++ {
			f.Add(n, mode)
		}
	}
	f.Fuzz(func(t *testing.T, n uint32, mode uint8) {
		indices := fmt.Sprintf(`[%d,0]`, n)
		valid := true
		switch mode % 6 {
		case 0:
			indices = ""
		case 1:
			indices = `null`
		case 2:
			indices = `[1],"INDICES":[]`
		case 3:
		case 4:
			indices = `[null],"indices":` + indices
			valid = false
		case 5:
			indices = indices + `,"indice\u017f":[null],"indices":null`
			valid = false
		}
		raw := fuzzyIndexRecord(indices)
		var got p.FuzzyFileSearchResult
		err := json.Unmarshal([]byte(raw), &got)
		if !valid {
			if err == nil || !reflect.DeepEqual(got, p.FuzzyFileSearchResult{}) {
				t.Fatal("invalid history published")
			}
			return
		}
		type native p.FuzzyFileSearchResult
		var want native
		if e := json.Unmarshal([]byte(raw), &want); e != nil {
			t.Fatal(e)
		}
		if err != nil || !reflect.DeepEqual(got, p.FuzzyFileSearchResult(want)) {
			t.Fatalf("valid native result lost: %v", err)
		}
		encoded, e := json.Marshal(got)
		if e != nil {
			t.Fatal(e)
		}
		var round p.FuzzyFileSearchResult
		if e = json.Unmarshal(encoded, &round); e != nil || !reflect.DeepEqual(got, round) {
			t.Fatalf("roundtrip lost value: %v", e)
		}
	})
}

func TestFuzzyIndexCompositions(t *testing.T) {
	bad := fuzzyIndexRecord(`[null]`)
	// A containing files replacement must not hide a failed result decoder.
	for _, payload := range []string{`{"files":[` + bad + `],"files":[]}`, `{"sessionId":"s","query":"q","files":[` + bad + `],"files":[]}`} {
		var response p.FuzzyFileSearchResponse
		var notification p.FuzzyFileSearchSessionUpdatedNotification
		if err := json.Unmarshal([]byte(payload), &response); err == nil {
			t.Fatal("response replacement hid bad result")
		}
		if err := json.Unmarshal([]byte(payload), &notification); err == nil {
			t.Fatal("notification replacement hid bad result")
		}
	}
	// Existing JSON methods are preserved: named wrappers decode their own
	// fields, while anonymous embedding promotes the result decoder.
	var named struct {
		Result p.FuzzyFileSearchResult
		After  int
	}
	if err := json.Unmarshal([]byte(`{"Result":`+fuzzyIndexRecord(`[0]`)+`,"After":7}`), &named); err != nil || named.After != 7 || named.Result.Indices == nil {
		t.Fatal("named composition changed")
	}
	var embedded struct {
		p.FuzzyFileSearchResult
		After int
	}
	raw := strings.TrimSuffix(fuzzyIndexRecord(`[0]`), "}") + `,"After":7}`
	if err := json.Unmarshal([]byte(raw), &embedded); err != nil || embedded.After != 0 || embedded.Indices == nil {
		t.Fatal("promoted decoder changed")
	}
	named.Result = p.FuzzyFileSearchResult{Path: "prior"}
	if err := json.Unmarshal([]byte(`{"Result":`+bad+`,"After":9}`), &named); err == nil || named.Result.Path != "prior" {
		t.Fatal("named failure published result")
	}
}
