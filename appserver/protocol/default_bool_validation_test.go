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

type defaultBoolCase struct{ owner, field string }

var nativeDefaultBools = []defaultBoolCase{
	{"app", "isEnabled"}, {"app", "isAccessible"},
	{"model", "supportsPersonality"},
	{"config", "exclude_slash_tmp"}, {"config", "exclude_tmpdir_env_var"}, {"config", "network_access"},
	{"approval", "request_permissions"}, {"approval", "skill_approval"},
	{"sandbox", "excludeSlashTmp"}, {"sandbox", "excludeTmpdirEnvVar"}, {"sandbox", "networkAccess"},
}

func defaultBoolBody(owner, members string) string {
	switch owner {
	case "app":
		return `{"id":"app","name":"App"` + members + `}`
	case "model":
		return `{"id":"m","model":"m","displayName":"Model","description":"D","hidden":false,"isDefault":false,"defaultReasoningEffort":"medium","supportedReasoningEfforts":[]` + members + `}`
	case "config":
		return `{"sandbox_workspace_write":{` + strings.TrimPrefix(members, ",") + `}}`
	case "approval":
		return `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false` + members + `}}`
	case "sandbox":
		return `{"type":"workspaceWrite"` + members + `}`
	default:
		panic(owner)
	}
}
func defaultBoolTarget(owner string) any {
	switch owner {
	case "app":
		return &p.AppInfo{}
	case "model":
		return &p.Model{}
	case "config":
		return &p.Config{}
	case "approval":
		return &p.ApprovalPolicyGranular{}
	case "sandbox":
		return &p.SandboxPolicyWrapper{}
	default:
		panic(owner)
	}
}
func defaultBoolService(t *testing.T, owner, raw string) (any, error) {
	return defaultBoolCaller(t, owner)(raw)
}

