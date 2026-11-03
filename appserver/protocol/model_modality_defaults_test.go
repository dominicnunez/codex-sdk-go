package protocol_test

import (
	"context"
	"encoding/json"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"reflect"
	"strings"
	"testing"
)

func TestModelModalityDefaults(t *testing.T) {
	base := `"id":"m","model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]`
	for _, tc := range []struct {
		name, member string
		want         []p.InputModality
	}{
		{"omitted", "", []p.InputModality{p.InputModalityText, p.InputModalityImage}},
		{"explicit-empty", `,"inputModalities":[]`, []p.InputModality{}},
		{"text", `,"inputModalities":["text"]`, []p.InputModality{p.InputModalityText}},
		{"image", `,"inputModalities":["image"]`, []p.InputModality{p.InputModalityImage}},
		{"both", `,"inputModalities":["text","image"]`, []p.InputModality{p.InputModalityText, p.InputModalityImage}},
		{"audio", `,"inputModalities":["audio"]`, []p.InputModality{p.InputModalityAudio}},
		{"duplicates", `,"inputModalities":["text"],"INPUTMODALITIES":[]`, []p.InputModality{}},
		{"escaped", `,"inputModalit\u0069es":["image"]`, []p.InputModality{p.InputModalityImage}},
		{"reverse", `,"inputModalities":["image","text"]`, []p.InputModality{p.InputModalityImage, p.InputModalityText}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`{` + base + tc.member + `}`)
			var direct p.Model
			if err := json.Unmarshal(raw, &direct); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(direct.InputModalities, tc.want) {
				t.Errorf("direct modalities=%#v want=%#v", direct.InputModalities, tc.want)
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("model/list", p.Response{Result: json.RawMessage(`{"data":[` + string(raw) + `]}`)})
			response, err := client.Model.List(context.Background(), p.ModelListParams{})
			if err != nil || len(response.Data) != 1 {
				t.Fatalf("list err=%v data=%+v", err, response)
			}
			got := response.Data[0]
			if !reflect.DeepEqual(got.InputModalities, tc.want) {
				t.Errorf("public list modalities=%#v want=%#v", got.InputModalities, tc.want)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var out map[string]json.RawMessage
			if err = json.Unmarshal(encoded, &out); err != nil {
				t.Fatal(err)
			}
			if tc.name == "explicit-empty" {
				if string(out["inputModalities"]) != "[]" {
					t.Errorf("explicit empty list erased: %s", encoded)
				}
			}
			var again p.Model
			if err = json.Unmarshal(encoded, &again); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(again.InputModalities, tc.want) {
				t.Errorf("round-trip modalities=%#v want=%#v", again.InputModalities, tc.want)
			}
		})
	}
}
func TestModelModalityNullControls(t *testing.T) {
	base := `"id":"m","model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]`
	for _, raw := range []string{`null`, `[null]`, `null,"inputModalities":["text"]`, `[null],"INPUTMODALITIES":[]`, `["unknown"],"inputModalities":[]`, `["text"],"inputModalities":null`, `[1]`, `{}`} {
		t.Run(raw, func(t *testing.T) {
			body := []byte(`{` + base + `,"inputModalities":` + raw + `}`)
			var direct p.Model
			if err := json.Unmarshal(body, &direct); err == nil {
				t.Error("null accepted directly")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			t.Cleanup(func() { _ = client.Close() })
			mock.SetResponse("model/list", p.Response{Result: json.RawMessage(`{"data":[` + string(body) + `]}`)})
			if got, err := client.Model.List(context.Background(), p.ModelListParams{}); err == nil || !reflect.DeepEqual(got, p.ModelListResponse{}) {
				t.Error("null accepted publicly")
			}
		})
	}
}
func TestModelModalityReceiverReuse(t *testing.T) {
	base := `"id":"m","model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]`
	for _, tc := range []struct {
		name, member string
		want         []p.InputModality
		invalid      bool
	}{
		{"omitted", "", []p.InputModality{p.InputModalityText, p.InputModalityImage}, false},
		{"explicit-empty", `,"inputModalities":[]`, []p.InputModality{}, false},
		{"explicit-image", `,"inputModalities":["image"]`, []p.InputModality{p.InputModalityImage}, false},
		{"explicit-null", `,"inputModalities":null`, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v p.Model
			if err := json.Unmarshal([]byte(`{`+base+`,"inputModalities":["text"]}`), &v); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v.InputModalities, []p.InputModality{p.InputModalityText}) {
				t.Fatal("seed receiver failed")
			}
			err := json.Unmarshal([]byte(`{`+base+tc.member+`}`), &v)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid null accepted in reused receiver")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v.InputModalities, tc.want) {
				t.Errorf("reused modalities=%#v want=%#v", v.InputModalities, tc.want)
			}
			encoded, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if tc.name == "explicit-empty" && string(fields["inputModalities"]) != "[]" {
				t.Errorf("reused explicit empty list lost in JSON: %s", encoded)
			}
		})
	}
}

func modalityModelJSON(member string) string {
	return `{"id":"m","model":"m","displayName":"M","description":"","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]` + member + `}`
}

