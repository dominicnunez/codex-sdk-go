package protocol_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

type stringArrayContractCase struct {
	name, field, body, validItem string
	nullable, aliases            bool
	newValue                     func() any
}

func TestStringArrayEstablishedErrors(t *testing.T) {
	for _, tc := range stringArrayContractCases() {
		t.Run(tc.name, func(t *testing.T) {
			base := strings.Replace(tc.body, "@VALUE@", "[42]", 1)
			mixed := strings.Replace(tc.body, "@VALUE@", `[42],"`+tc.field+`":[null]`, 1)
			reference, actual := tc.newValue(), tc.newValue()
			seed := strings.Replace(tc.body, "@VALUE@", "["+tc.validItem+"]", 1)
			for _, target := range []any{reference, actual} {
				if err := json.Unmarshal([]byte(seed), target); err != nil {
					t.Fatal(err)
				}
			}
			wantErr := json.Unmarshal([]byte(base), reference)
			var typeError *json.UnmarshalTypeError
			if !errors.As(wantErr, &typeError) {
				t.Fatalf("reference lacks native type error: %v", wantErr)
			}
			gotErr := json.Unmarshal([]byte(mixed), actual)
			if !reflect.DeepEqual(wantErr, gotErr) || !reflect.DeepEqual(reference, actual) {
				t.Fatalf("null guard changed established error or receiver: got %v want %v", gotErr, wantErr)
			}
		})
	}
}

func TestStringArrayNestedMergeAndReset(t *testing.T) {
	for _, body := range []string{
		`{"id":"a","name":"A","appMetadata":{"categories":["first"]},"appMetadata":{"subCategories":["second"]}}`,
		`{"id":"a","name":"A","appMetadata":{"categories":["first"]},"appMetadata":null,"appMetadata":{"subCategories":["second"]}}`,
	} {
		var app protocol.AppInfo
		if err := json.Unmarshal([]byte(body), &app); err != nil {
			t.Fatal(err)
		}
		if app.AppMetadata == nil || app.AppMetadata.SubCategories == nil || !reflect.DeepEqual(*app.AppMetadata.SubCategories, []string{"second"}) {
			t.Fatal("nested duplicate lost subcategories")
		}
		wantCategories := []string{"first"}
		if strings.Contains(body, `"appMetadata":null`) {
			wantCategories = nil
		}
		if (wantCategories == nil && app.AppMetadata.Categories != nil) || (wantCategories != nil && (app.AppMetadata.Categories == nil || !reflect.DeepEqual(*app.AppMetadata.Categories, wantCategories))) {
			t.Fatal("nested object merge/reset changed")
		}
	}
	var item protocol.ThreadItemWrapper
	if err := json.Unmarshal([]byte(`{"type":"agentMessage","id":"i","text":"","memoryCitation":{"entries":[],"threadIds":["first"]},"memoryCitation":{"threadIds":["second"]}}`), &item); err != nil {
		t.Fatal(err)
	}
	citation := item.Value.(*protocol.AgentMessageThreadItem).MemoryCitation
	if citation == nil || citation.Entries == nil || !reflect.DeepEqual(citation.ThreadIDs, []string{"second"}) {
		t.Fatal("citation duplicate object merge changed")
	}
}

func TestStringArrayPluginPathPrecedence(t *testing.T) {
	for _, pathField := range []string{"composerIcon", "logo", "logoDark", "screenshots"} {
		for _, arrayField := range []string{"capabilities", "screenshotUrls", "defaultPrompt"} {
			t.Run(pathField+"/"+arrayField, func(t *testing.T) {
				body := `{"brandColor":"new","capabilities":[],"screenshotUrls":[],"screenshots":[],"defaultPrompt":[],"` + pathField + `":"relative"}`
				if pathField == "screenshots" {
					body = strings.TrimSuffix(body, `"relative"}`) + `["relative"]}`
				}
				base, mixed := protocol.PluginInterface{}, protocol.PluginInterface{}
				seed := `{"brandColor":"prior","capabilities":[],"screenshotUrls":[],"screenshots":[]}`
				for _, target := range []*protocol.PluginInterface{&base, &mixed} {
					if err := json.Unmarshal([]byte(seed), target); err != nil {
						t.Fatal(err)
					}
				}
				wantErr := json.Unmarshal([]byte(body), &base)
				if wantErr == nil {
					t.Fatal("invalid path fixture admitted")
				}
				// An earlier invalid occurrence remains hidden from the final
				// wire value. The established final path failure still owns it.
				invalid := `"` + arrayField + `":[null],"` + arrayField + `":[],`
				gotErr := json.Unmarshal([]byte("{"+invalid+body[1:]), &mixed)
				if !reflect.DeepEqual(wantErr, gotErr) || !reflect.DeepEqual(base, mixed) {
					t.Fatalf("path failure or partial receiver changed: got %v want %v", gotErr, wantErr)
				}
			})
		}
	}
}

