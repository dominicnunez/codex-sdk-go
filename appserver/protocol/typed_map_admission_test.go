package protocol_test

import (
	"context"
	"encoding/json"
	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
	"testing"
)

func TestTypedMapEntryAdmission(t *testing.T) {
	for _, field := range []string{"labels", "iconAssets", "iconDarkAssets", "featureRequirements", "origins", "enablement"} {
		value := `""`
		if field == "featureRequirements" || field == "enablement" {
			value = `false`
		}
		if field == "origins" {
			value = `{}`
		}
		for _, tc := range []struct {
			name, raw string
			valid     bool
		}{
			{"empty", `{}`, true}, {"value", `{"key":` + value + `}`, true}, {"duplicate-valid", `{"key":` + value + `,"key":` + value + `}`, true},
			{"null-entry", `{"key":null}`, false}, {"hidden-null", `{"key":null,"key":` + value + `}`, false},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				client := p.NewClient(mock)
				t.Cleanup(func() { _ = client.Close() })
				var err error
				switch field {
				case "labels", "iconAssets", "iconDarkAssets":
					body := `{"data":[{"id":"a","name":"A","` + field + `":` + tc.raw + `}]}`
					mock.SetResponse("app/list", p.Response{Result: json.RawMessage(body)})
					resp, e := client.Apps.List(context.Background(), p.AppsListParams{})
					err = e
					if tc.valid && (e != nil || len(resp.Data) != 1) {
						t.Fatalf("valid listing failed: %v", e)
					}
					on, add := 0, 0
					client.OnAppListUpdated(func(p.AppListUpdatedNotification) { on++ })
					remove := client.AddAppListUpdatedListener(func(p.AppListUpdatedNotification) { add++ })
					defer remove()
					mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: json.RawMessage(body)})
					want := 0
					if tc.valid {
						want = 1
					}
					if on != want || add != want {
						t.Errorf("delivery On=%d Add=%d want=%d", on, add, want)
					}
				case "featureRequirements":
					mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{"featureRequirements":` + tc.raw + `}}`)})
					_, err = client.Config.ReadRequirements(context.Background())
				case "origins":
					mock.SetResponse("config/read", p.Response{Result: json.RawMessage(`{"config":{"browser_use":{"origins":` + tc.raw + `}},"origins":{}}`)})
					_, err = client.Config.Read(context.Background(), p.ConfigReadParams{})
				case "enablement":
					mock.SetResponse("experimentalFeature/enablement/set", p.Response{Result: json.RawMessage(`{"enablement":` + tc.raw + `}`)})
					_, err = client.Experimental.FeatureEnablementSet(context.Background(), p.ExperimentalFeatureEnablementSetParams{Enablement: map[string]bool{}})
				}
				if tc.valid && err != nil {
					t.Fatal(err)
				}
				if !tc.valid && err == nil {
					t.Errorf("schema-forbidden null map entry published: %s=%s", field, tc.raw)
				}
			})
		}
	}
}
