package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func requireJSONMember(t *testing.T, value any, key string, present bool, expected string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		t.Fatal(err)
	}
	raw, found := members[key]
	if found != present || (found && string(raw) != expected) {
		t.Errorf("%s: present=%v raw=%s, want present=%v raw=%s", key, found, raw, present, expected)
	}
}

func TestNullableCollectionsPublicRoundTrip(t *testing.T) {
	for _, state := range []string{"absent", "null", "empty"} {
		t.Run(state, func(t *testing.T) {
			detail := issue74PluginDetail()
			summary := detail["summary"].(map[string]any)
			delete(detail, "scheduledTasks")
			delete(summary, "eligiblePlanTypes")
			if state != "absent" {
				detail["scheduledTasks"], summary["eligiblePlanTypes"] = nil, nil
				if state == "empty" {
					detail["scheduledTasks"], summary["eligiblePlanTypes"] = []any{}, []any{}
				}
			}
			response, err := issue74Read(t, detail)
			if err != nil {
				t.Fatal(err)
			}
			want := "null"
			if state == "empty" {
				want = "[]"
			}
			requireJSONMember(t, response.Plugin, "scheduledTasks", state != "absent", want)
			requireJSONMember(t, &response.Plugin.Summary, "eligiblePlanTypes", state != "absent", want)
			properties := ``
			if state != "absent" {
				properties = `,"days":` + want
			}
			var hourly codex.HourlyScheduledTaskSchedule
			if err := json.Unmarshal([]byte(`{"type":"hourly","intervalHours":1`+properties+`}`), &hourly); err != nil {
				t.Fatal(err)
			}
			requireJSONMember(t, &hourly, "days", state != "absent", want)
			configBody := `{}`
			if state != "absent" {
				want = "null"
				if state == "empty" {
					want = "{}"
				}
				configBody = `{"desktop":` + want + `}`
			}
			mock := NewMockTransport()
			if err := mock.SetResponseData("config/read", json.RawMessage(`{"config":`+configBody+`,"origins":{}}`)); err != nil {
				t.Fatal(err)
			}
			client := codex.NewClient(mock)
			defer client.Close()
			config, err := client.Config.Read(context.Background(), codex.ConfigReadParams{})
			if err != nil {
				t.Fatal(err)
			}
			requireJSONMember(t, config.Config, "desktop", state != "absent", want)
		})
	}
}

func TestDesktopMapDecodeHistory(t *testing.T) {
	var config codex.Config
	for _, body := range []string{`{"desktop":{"a":1}}`, `{}`, `{"desktop":{"b":null}}`} {
		if err := json.Unmarshal([]byte(body), &config); err != nil {
			t.Fatal(err)
		}
	}
	requireWireFields(t, config, `{"desktop":{"a":1,"b":null}}`)
	for _, body := range []string{
		`{"desktop":{"a":1},"DESKTOP":{"b":null}}`,
		`{"desktop":{"a":1},"desktop":null,"desktop":{"b":null}}`,
	} {
		var fresh codex.Config
		if err := json.Unmarshal([]byte(body), &fresh); err != nil {
			t.Fatal(err)
		}
		want := `{"desktop":{"a":1,"b":null}}`
		if body == `{"desktop":{"a":1},"desktop":null,"desktop":{"b":null}}` {
			want = `{"desktop":{"b":null}}`
		}
		requireWireFields(t, fresh, want)
	}
}