func TestStringArrayPlainApplicationEnvelopes(t *testing.T) {
	for _, target := range []any{
		&struct {
			Before string
			protocol.AppMetadata
			After string
		}{},
		&struct {
			Before string
			protocol.MemoryCitation
			After string
		}{},
		&struct {
			Before string
			protocol.CollabAgentToolCallThreadItem
			After string
		}{},
		&struct {
			Before string
			protocol.SearchWebSearchAction
			After string
		}{},
	} {
		t.Run(reflect.TypeOf(target).String(), func(t *testing.T) {
			if err := json.Unmarshal([]byte(`{"Before":"b","After":"a","categories":[null],"threadIds":[null],"receiverThreadIds":[null],"queries":[null]}`), target); err != nil {
				t.Fatalf("plain application representation gained carrier admission: %v", err)
			}
			value := reflect.ValueOf(target).Elem()
			if value.FieldByName("Before").String() != "b" || value.FieldByName("After").String() != "a" {
				t.Fatal("embedded representation consumed application envelope fields")
			}
		})
	}
}

func TestStringArraySiblingPathPrecedence(t *testing.T) {
	detail := strings.Replace(stringArrayContractCases()[9].body, "@VALUE@", "[]", 1)
	detail = strings.TrimSuffix(detail, "}") + `,"marketplacePath":"relative"}`
	for _, tc := range []struct {
		name, body, array string
		newValue          func() any
	}{
		{"marketplace", `{"errors":[],"selectedMarketplaces":[],"upgradedRoots":["relative"]}`, "selectedMarketplaces", func() any { return new(protocol.MarketplaceUpgradeResponse) }},
		{"plugin detail", detail, "mcpServers", func() any { return new(protocol.PluginDetail) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, mixed := tc.newValue(), tc.newValue()
			seed := strings.ReplaceAll(tc.body, `"relative"`, `"/tmp"`)
			for _, target := range []any{base, mixed} {
				if err := json.Unmarshal([]byte(seed), target); err != nil {
					t.Fatal(err)
				}
				if detail, ok := target.(*protocol.PluginDetail); ok {
					detail.MarketplaceName = "prior"
				}
			}
			wantErr := json.Unmarshal([]byte(tc.body), base)
			if wantErr == nil {
				t.Fatal("invalid path admitted")
			}
			invalid := `"` + tc.array + `":[null],`
			gotErr := json.Unmarshal([]byte("{"+invalid+tc.body[1:]), mixed)
			if !reflect.DeepEqual(wantErr, gotErr) || !reflect.DeepEqual(base, mixed) {
				t.Fatalf("sibling path error or receiver changed: got %v want %v", gotErr, wantErr)
			}
			if detail, ok := mixed.(*protocol.PluginDetail); ok && detail.MarketplaceName != "M" {
				t.Fatal("existing path failure lost its partial receiver update")
			}
		})
	}
}