func TestModelModalityConstructedAndOwned(t *testing.T) {
	for _, list := range [][]p.InputModality{nil, {}, {p.InputModalityImage, p.InputModalityText}} {
		var value p.Model
		if err := json.Unmarshal([]byte(modalityModelJSON("")), &value); err != nil {
			t.Fatal(err)
		}
		value.InputModalities = list
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		raw, present := fields["inputModalities"]
		if list == nil {
			if present {
				t.Fatalf("constructed nil must omit: %s", raw)
			}
		} else {
			want, e := json.Marshal(list)
			if e != nil {
				t.Fatal(e)
			}
			if !present || string(raw) != string(want) {
				t.Fatalf("constructed value lost: %s", encoded)
			}
		}
		var round p.Model
		if err = json.Unmarshal(encoded, &round); err != nil {
			t.Fatal(err)
		}
		want := list
		if list == nil {
			want = []p.InputModality{p.InputModalityText, p.InputModalityImage}
		}
		if !reflect.DeepEqual(round.InputModalities, want) {
			t.Fatal("constructed roundtrip lost meaning")
		}
	}
	var first, second p.Model
	if err := json.Unmarshal([]byte(modalityModelJSON("")), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(modalityModelJSON("")), &second); err != nil {
		t.Fatal(err)
	}
	first.InputModalities[0] = p.InputModalityAudio
	if !reflect.DeepEqual(second.InputModalities, []p.InputModality{p.InputModalityText, p.InputModalityImage}) {
		t.Fatal("defaults alias another record")
	}
	old := first
	for _, bad := range []string{`null`, `[null]`, `null,"INPUTMODALITIES":[]`} {
		if err := first.UnmarshalJSON([]byte(modalityModelJSON(`,"inputModalities":` + bad))); err == nil {
			t.Fatal("null accepted")
		}
		if !reflect.DeepEqual(first, old) || &first.InputModalities[0] != &old.InputModalities[0] {
			t.Fatal("failed decode replaced prior references")
		}
	}
	var response p.ModelListResponse
	if err := json.Unmarshal([]byte(`{"data":[`+modalityModelJSON("")+`,`+modalityModelJSON("")+`]}`), &response); err != nil {
		t.Fatal(err)
	}
	response.Data[0].InputModalities[0] = p.InputModalityAudio
	if response.Data[1].InputModalities[0] != p.InputModalityText {
		t.Fatal("list defaults share backing storage")
	}
	// Only a tag changes serialization; no new marshal method is promoted into
	// application wrappers, so unrelated wrapper fields remain visible.
	wrapper := struct {
		p.Model
		Extra int
	}{Model: second, Extra: 7}
	encoded, err := json.Marshal(wrapper)
	if err != nil || !strings.Contains(string(encoded), `"Extra":7`) {
		t.Fatalf("embedded marshal changed: %s %v", encoded, err)
	}
	// Earlier native errors retain priority even alongside a forbidden null.
	for _, bad := range []string{strings.Replace(modalityModelJSON(""), `"hidden":false`, `"hidden":"bad"`, 1), strings.Replace(modalityModelJSON(""), `"supportedReasoningEfforts":[]`, `"supportedReasoningEfforts":false`, 1)} {
		control := first.UnmarshalJSON([]byte(bad))
		mixed := first.UnmarshalJSON([]byte(strings.TrimSuffix(bad, "}") + `,"inputModalities":null}`))
		if control == nil || mixed == nil || control.Error() != mixed.Error() {
			t.Fatalf("old error changed: %v / %v", control, mixed)
		}
	}
}

func FuzzModelModalityDefaults(f *testing.F) {
	for mode := uint8(0); mode < 5; mode++ {
		f.Add([]byte{0, 1, 2, 0}, mode)
	}
	f.Fuzz(func(t *testing.T, values []byte, mode uint8) {
		if len(values) > 32 {
			values = values[:32]
		}
		list := make([]p.InputModality, 0, len(values))
		choices := []p.InputModality{p.InputModalityText, p.InputModalityImage, p.InputModalityAudio}
		for _, b := range values {
			list = append(list, choices[int(b)%len(choices)])
		}
		encoded, err := json.Marshal(list)
		if err != nil {
			t.Fatal(err)
		}
		member := `,"inputModalities":` + string(encoded)
		want := list
		valid := true
		switch mode % 5 {
		case 0:
			member = ""
			want = []p.InputModality{p.InputModalityText, p.InputModalityImage}
		case 1:
			member = `,"inputModalities":[]`
			want = []p.InputModality{}
		case 2:
		case 3:
			member = `,"inputModalities":null` + member
			valid = false
		case 4:
			member = `,"inputModalities":[null],"INPUTMODALITIES":` + string(encoded)
			valid = false
		}
		old := []p.InputModality{p.InputModalityAudio}
		value := p.Model{ID: "prior", InputModalities: old}
		err = json.Unmarshal([]byte(modalityModelJSON(member)), &value)
		if !valid {
			if err == nil || value.ID != "prior" || &value.InputModalities[0] != &old[0] || value.InputModalities[0] != p.InputModalityAudio {
				t.Fatal("invalid input published")
			}
			return
		}
		if err != nil || !reflect.DeepEqual(value.InputModalities, want) {
			t.Fatalf("contract mismatch: %v %#v / %#v", err, value.InputModalities, want)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var round p.Model
		if err = json.Unmarshal(raw, &round); err != nil || !reflect.DeepEqual(round.InputModalities, want) {
			t.Fatalf("roundtrip changed meaning: %v", err)
		}
	})
}