func defaultBoolCaller(t *testing.T, owner string) func(string) (any, error) {
	t.Helper()
	mock := NewMockTransport()
	client := p.NewClient(mock)
	return func(raw string) (any, error) {
		switch owner {
		case "app":
			mock.SetResponse("app/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[` + raw + `]}`)})
			return client.Apps.List(context.Background(), p.AppsListParams{})
		case "model":
			mock.SetResponse("model/list", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"data":[` + raw + `]}`)})
			return client.Model.List(context.Background(), p.ModelListParams{})
		case "config":
			mock.SetResponse("config/read", p.Response{JSONRPC: "2.0", Result: json.RawMessage(`{"config":` + raw + `,"origins":{}}`)})
			return client.Config.Read(context.Background(), p.ConfigReadParams{})
		default:
			body := validThreadLifecycleResponse(validProcessThreadPayload("bool-thread"))
			field := "sandbox"
			if owner == "approval" {
				field = "approvalPolicy"
			}
			body[field] = json.RawMessage(raw)
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			mock.SetResponse("thread/start", p.Response{JSONRPC: "2.0", Result: encoded})
			return client.Thread.Start(context.Background(), p.ThreadStartParams{})
		}
	}
}

func TestNativeDefaultBooleansRejectNull(t *testing.T) {
	for _, tc := range nativeDefaultBools {
		t.Run(tc.owner+"/"+tc.field, func(t *testing.T) {
			raw := defaultBoolBody(tc.owner, fmt.Sprintf(`,%q:null`, strings.ToUpper(tc.field)))
			if err := json.Unmarshal([]byte(raw), defaultBoolTarget(tc.owner)); err == nil {
				t.Error("native alias null accepted")
			}
			response, err := defaultBoolService(t, tc.owner, raw)
			if err == nil || !reflect.ValueOf(response).IsZero() {
				t.Errorf("invalid null published by public service: %v %+v", err, response)
			}
		})
	}
}

func defaultBoolValue(owner, field string, target any) (bool, bool) {
	value := reflect.ValueOf(target)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	switch owner {
	case "config":
		value = value.FieldByName("SandboxWorkspaceWrite")
		if value.IsNil() {
			return false, false
		}
		value = value.Elem()
	case "approval":
		value = value.FieldByName("Granular")
	case "sandbox":
		value = reflect.ValueOf(target.(*p.SandboxPolicyWrapper).Value)
		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
	}
	for i := 0; i < value.NumField(); i++ {
		if strings.Split(value.Type().Field(i).Tag.Get("json"), ",")[0] != field {
			continue
		}
		member := value.Field(i)
		if member.Kind() == reflect.Pointer {
			if member.IsNil() {
				return false, false
			}
			member = member.Elem()
		}
		return true, member.Bool()
	}
	panic(field)
}

func defaultBoolPublicValue(owner, field string, response any) (bool, bool) {
	switch owner {
	case "app":
		value := response.(p.AppsListResponse).Data[0]
		return defaultBoolValue(owner, field, &value)
	case "model":
		value := response.(p.ModelListResponse).Data[0]
		return defaultBoolValue(owner, field, &value)
	case "config":
		return defaultBoolValue(owner, field, response.(p.ConfigReadResponse).Config)
	case "approval":
		value := response.(p.ThreadStartResponse).ApprovalPolicy.Value
		return defaultBoolValue(owner, field, value)
	case "sandbox":
		value := response.(p.ThreadStartResponse).Sandbox
		return defaultBoolValue(owner, field, &value)
	default:
		panic(owner)
	}
}

func TestNativeDefaultBooleansMatrix(t *testing.T) {
	for _, tc := range nativeDefaultBools {
		t.Run(tc.owner+"/"+tc.field, func(t *testing.T) {
			alias := strings.ToUpper(tc.field)
			escaped := fmt.Sprintf(`"\u%04x%s"`, tc.field[0], tc.field[1:])
			// Every fixture is generated from a valid owner body, so failures exercise
			// only null admission. The old native wire still owns wrong JSON kinds.
			for _, members := range []string{
				fmt.Sprintf(`,%q:null`, tc.field), fmt.Sprintf(`,%q:null`, alias), `,` + escaped + `:null`,
				fmt.Sprintf(`,%q:null,%q:true`, alias, tc.field), fmt.Sprintf(`,%q:false,%q:null`, tc.field, alias),
			} {
				raw := defaultBoolBody(tc.owner, members)
				if err := json.Unmarshal([]byte(raw), defaultBoolTarget(tc.owner)); !errors.Is(err, p.ErrNullResultField) {
					t.Fatalf("missing null error: %v JSON=%s", err, raw)
				}
				call := defaultBoolCaller(t, tc.owner)
				response, err := call(raw)
				if !errors.Is(err, p.ErrNullResultField) || !reflect.ValueOf(response).IsZero() {
					t.Fatalf("invalid response published: %v %+v", err, response)
				}
				response, err = call(defaultBoolBody(tc.owner, fmt.Sprintf(`,%q:false`, tc.field)))
				present, value := false, false
				if err == nil {
					present, value = defaultBoolPublicValue(tc.owner, tc.field, response)
				}
				if err != nil || !present || value {
					t.Fatalf("same-client false recovery failed: %v %+v", err, response)
				}
			}
			for _, tcValue := range []struct {
				members  string
				want     bool
				explicit bool
			}{
				{"", tc.owner == "app" && tc.field == "isEnabled", false},
				{fmt.Sprintf(`,%q:false`, tc.field), false, true},
				{fmt.Sprintf(`,%q:true`, alias), true, true},
				{fmt.Sprintf(`,%q:true,%q:false`, tc.field, alias), false, true},
				{fmt.Sprintf(`,%q:false,%q:true`, alias, tc.field), true, true},
			} {
				raw := defaultBoolBody(tc.owner, tcValue.members)
				target := defaultBoolTarget(tc.owner)
				if err := json.Unmarshal([]byte(raw), target); err != nil {
					t.Fatalf("valid native bool rejected: %v", err)
				}
				present, value := defaultBoolValue(tc.owner, tc.field, target)
				if value != tcValue.want || (tcValue.explicit && !present) {
					t.Fatal("default/explicit bool changed")
				}
				encoded, err := json.Marshal(target)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(encoded, target); err != nil {
					t.Fatalf("round trip failed: %v", err)
				}
				_, roundTrip := defaultBoolValue(tc.owner, tc.field, target)
				if roundTrip != tcValue.want {
					t.Fatal("round trip changed flag")
				}
				response, err := defaultBoolService(t, tc.owner, raw)
				if err != nil {
					t.Fatalf("valid public response rejected: %v", err)
				}
				present, value = defaultBoolPublicValue(tc.owner, tc.field, response)
				if value != tcValue.want || (tcValue.explicit && !present) {
					t.Fatal("public default/explicit bool changed")
				}
			}
			for _, bad := range []string{`1`, `"false"`, `[]`, `{}`} {
				raw := defaultBoolBody(tc.owner, fmt.Sprintf(`,%q:%s`, tc.field, bad))
				err := json.Unmarshal([]byte(raw), defaultBoolTarget(tc.owner))
				var mismatch *json.UnmarshalTypeError
				if !errors.As(err, &mismatch) {
					t.Fatalf("native wrong-kind error changed: %v", err)
				}
			}
		})
	}
}

func TestNativeDefaultBooleansUnicodeAndOuterAliases(t *testing.T) {
	for _, tc := range []struct{ owner, raw string }{
		{"app", defaultBoolBody("app", `,"i\u017fEnabled":null`)},
		{"model", defaultBoolBody("model", `,"\u017fupportsPersonality":null`)},
		{"config", `{"SANDBOX_WORKSPACE_WRITE":{"networ\u212a_access":null}}`},
		{"config", `{"sandbox_workspace_write":{"NETWORK_ACCESS":null},"SANDBOX_WORKSPACE_WRITE":{"network_access":true}}`},
		{"approval", `{"granular":{"mcp_elicitations":false,"rules":false,"sandbox_approval":false},"GRANULAR":{"reque\u017ft_permissions":null}}`},
		{"sandbox", defaultBoolBody("sandbox", `,"networ\u212aAccess":null`)},
	} {
		if err := json.Unmarshal([]byte(tc.raw), defaultBoolTarget(tc.owner)); !errors.Is(err, p.ErrNullResultField) {
			t.Fatalf("recognized alias null accepted: owner=%s err=%v", tc.owner, err)
		}
	}
	// Additional flags constrained only in workspaceWrite stay unrestricted extras
	// in the other sandbox alternatives and in preserved unknown policy variants.
	for _, raw := range []string{`{"type":"dangerFullAccess","networkAccess":null}`, `{"type":"readOnly","excludeSlashTmp":null}`, `{"type":"future","networkAccess":null}`} {
		var target p.SandboxPolicyWrapper
		if err := json.Unmarshal([]byte(raw), &target); err != nil {
			t.Fatalf("inactive/future branch narrowed: %v", err)
		}
	}
}

func TestNativeDefaultBooleanNotifications(t *testing.T) {
	for _, owner := range []string{"approval", "sandbox"} {
		mock := NewMockTransport()
		on, add := 0, 0
		reported := 0
		client := p.NewClient(mock, p.WithHandlerErrorCallback(func(method string, err error) {
			if method != "thread/settings/updated" || !errors.Is(err, p.ErrNullResultField) {
				t.Errorf("wrong notification error: %s %v", method, err)
			}
			reported++
		}))
		client.CacheThreadState(p.Thread{ID: "thread", DisabledPluginIDs: []string{"old"}})
		client.OnThreadSettingsUpdated(func(p.ThreadSettingsUpdatedNotification) { on++ })
		remove := client.AddThreadSettingsUpdatedListener(func(p.ThreadSettingsUpdatedNotification) { add++ })
		defer remove()
		settings := map[string]any{"approvalPolicy": "never", "approvalsReviewer": "user", "cwd": "/tmp", "model": "model", "modelProvider": "openai", "sandboxPolicy": map[string]any{"type": "readOnly"}, "collaborationMode": map[string]any{"mode": "default", "settings": map[string]any{"model": "model"}}}
		key, field := "sandboxPolicy", "NETWORKACCESS"
		if owner == "approval" {
			key, field = "approvalPolicy", "REQUEST_PERMISSIONS"
		}
		settings[key] = json.RawMessage(defaultBoolBody(owner, fmt.Sprintf(`,%q:null`, field)))
		encoded, err := json.Marshal(map[string]any{"threadId": "thread", "threadSettings": settings})
		if err != nil {
			t.Fatal(err)
		}
		mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: encoded})
		if on != 0 || add != 0 || reported < 2 {
			t.Fatalf("invalid policy notification published: owner=%s On=%d Add=%d errors=%d", owner, on, add, reported)
		}
		savedReports := reported
		snapshot, ok := client.ThreadStateSnapshot("thread")
		if !ok || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"old"}) {
			t.Fatal("invalid settings entered cache")
		}
		settings["disabledPluginIds"] = []string{"new"}
		settings[key] = json.RawMessage(defaultBoolBody(owner, fmt.Sprintf(`,%q:false`, field)))
		encoded, err = json.Marshal(map[string]any{"threadId": "thread", "threadSettings": settings})
		if err != nil {
			t.Fatal(err)
		}
		mock.InjectServerNotification(context.Background(), p.Notification{Method: "thread/settings/updated", Params: encoded})
		if on != 1 || add != 1 || reported != savedReports {
			t.Fatal("policy notification recovery failed")
		}
		snapshot, ok = client.ThreadStateSnapshot("thread")
		if !ok || !reflect.DeepEqual(snapshot.DisabledPluginIDs, []string{"new"}) {
			t.Fatal("valid settings cache recovery failed")
		}
	}
}

func TestNativeDefaultBooleanNoCachePublication(t *testing.T) {
	for _, owner := range []string{"approval", "sandbox"} {
		field := "networkAccess"
		if owner == "approval" {
			field = "request_permissions"
		}
		mock := NewMockTransport()
		client := p.NewClient(mock)
		calls := 0
		payload := validThreadLifecycleResponse(validProcessThreadPayload("bool-thread"))
		key := "sandbox"
		if owner == "approval" {
			key = "approvalPolicy"
		}
		payload[key] = json.RawMessage(defaultBoolBody(owner, fmt.Sprintf(`,%q:null`, field)))
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		mock.SetResponse("thread/start", p.Response{JSONRPC: "2.0", Result: raw})
		result, generation, cleanup, err := client.Thread.StartWithStateListener(context.Background(), p.ThreadStartParams{}, func(p.Thread) { calls++ }, nil)
		if !errors.Is(err, p.ErrNullResultField) || !reflect.ValueOf(result).IsZero() || generation != 0 || cleanup == nil || calls != 0 {
			t.Fatalf("failed startup published state: err=%v generation=%d calls=%d cleanup=%v", err, generation, calls, cleanup != nil)
		}
		cleanup()
		if _, ok := client.ThreadStateSnapshot("bool-thread"); ok {
			t.Fatal("invalid response entered thread cache")
		}
	}
}

func TestNativeDefaultBooleanConfigCompatibility(t *testing.T) {
	// Independent method-free native model preserves field tags, root name and
	// nested pointer/slice behavior. No workspace-restriction field is exercised.
	type Config p.Config
	for _, raw := range []string{
		`{}`, `{"sandbox_workspace_write":null}`,
		`{"sandbox_workspace_write":{"network_access":null,"exclude_slash_tmp":true},"model_provider":"changed"}`,
		`{"sandbox_workspace_write":{"NETWORK_ACCESS":null},"SANDBOX_WORKSPACE_WRITE":{"network_access":true},"model_provider":"changed"}`,
		`{"sandbox_workspace_write":{"network_access":42},"model_provider":"changed"}`,
		`{"sandbox_workspace_write":{"network_access":null},"model_provider":42}`,
	} {
		seed := func() *p.SandboxWorkspaceWrite {
			return &p.SandboxWorkspaceWrite{NetworkAccess: p.Ptr(true), WritableRoots: []string{"prior"}}
		}
		old, referenceOld := seed(), seed()
		actual := p.Config{SandboxWorkspaceWrite: old}
		reference := Config{SandboxWorkspaceWrite: referenceOld}
		got, want := json.Unmarshal([]byte(raw), &actual), json.Unmarshal([]byte(raw), &reference)
		if want != nil {
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("old native error changed: got=%v want=%v JSON=%s", got, want, raw)
			}
		} else if strings.Contains(raw, `:null`) && !strings.Contains(raw, `"sandbox_workspace_write":null`) {
			if !errors.Is(got, p.ErrNullResultField) {
				t.Fatalf("new null failure missing: %v", got)
			}
		} else if got != nil {
			t.Fatal(got)
		}
		if !reflect.DeepEqual(Config(actual), reference) || !reflect.DeepEqual(old, referenceOld) {
			t.Fatalf("native Config partial state changed: actual=%+v reference=%+v", actual, reference)
		}
		if actual.SandboxWorkspaceWrite != nil && actual.SandboxWorkspaceWrite != old {
			t.Fatal("Config replaced existing nested pointer")
		}
	}
	// Root Config arrays keep their existing precedence across all duplicate
	// workspace objects, even when an earlier object contains a new bool failure.
	var config p.Config
	raw := `{"sandbox_workspace_write":{"network_access":null},"SANDBOX_WORKSPACE_WRITE":{"writable_roots":[null]}}`
	if err := json.Unmarshal([]byte(raw), &config); err == nil || !strings.Contains(err.Error(), "index 0") {
		t.Fatalf("earlier array validation lost priority: %v", err)
	}
	type envelope struct {
		Config p.Config `json:"config"`
		After  string   `json:"after"`
	}
	var outer envelope
	if err := json.Unmarshal([]byte(`{"config":{"sandbox_workspace_write":{"network_access":null},"model_provider":"changed"},"after":"visited"}`), &outer); !errors.Is(err, p.ErrNullResultField) || outer.After != "visited" || outer.Config.ModelProvider == nil || *outer.Config.ModelProvider != "changed" {
		t.Fatalf("Config/enclosing partial traversal changed: %v %+v", err, outer)
	}
	// The plain public representation stays method-free, including anonymous
	// envelopes; strict admission belongs to Config, not the nested record.
	var plain struct {
		p.SandboxWorkspaceWrite
		After string `json:"after"`
	}
	if err := json.Unmarshal([]byte(`{"network_access":null,"after":"visited"}`), &plain); err != nil || plain.After != "visited" {
		t.Fatalf("plain representation acquired codec behavior: %v", err)
	}
	if _, ok := any(&p.SandboxWorkspaceWrite{}).(json.Unmarshaler); ok {
		t.Fatal("public representation acquired UnmarshalJSON")
	}
	if _, ok := any(p.SandboxWorkspaceWrite{}).(json.Marshaler); ok {
		t.Fatal("public representation acquired MarshalJSON")
	}
}

func TestNativeDefaultBooleanPriorErrorsAndReceivers(t *testing.T) {
	for _, tc := range []struct {
		owner, raw, want string
		native           bool
	}{
		{"app", `{"name":"App","ISENABLED":null}`, "id", false},
		{"app", defaultBoolBody("app", `,"ISENABLED":null,"pluginDisplayNames":[null]`), "pluginDisplayNames", false},
		{"model", defaultBoolBody("model", `,"supportsPersonality":null,"inputModalities":["future"]`), "inputModality", false},
		{"model", defaultBoolBody("model", `,"SUPPORTSPERSONALITY":null,"hidden":1`), "cannot unmarshal number", true},
		{"approval", `{"granular":{"rules":false,"sandbox_approval":false,"REQUEST_PERMISSIONS":null}}`, "mcp_elicitations", false},
		{"approval", defaultBoolBody("approval", `,"REQUEST_PERMISSIONS":null,"rules":1`), "cannot unmarshal number", true},
		{"sandbox", defaultBoolBody("sandbox", `,"networkAccess":null,"writableRoots":["relative"]`), "writableRoots", false},
		{"sandbox", defaultBoolBody("sandbox", `,"networkAccess":null,"excludeSlashTmp":1`), "cannot unmarshal number", true},
	} {
		err := json.Unmarshal([]byte(tc.raw), defaultBoolTarget(tc.owner))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("older owner error lost priority: owner=%s err=%v", tc.owner, err)
		}
		if tc.native {
			var mismatch *json.UnmarshalTypeError
			if !errors.As(err, &mismatch) {
				t.Fatalf("native error replaced: %v", err)
			}
		}
	}
	app := p.AppInfo{ID: "old", Name: "Old", IsEnabled: false, Description: p.Ptr("old")}
	appBefore := app
	if err := json.Unmarshal([]byte(defaultBoolBody("app", `,"ISENABLED":null`)), &app); !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(app, appBefore) || app.Description != appBefore.Description {
		t.Fatal("new app failure replaced receiver/reference")
	}
	model := p.Model{ID: "old", Upgrade: p.Ptr("old-upgrade")}
	modelBefore := model
	if err := json.Unmarshal([]byte(defaultBoolBody("model", `,"SUPPORTSPERSONALITY":null`)), &model); !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(model, modelBefore) || model.Upgrade != modelBefore.Upgrade {
		t.Fatal("new model failure replaced receiver/reference")
	}
	var approval p.ApprovalPolicyGranular
	approval.Granular.RequestPermissions = p.Ptr(true)
	approvalBefore := approval
	if err := json.Unmarshal([]byte(defaultBoolBody("approval", `,"REQUEST_PERMISSIONS":null`)), &approval); !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(approval, approvalBefore) || approval.Granular.RequestPermissions != approvalBefore.Granular.RequestPermissions {
		t.Fatal("new approval failure replaced receiver/reference")
	}
	roots := []string{"/old"}
	policy := p.SandboxPolicyWorkspaceWrite{NetworkAccess: p.Ptr(true), WritableRoots: roots}
	sandbox := p.SandboxPolicyWrapper{Value: policy}
	sandboxBefore := sandbox
	if err := json.Unmarshal([]byte(defaultBoolBody("sandbox", `,"NETWORKACCESS":null`)), &sandbox); !errors.Is(err, p.ErrNullResultField) || !reflect.DeepEqual(sandbox, sandboxBefore) {
		t.Fatal("new sandbox failure replaced receiver")
	}
	retained := sandbox.Value.(p.SandboxPolicyWorkspaceWrite)
	if retained.NetworkAccess != policy.NetworkAccess || &retained.WritableRoots[0] != &roots[0] {
		t.Fatal("new sandbox failure replaced references")
	}
}
