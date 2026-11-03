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

func typedMapRead(t *testing.T, c *p.Client, mock *MockTransport, field, members string) (any, error) {
	t.Helper()
	switch field {
	case "labels", "iconAssets", "iconDarkAssets":
		body := `{"data":[{"id":"preceding","name":"Good"},{"id":"app","name":"App"` + members + `}]}`
		mock.SetResponse("app/list", p.Response{Result: json.RawMessage(body)})
		return c.Apps.List(context.Background(), p.AppsListParams{})
	case "featureRequirements":
		mock.SetResponse("configRequirements/read", p.Response{Result: json.RawMessage(`{"requirements":{` + strings.TrimPrefix(members, ",") + `}}`)})
		return c.Config.ReadRequirements(context.Background())
	case "origins":
		mock.SetResponse("config/read", p.Response{Result: json.RawMessage(`{"config":{"browser_use":{` + strings.TrimPrefix(members, ",") + `}},"origins":{}}`)})
		return c.Config.Read(context.Background(), p.ConfigReadParams{})
	default:
		mock.SetResponse("experimentalFeature/enablement/set", p.Response{Result: json.RawMessage(`{` + strings.TrimPrefix(members, ",") + `}`)})
		return c.Experimental.FeatureEnablementSet(context.Background(), p.ExperimentalFeatureEnablementSetParams{Enablement: map[string]bool{}})
	}
}

// Independent native decoding models map merge/reset/selection. App defaults
// and containing response fields are outside this map-only reference model.
func expectedTypedMap(t *testing.T, field, members string) any {
	t.Helper()
	raw := []byte(`{` + strings.TrimPrefix(members, ",") + `}`)
	switch field {
	case "labels", "iconAssets", "iconDarkAssets":
		type native p.AppInfo
		var v native
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return appMap(p.AppInfo(v), field)
	case "origins":
		var v p.BrowserUseConfig
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v.Origins
	default:
		var v struct {
			Feature    map[string]bool `json:"featureRequirements"`
			Enablement map[string]bool `json:"enablement"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if field == "enablement" {
			return v.Enablement
		}
		return v.Feature
	}
}
func appMap(v p.AppInfo, field string) map[string]string {
	switch field {
	case "labels":
		return v.Labels
	case "iconAssets":
		return v.IconAssets
	default:
		return v.IconDarkAssets
	}
}
func actualTypedMap(result any, field string) any {
	switch v := result.(type) {
	case p.AppsListResponse:
		return appMap(v.Data[len(v.Data)-1], field)
	case p.ConfigRequirementsReadResponse:
		return v.Requirements.FeatureRequirements
	case p.ConfigReadResponse:
		return v.Config.BrowserUse.Origins
	case p.ExperimentalFeatureEnablementSetResponse:
		return v.Enablement
	default:
		panic("unexpected typed map carrier")
	}
}

func TestTypedMapOccurrenceContracts(t *testing.T) {
	for _, field := range []string{"labels", "iconAssets", "iconDarkAssets", "featureRequirements", "origins", "enablement"} {
		value := `""`
		if field == "featureRequirements" || field == "enablement" {
			value = `false`
		}
		if field == "origins" {
			value = `{}`
		}
		canonical := `,"` + field + `":`
		valid := `{"key":` + value + `}`
		for _, tc := range []struct {
			name, members string
			valid         bool
		}{
			{"omitted", "", field != "enablement"},
			{"whole-null", canonical + `null`, field != "enablement"},
			{"empty", canonical + `{}`, true},
			{"merge", canonical + `{"first":` + value + `}` + canonical + `{"second":` + value + `}`, true},
			{"reset", canonical + valid + canonical + `null`, field != "enablement"},
			{"null-then-value", canonical + `null` + canonical + valid, field != "enablement"},
			{"earlier-null-entry", canonical + `{"key":null}` + canonical + valid, false},
			{"entry-before-reset", canonical + `{"key":null}` + canonical + `null`, false},
			{"uppercase-entry", `,"` + strings.ToUpper(field) + `":{"key":null}` + canonical + valid, false},
			{"uppercase-null", canonical + valid + `,"` + strings.ToUpper(field) + `":null`, field != "enablement"},
			{"escaped-field", fmt.Sprintf(`,"\u%04x%s":{"key":null}`, field[0], field[1:]) + canonical + valid, false},
			{"escaped-entry", canonical + `{"k\u0065y":null,"key":` + value + `}`, false},
			{"wrong-value-kind", canonical + `{"key":42}`, false},
			{"wrong-map-kind", canonical + `[]`, false},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				mock := NewMockTransport()
				c := p.NewClient(mock)
				t.Cleanup(func() { _ = c.Close() })
				got, err := typedMapRead(t, c, mock, field, tc.members)
				if (err == nil) != tc.valid {
					t.Fatalf("valid=%v %s: %v", tc.valid, tc.members, err)
				}
				if !tc.valid {
					if !reflect.ValueOf(got).IsZero() {
						t.Fatalf("partial response published: %+v", got)
					}
					if _, err = typedMapRead(t, c, mock, field, canonical+valid); err != nil {
						t.Fatalf("same-client recovery: %v", err)
					}
					return
				}
				want := expectedTypedMap(t, field, tc.members)
				if !reflect.DeepEqual(actualTypedMap(got, field), want) {
					t.Fatalf("map selection changed: %#v vs %#v", actualTypedMap(got, field), want)
				}
			})
		}
	}
}

func TestTypedMapNotificationRecovery(t *testing.T) {
	for _, field := range []string{"labels", "iconAssets", "iconDarkAssets"} {
		t.Run(field, func(t *testing.T) {
			mock := NewMockTransport()
			on, add, reports := 0, 0, 0
			c := p.NewClient(mock, p.WithHandlerErrorCallback(func(origin string, _ error) {
				if origin != "app/list/updated" {
					t.Errorf("origin %s", origin)
				}
				reports++
			}))
			t.Cleanup(func() { _ = c.Close() })
			c.OnAppListUpdated(func(n p.AppListUpdatedNotification) {
				on++
				if value, ok := appMap(n.Data[1], field)["key"]; !ok || value != "" {
					t.Fatal("value changed")
				}
			})
			remove := c.AddAppListUpdatedListener(func(n p.AppListUpdatedNotification) {
				add++
				if value, ok := appMap(n.Data[1], field)["key"]; !ok || value != "" {
					t.Fatal("value changed")
				}
			})
			defer remove()
			for _, raw := range []string{`{"key":null}`, `{"key":null,"key":""}`} {
				body := json.RawMessage(`{"data":[{"id":"good","name":"Good"},{"id":"bad","name":"Bad","` + field + `":` + raw + `}]}`)
				before := reports
				mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: body})
				if on != 0 || add != 0 || reports <= before {
					t.Fatal("partial or invalid notification published")
				}
			}
			before := reports
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: json.RawMessage(`{"data":[{"id":"good","name":"Good"},{"id":"recovered","name":"Recovered","` + field + `":{"key":""}}]}`)})
			if on != 1 || add != 1 || reports != before {
				t.Fatal("valid notification recovery failed")
			}
		})
	}
}
