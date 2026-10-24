package protocol_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func TestAppDefaultFlagsRejectNull(t *testing.T) {
	for _, field := range []string{"isAccessible", "isEnabled"} {
		t.Run(field, func(t *testing.T) {
			app := fmt.Sprintf(`{"id":"app","name":"App",%q:null}`, field)
			var info p.AppInfo
			if err := json.Unmarshal([]byte(app), &info); err == nil {
				t.Error("null flag accepted as default")
			}
			mock := NewMockTransport()
			client := p.NewClient(mock)
			mock.SetResponse("app/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[` + app + `]}`)})
			result, err := client.Apps.List(context.Background(), p.AppsListParams{})
			if err == nil || !reflect.DeepEqual(result, p.AppsListResponse{}) {
				t.Errorf("null flag published as app listing: %v %+v", err, result)
			}
			primary, additional := 0, 0
			client.OnAppListUpdated(func(p.AppListUpdatedNotification) { primary++ })
			remove := client.AddAppListUpdatedListener(func(p.AppListUpdatedNotification) { additional++ })
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: json.RawMessage(`{"data":[` + app + `]}`)})
			if primary != 0 || additional != 0 {
				t.Errorf("null flag delivered to listeners: On=%d Add=%d", primary, additional)
			}
		})
	}
}

func TestAppDefaultFlagsValues(t *testing.T) {
	for _, tc := range []struct {
		members             string
		accessible, enabled bool
	}{
		{``, false, true},
		{`,"isAccessible":false,"isEnabled":false`, false, false},
		{`,"isAccessible":true,"isEnabled":true`, true, true},
		{`,"isAccessible":true,"isAccessible":false,"isEnabled":false,"isEnabled":true`, false, true},
		{`,"isAccessible":false,"isAccessible":true,"isEnabled":true,"isEnabled":false`, true, false},
		{`,"\u0069sAccessible":true,"\u0069sEnabled":false`, true, false},
		{`,"ISACCESSIBLE":true,"ISENABLED":false`, true, false},
		{`,"isEnabled":false,"isAccessible":true,"description":null,"appMetadata":null,"branding":null`, true, false},
	} {
		t.Run(tc.members, func(t *testing.T) {
			app := `{"id":"app","name":"App"` + tc.members + `}`
			check := func(info p.AppInfo) {
				t.Helper()
				if info.IsAccessible != tc.accessible || info.IsEnabled != tc.enabled {
					t.Fatalf("defaults or flag selection changed: %+v", info)
				}
			}
			var info p.AppInfo
			if err := json.Unmarshal([]byte(app), &info); err != nil {
				t.Fatal(err)
			}
			check(info)
			encoded, err := json.Marshal(info)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &info); err != nil {
				t.Fatal(err)
			}
			check(info)
			mock := NewMockTransport()
			client := p.NewClient(mock)
			raw := json.RawMessage(`{"data":[` + app + `]}`)
			mock.SetResponse("app/list", p.Response{JSONRPC: "2.0", Result: raw})
			result, err := client.Apps.List(context.Background(), p.AppsListParams{})
			if err != nil || len(result.Data) != 1 {
				t.Fatalf("valid app listing failed: %v", err)
			}
			check(result.Data[0])
			on, add := 0, 0
			client.OnAppListUpdated(func(n p.AppListUpdatedNotification) {
				on++
				if len(n.Data) != 1 {
					t.Fatal("On data missing")
				}
				check(n.Data[0])
			})
			remove := client.AddAppListUpdatedListener(func(n p.AppListUpdatedNotification) {
				add++
				if len(n.Data) != 1 {
					t.Fatal("Add data missing")
				}
				check(n.Data[0])
			})
			defer remove()
			mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: raw})
			if on != 1 || add != 1 {
				t.Fatalf("valid callbacks missing: On=%d Add=%d", on, add)
			}
		})
	}
}