func stringArrayContractCases() []stringArrayContractCase {
	iface := func(field string) string {
		body := `{"capabilities":[],"screenshotUrls":[],"screenshots":[]}`
		if field == "defaultPrompt" {
			return strings.TrimSuffix(body, "}") + `,"defaultPrompt":@VALUE@}`
		}
		return strings.Replace(body, `"`+field+`":[]`, `"`+field+`":@VALUE@`, 1)
	}
	return []stringArrayContractCase{
		{"loaded", "data", `{"data":@VALUE@}`, `""`, false, true, func() any { return new(protocol.ThreadLoadedListResponse) }},
		{"hooks", "warnings", `{"cwd":"relative","errors":[],"hooks":[],"warnings":@VALUE@}`, `""`, false, false, func() any { return new(protocol.HooksListEntry) }},
		{"marketplace selection", "selectedMarketplaces", `{"errors":[],"selectedMarketplaces":@VALUE@,"upgradedRoots":[]}`, `""`, false, true, func() any { return new(protocol.MarketplaceUpgradeResponse) }},
		{"marketplace roots", "upgradedRoots", `{"errors":[],"selectedMarketplaces":[],"upgradedRoots":@VALUE@}`, `"/tmp"`, false, true, func() any { return new(protocol.MarketplaceUpgradeResponse) }},
		{"reconcile materialization", "failedMaterializationRemotePluginIds", `{"changedPlugins":[],"failedMaterializationRemotePluginIds":@VALUE@,"failedRemotePluginIds":[]}`, `""`, false, true, func() any { return new(protocol.PluginReconcileResponse) }},
		{"reconcile remote", "failedRemotePluginIds", `{"changedPlugins":[],"failedMaterializationRemotePluginIds":[],"failedRemotePluginIds":@VALUE@}`, `""`, false, true, func() any { return new(protocol.PluginReconcileResponse) }},
		{"capabilities", "capabilities", iface("capabilities"), `""`, false, true, func() any { return new(protocol.PluginInterface) }},
		{"screenshot URLs", "screenshotUrls", iface("screenshotUrls"), `""`, false, true, func() any { return new(protocol.PluginInterface) }},
		{"screenshots", "screenshots", iface("screenshots"), `"/tmp"`, false, true, func() any { return new(protocol.PluginInterface) }},
		{"MCP server names", "mcpServers", `{"apps":[],"appTemplates":[],"hooks":[],"marketplaceName":"M","mcpServers":@VALUE@,"skills":[],"summary":` + issue74PluginSummary + `}`, `""`, false, true, func() any { return new(protocol.PluginDetail) }},
		{"citation threads", "threadIds", `{"type":"agentMessage","id":"i","text":"","memoryCitation":{"entries":[],"threadIds":@VALUE@}}`, `""`, false, true, func() any { return new(protocol.ThreadItemWrapper) }},
		{"collab receivers", "receiverThreadIds", `{"type":"collabAgentToolCall","id":"i","agentsStates":{},"receiverThreadIds":@VALUE@,"senderThreadId":"t","status":"completed","tool":"spawnAgent"}`, `""`, false, true, func() any { return new(protocol.ThreadItemWrapper) }},
		{"Windows paths", "samplePaths", `{"extraCount":0,"failedScan":false,"samplePaths":@VALUE@}`, `""`, false, false, func() any { return new(protocol.WindowsWorldWritableWarningNotification) }},
		{"verification", "verifications", `{"threadId":"t","turnId":"u","verifications":@VALUE@}`, `"trustedAccessForCyber"`, false, false, func() any { return new(protocol.ModelVerificationNotification) }},
		{"filesystem paths", "changedPaths", `{"watchId":"w","changedPaths":@VALUE@}`, `"/tmp"`, false, false, func() any { return new(protocol.FsChangedNotification) }},
		{"default prompt", "defaultPrompt", iface("defaultPrompt"), `""`, true, true, func() any { return new(protocol.PluginInterface) }},
		{"question options", "options", `{"title":"q","options":@VALUE@}`, `""`, true, false, func() any { return new(protocol.AsyncUserInputQuestion) }},
		{"app categories", "categories", `{"id":"a","name":"A","appMetadata":{"categories":@VALUE@}}`, `""`, true, true, func() any { return new(protocol.AppInfo) }},
		{"app subcategories", "subCategories", `{"id":"a","name":"A","appMetadata":{"subCategories":@VALUE@}}`, `""`, true, true, func() any { return new(protocol.AppInfo) }},
		{"web queries", "queries", `{"type":"search","queries":@VALUE@}`, `""`, true, true, func() any { return new(protocol.WebSearchActionWrapper) }},
	}
}

func TestStringArrayOwnerContract(t *testing.T) {
	for _, tc := range stringArrayContractCases() {
		t.Run(tc.name, func(t *testing.T) {
			valid := "[" + tc.validItem + "]"
			for _, value := range []string{"[]", valid, valid + `,"` + tc.field + `":[]`} {
				if err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", value, 1)), tc.newValue()); err != nil {
					t.Fatalf("valid occurrence %s: %v", value, err)
				}
			}
			for _, value := range []string{`[null]`, `[null],"` + tc.field + `":[]`, valid + `,"` + tc.field + `":[null]`, `[null],"` + tc.field + `":null`} {
				target := tc.newValue()
				if err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", valid, 1)), target); err != nil {
					t.Fatal(err)
				}
				before, err := json.Marshal(target)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", value, 1)), target); err == nil {
					t.Fatalf("invalid occurrence %s admitted", value)
				}
				after, err := json.Marshal(target)
				if err != nil || string(before) != string(after) {
					t.Fatalf("null-item failure changed prior receiver: %v", err)
				}
				if err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", "[]", 1)), target); err != nil {
					t.Fatalf("recovery: %v", err)
				}
			}
			for _, value := range []string{`null`, `null,"` + tc.field + `":[]`, valid + `,"` + tc.field + `":null`} {
				err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", value, 1)), tc.newValue())
				if (err == nil) != tc.nullable {
					t.Fatalf("whole-null %s nullable=%t: %v", value, tc.nullable, err)
				}
			}
			aliasValue := valid + `,"` + strings.ToUpper(tc.field) + `":[null]`
			err := json.Unmarshal([]byte(strings.Replace(tc.body, "@VALUE@", aliasValue, 1)), tc.newValue())
			if (err != nil) != tc.aliases {
				t.Fatalf("alias recognition mismatch: %v", err)
			}
			escapedName := fmt.Sprintf(`\u%04x%s`, tc.field[0], tc.field[1:])
			escapedBody := strings.Replace(tc.body, `"`+tc.field+`":@VALUE@`, `"`+escapedName+`":@VALUE@`, 1)
			if err := json.Unmarshal([]byte(strings.Replace(escapedBody, "@VALUE@", valid, 1)), tc.newValue()); err != nil {
				t.Fatalf("escaped canonical valid value: %v", err)
			}
			if err := json.Unmarshal([]byte(strings.Replace(escapedBody, "@VALUE@", `[null]`, 1)), tc.newValue()); err == nil {
				t.Fatal("escaped canonical null item admitted")
			}
		})
	}
}