func TestNullableCollectionReceiverHistory(t *testing.T) {
	var summary codex.PluginSummary
	for _, suffix := range []string{
		`,"eligiblePlanTypes":["one"],"ELIGIBLEPLANTYPES":null`,
		`,"eligiblePlanTypes":null,"ELIGIBLEPLANTYPES":[]`,
	} {
		body := issue74PluginSummary[:len(issue74PluginSummary)-1] + suffix + `}`
		if err := json.Unmarshal([]byte(body), &summary); err != nil {
			t.Fatal(err)
		}
		want := "null"
		if suffix[len(suffix)-2:] == "[]" {
			want = "[]"
		}
		requireJSONMember(t, summary, "eligiblePlanTypes", true, want)
	}
	// Summary uses a fresh wire destination on each call; unlike Config, omitted
	// collections reset their prior presence rather than merging a reused value.
	body := `{"authPolicy":"ON_USE","enabled":true,"id":"p","installPolicy":"AVAILABLE","installed":false,"name":"P","source":{"type":"remote"}}`
	if err := json.Unmarshal([]byte(body), &summary); err != nil {
		t.Fatal(err)
	}
	requireJSONMember(t, summary, "eligiblePlanTypes", false, "")
	var hourly codex.HourlyScheduledTaskSchedule
	if err := json.Unmarshal([]byte(`{"type":"hourly","intervalHours":1,"days":null}`), &hourly); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"type":"hourly","intervalHours":2}`), &hourly); err != nil {
		t.Fatal(err)
	}
	requireJSONMember(t, hourly, "days", false, "")
	for _, properties := range []string{
		`,"days":[],"DAYS":null`,
		`,"days":null,"DAYS":[]`,
	} {
		if err := json.Unmarshal([]byte(`{"type":"hourly","intervalHours":1`+properties+`}`), &hourly); err != nil {
			t.Fatal(err)
		}
		want := "null"
		if properties[len(properties)-2:] == "[]" {
			want = "[]"
		}
		requireJSONMember(t, hourly, "days", true, want)
	}
	data, err := json.Marshal(issue74PluginDetail())
	if err != nil {
		t.Fatal(err)
	}
	var detail codex.PluginDetail
	for _, suffix := range []string{`,"scheduledTasks":[],"SCHEDULEDTASKS":null`, `,"scheduledTasks":null,"SCHEDULEDTASKS":[]`} {
		body := string(data[:len(data)-1]) + suffix + `}`
		if err := json.Unmarshal([]byte(body), &detail); err != nil {
			t.Fatal(err)
		}
		want := "null"
		if suffix[len(suffix)-2:] == "[]" {
			want = "[]"
		}
		requireJSONMember(t, detail, "scheduledTasks", true, want)
	}
	fixture := issue74PluginDetail()
	delete(fixture, "scheduledTasks")
	data, err = json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &detail); err != nil {
		t.Fatal(err)
	}
	requireJSONMember(t, detail, "scheduledTasks", false, "")
}

func TestConfigNullableFieldErrorCompatibility(t *testing.T) {
	// Independent stdlib reference for established fields and map merges.
	type Config struct {
		Model        *string                    `json:"model,omitempty"`
		Instructions *string                    `json:"instructions,omitempty"`
		Desktop      map[string]json.RawMessage `json:"desktop,omitempty"`
	}
	for _, body := range []string{
		`{"model":7,"desktop":{"new":true},"instructions":"later"}`,
		`{"desktop":{"new":true},"model":7,"instructions":"later"}`,
		`{"model":7,"instructions":"later"}`,
	} {
		var actual codex.Config
		var reference Config
		const initial = `{"desktop":{"old":false},"model":"old"}`
		if err := json.Unmarshal([]byte(initial), &actual); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(initial), &reference); err != nil {
			t.Fatal(err)
		}
		retained := *actual.Desktop.Value
		wantErr := json.Unmarshal([]byte(body), &reference)
		gotErr := json.Unmarshal([]byte(body), &actual)
		var want, got *json.UnmarshalTypeError
		if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) || !reflect.DeepEqual(got, want) {
			t.Fatalf("error=%+v, want %+v", got, want)
		}
		if !reflect.DeepEqual(actual.Model, reference.Model) || !reflect.DeepEqual(actual.Instructions, reference.Instructions) || !reflect.DeepEqual(*actual.Desktop.Value, reference.Desktop) || !reflect.DeepEqual(retained, reference.Desktop) {
			t.Fatal("receiver updates or retained maps differ")
		}
	}
}

func FuzzDesktopNullableHistory(f *testing.F) {
	for _, seed := range [][]byte{{}, {1}, {2}, {3}, {2, 3, 1, 2}, {2, 0, 3}, {3, 2, 1, 0, 3}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, history []byte) {
		if len(history) > 64 {
			history = history[:64]
		}
		var config codex.Config
		var reference struct {
			Desktop map[string]json.RawMessage `json:"desktop"`
		}
		present := false
		for _, operation := range history {
			var body string
			switch operation % 4 {
			case 0:
				body = `{}`
			case 1:
				body = `{"desktop":null}`
			case 2:
				body = `{"desktop":{"a":1}}`
			case 3:
				body = `{"DESKTOP":{"b":null}}`
			}
			if err := json.Unmarshal([]byte(body), &reference); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(body), &config); err != nil {
				t.Fatal(err)
			}
			if operation%4 != 0 {
				present = true
			}
			if config.Desktop.Present != present {
				t.Fatal("presence diverged")
			}
			data, err := json.Marshal(reference.Desktop)
			if err != nil {
				t.Fatal(err)
			}
			requireJSONMember(t, config, "desktop", present, string(data))
		}
	})
}