func TestAppDefaultFlagsFailures(t *testing.T) {
	for _, field := range []string{"isAccessible", "isEnabled"} {
		for _, members := range []string{
			fmt.Sprintf(`%q:null`, strings.ToUpper(field)),
			fmt.Sprintf(`%q:false,%q:null`, field, strings.ToUpper(field)),
			fmt.Sprintf(`%q:null`, field),
			fmt.Sprintf(`%q:null,%q:false`, field, field),
			fmt.Sprintf(`%q:true,%q:null`, field, field),
			fmt.Sprintf(`"\u0069%s":null,%q:true`, field[1:], field),
		} {
			t.Run(field+"/"+members, func(t *testing.T) {
				app := `{"id":"bad","name":"Bad",` + members + `}`
				good := `{"id":"good","name":"Good","isEnabled":false,"isAccessible":true}`
				for _, prefix := range []string{"", good + ","} {
					mock := NewMockTransport()
					on, add := 0, 0
					var reported []error
					client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, err error) {
						if method != "app/list/updated" {
							t.Errorf("wrong diagnostic origin: %s", method)
						}
						reported = append(reported, err)
					}))
					client.OnAppListUpdated(func(p.AppListUpdatedNotification) { on++ })
					remove := client.AddAppListUpdatedListener(func(p.AppListUpdatedNotification) { add++ })
					defer remove()
					raw := json.RawMessage(`{"data":[` + prefix + app + `],"nextCursor":"unpublished"}`)
					mock.SetResponse("app/list", p.Response{JSONRPC: "2.0", Result: raw})
					result, err := client.Apps.List(context.Background(), p.AppsListParams{})
					if !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(result, p.AppsListResponse{}) {
						t.Fatalf("invalid listing published: %v %+v", err, result)
					}
					mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: raw})
					if on != 0 || add != 0 || len(reported) != 2 {
						t.Fatalf("failed typed notification admission: On=%d Add=%d errors=%d", on, add, len(reported))
					}
					for _, err := range reported {
						if !errors.Is(err, p.ErrNullResultField) {
							t.Fatalf("wrong notification diagnostic: %v", err)
						}
					}
					valid := json.RawMessage(`{"data":[` + good + `]}`)
					mock.SetResponse("app/list", p.Response{JSONRPC: "2.0", Result: valid})
					result, err = client.Apps.List(context.Background(), p.AppsListParams{})
					if err != nil || len(result.Data) != 1 || result.Data[0].IsEnabled || !result.Data[0].IsAccessible {
						t.Fatalf("list recovery failed: %v %+v", err, result)
					}
					mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: valid})
					if on != 1 || add != 1 || len(reported) != 2 {
						t.Fatal("notification recovery failed")
					}
					remove()
					mock.InjectServerNotification(context.Background(), p.Notification{Method: "app/list/updated", Params: valid})
					if on != 2 || add != 1 {
						t.Fatal("listener removal failed after rejected notification")
					}
				}
			})
		}
	}
}

func TestAppDefaultFlagsReceiverAndErrors(t *testing.T) {
	var info p.AppInfo
	seed := `{"id":"old","name":"Old","description":"old-description","labels":{"old":"value"},"pluginDisplayNames":["old-plugin"],"isEnabled":false,"isAccessible":true}`
	if err := json.Unmarshal([]byte(seed), &info); err != nil {
		t.Fatal(err)
	}
	before := info
	for _, bad := range []string{
		`{"id":"new","name":"New","isEnabled":null}`,
		`{"id":"new","name":"New","isAccessible":null}`,
	} {
		if err := json.Unmarshal([]byte(bad), &info); !errors.Is(err, p.ErrNullResultField) {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(info, before) || info.Description != before.Description || &info.PluginDisplayNames[0] != &before.PluginDisplayNames[0] {
			t.Fatal("failure replaced receiver or references")
		}
		info.Labels["probe"] = "same map"
		if before.Labels["probe"] != "same map" {
			t.Fatal("failure replaced map")
		}
		var envelope struct {
			App   p.AppInfo `json:"app"`
			After string    `json:"after"`
		}
		envelope.App = before
		if err := json.Unmarshal([]byte(`{"app":`+bad+`,"after":"visited"}`), &envelope); !errors.Is(err, p.ErrNullResultField) || envelope.After != "visited" || envelope.App.Description != before.Description {
			t.Fatalf("enclosing traversal changed: %v", err)
		}
		listing := p.AppsListResponse{Data: []p.AppInfo{before}, NextCursor: p.Ptr("old-cursor")}
		saved := listing
		if err := json.Unmarshal([]byte(`{"data":[`+bad+`]}`), &listing); !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(listing, saved) || &listing.Data[0] != &saved.Data[0] || listing.NextCursor != saved.NextCursor {
			t.Fatalf("direct response receiver changed: %v", err)
		}
		notification := p.AppListUpdatedNotification{Data: []p.AppInfo{before}}
		savedNotification := notification
		if err := json.Unmarshal([]byte(`{"data":[`+bad+`]}`), &notification); !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(notification, savedNotification) || &notification.Data[0] != &savedNotification.Data[0] {
			t.Fatalf("direct notification receiver changed: %v", err)
		}
	}
	for _, tc := range []struct{ raw, want string }{
		{`{"name":"New","isEnabled":null}`, `"id"`},
		{`{"id":1,"name":"New","isEnabled":null}`, "cannot unmarshal number"},
		{`{"id":"new","name":"New","isEnabled":"false"}`, "cannot unmarshal string"},
		{`{"id":"new","name":"New","isAccessible":{}}`, "cannot unmarshal object"},
		{`{"id":"new","name":"New","isEnabled":null,"pluginDisplayNames":[null]}`, "pluginDisplayNames"},
		{`{"id":"new","name":"New","isEnabled":null,"appMetadata":{"categories":[null]}}`, "categories"},
	} {
		err := json.Unmarshal([]byte(tc.raw), &info)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("earlier failure lost precedence: %v", err)
		}
	}
	if err := json.Unmarshal([]byte(`{"id":"new","name":"New"}`), &info); err != nil || !info.IsEnabled || info.IsAccessible || info.Description != nil || info.Labels != nil || len(info.PluginDisplayNames) != 0 {
		t.Fatalf("successful reuse retained old state or lost defaults: %v %+v", err, info)
	}
	if *before.Description != "old-description" || before.PluginDisplayNames[0] != "old-plugin" || before.Labels["old"] != "value" {
		t.Fatal("successful replacement mutated prior references")
	}
}
