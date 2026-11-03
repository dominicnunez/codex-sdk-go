package protocol_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	p "github.com/dominicnunez/codex-sdk-go/appserver/protocol"
)

func callSkillIconResponse(t *testing.T, route, field string, path any) (*p.SkillInterface, error) {
	t.Helper()
	mock := NewMockTransport()
	c := p.NewClient(mock)
	t.Cleanup(func() { _ = c.Close() })
	face := map[string]any{field: path, "iconSmallUrl": "relative-url-text", "iconLargeUrl": "opaque://asset"}
	skill := map[string]any{"name": "skill", "description": "description", "enabled": true, "interface": face, "path": "relative/SKILL.md"}
	if route == "skills/list" {
		skill["path"] = "relative/SKILL.md"
		skill["scope"] = "repo"
		if err := mock.SetResponseData(route, map[string]any{"data": []any{map[string]any{"cwd": "relative/group", "errors": []any{}, "skills": []any{skill}}}}); err != nil {
			t.Fatal(err)
		}
		got, err := c.Skills.List(context.Background(), p.SkillsListParams{})
		if err != nil {
			if !reflect.DeepEqual(got, p.SkillsListResponse{}) {
				t.Fatal("failed skill response published metadata")
			}
			return nil, err
		}
		if got.Data[0].Cwd != "relative/group" || got.Data[0].Skills[0].Path != "relative/SKILL.md" {
			t.Fatal("opaque sibling paths changed")
		}
		return got.Data[0].Skills[0].Interface, nil
	}
	detail := issue74PluginDetail()
	detail["skills"] = []any{skill}
	if route == "plugin/onboarding" {
		detail["skills"] = []any{}
		detail["onboardingSkill"] = skill
	}
	if err := mock.SetResponseData("plugin/read", map[string]any{"plugin": detail}); err != nil {
		t.Fatal(err)
	}
	got, err := c.Plugin.Read(context.Background(), p.PluginReadParams{PluginName: "plugin", RemoteMarketplaceName: p.Ptr("marketplace")})
	if err != nil {
		if !reflect.DeepEqual(got, p.PluginReadResponse{}) {
			t.Fatal("failed plugin response published metadata")
		}
		return nil, err
	}
	if route == "plugin/onboarding" {
		if got.Plugin.OnboardingSkill == nil || got.Plugin.OnboardingSkill.Path == nil || *got.Plugin.OnboardingSkill.Path != "relative/SKILL.md" {
			t.Fatal("opaque onboarding skill path changed")
		}
		return got.Plugin.OnboardingSkill.Interface, nil
	}
	if got.Plugin.Skills[0].Path == nil || *got.Plugin.Skills[0].Path != "relative/SKILL.md" {
		t.Fatal("opaque plugin skill path changed")
	}
	return got.Plugin.Skills[0].Interface, nil
}

func TestSkillIconPathAdmission(t *testing.T) {
	for _, route := range []string{"skills/list", "plugin/read", "plugin/onboarding"} {
		for _, field := range []string{"iconSmall", "iconLarge"} {
			for _, tc := range []struct {
				name  string
				path  any
				valid bool
			}{
				{"null", nil, true}, {"absolute", "/tmp/icon.svg", true}, {"nonexistent", "/does-not-exist/icon.svg", true},
				{"root", "/", true}, {"drive", `C:\icons\small.svg`, true}, {"share", `\\server\share\small.svg`, true},
				{"empty", "", false}, {"relative", "relative", false}, {"parent", "/tmp/../icon.svg", false},
				{"dot", "/tmp/./icon.svg", false}, {"separators", "/tmp//icon.svg", false}, {"nul", "\x00", false},
			} {
				t.Run(route+"/"+field+"/"+tc.name, func(t *testing.T) {
					got, err := callSkillIconResponse(t, route, field, tc.path)
					if !tc.valid {
						if err == nil || !strings.Contains(err.Error(), field) || got != nil {
							t.Errorf("invalid absolute icon path published or context lost: %q err=%v", tc.path, err)
						}
						return
					}
					if err != nil || got == nil {
						t.Fatalf("valid icon metadata rejected: %v", err)
					}
					ptr := got.IconSmall
					if field == "iconLarge" {
						ptr = got.IconLarge
					}
					if tc.path == nil {
						if ptr != nil {
							t.Fatal("null became nonnull")
						}
					} else if ptr == nil || *ptr != tc.path.(string) {
						t.Fatal("path changed")
					}
					if got.IconSmallURL == nil || *got.IconSmallURL != "relative-url-text" || got.IconLargeURL == nil || *got.IconLargeURL != "opaque://asset" {
						t.Fatal("opaque URL strings changed")
					}
				})
			}
		}
	}
}

func TestSkillIconSelectionAndReceiver(t *testing.T) {
	for _, owner := range []string{"metadata", "summary"} {
		for _, tc := range []struct{ name, fields, small, large, wantErr string }{
			{name: "absent"},
			{name: "null interface", fields: `,"interface":null`},
			{name: "empty interface", fields: `,"interface":{}`},
			{name: "null icons", fields: `,"interface":{"iconSmall":null,"iconLarge":null}`},
			{name: "alias escaped", fields: `,"INTERFACE":{"ICONSMALL":"/fixture/small","icon\u004carge":"/fixture/large","future":{"arbitrary":true}}`, small: "/fixture/small", large: "/fixture/large"},
			{name: "selected valid", fields: `,"interface":{"iconSmall":"relative","iconSmall":"/fixture/small"}`, small: "/fixture/small"},
			{name: "selected null", fields: `,"interface":{"iconSmall":"relative","iconSmall":null}`},
			{name: "merged interface", fields: `,"interface":{"iconSmall":"/fixture/small"},"interface":{"iconLarge":"/fixture/large"}`, small: "/fixture/small", large: "/fixture/large"},
			{name: "reset interface", fields: `,"interface":{"iconSmall":"relative"},"interface":null`},
			{name: "invalid small", fields: `,"interface":{"iconSmall":"relative","iconLarge":"/fixture/large"}`, wantErr: "iconSmall"},
			{name: "invalid large", fields: `,"interface":{"iconSmall":"/fixture/small","iconLarge":"relative"}`, wantErr: "iconLarge"},
			{name: "selected invalid", fields: `,"interface":{"iconSmall":"/fixture/small","ICONSMALL":"relative"}`, wantErr: "iconSmall"},
		} {
			t.Run(owner+"/"+tc.name, func(t *testing.T) {
				seedFace := &p.SkillInterface{IconSmall: p.Ptr("/old/icon")}
				var dst any
				var body string
				if owner == "metadata" {
					dst = &p.SkillMetadata{Name: "old", Path: "old", Interface: seedFace}
					body = `{"name":"new","description":"new","enabled":true,"path":"relative/SKILL.md","scope":"repo"` + tc.fields + `}`
				} else {
					dst = &p.SkillSummary{Name: "old", Path: p.Ptr("old"), Interface: seedFace}
					body = `{"name":"new","description":"new","enabled":true` + tc.fields + `}`
				}
				before, _ := json.Marshal(dst)
				err := json.Unmarshal([]byte(body), dst)
				if tc.wantErr != "" {
					after, _ := json.Marshal(dst)
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) || string(before) != string(after) || *seedFace.IconSmall != "/old/icon" {
						t.Fatalf("new failure changed receiver: %s -> %s err=%v", before, after, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var face *p.SkillInterface
				switch v := dst.(type) {
				case *p.SkillMetadata:
					face = v.Interface
				case *p.SkillSummary:
					face = v.Interface
				}
				var small, large string
				if face != nil {
					if face.IconSmall != nil {
						small = *face.IconSmall
					}
					if face.IconLarge != nil {
						large = *face.IconLarge
					}
				}
				if small != tc.small || large != tc.large {
					t.Fatalf("selected icons changed: %q %q", small, large)
				}
			})
		}
	}
}

func TestSkillIconPriorErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		summary          bool
	}{
		{"metadata required", `{"enabled":true,"name":"new","path":"relative","scope":"repo","interface":{"iconSmall":"relative"}}`, "description", false},
		{"metadata kind", `{"description":"new","enabled":true,"name":"new","path":"relative","scope":"repo","interface":{"iconSmall":"relative","displayName":42}}`, "displayName", false},
		{"metadata scope", `{"description":"new","enabled":true,"name":"new","path":"relative","scope":"future","interface":{"iconSmall":"relative"}}`, "scope", false},
		{"summary required", `{"enabled":true,"name":"new","interface":{"iconSmall":"relative"}}`, "missing plugin.skill.description", true},
		{"summary prior null", `{"description":null,"description":"new","enabled":true,"name":"new","interface":{"iconSmall":"relative"}}`, "null", true},
		{"summary kind", `{"description":"new","enabled":true,"name":"new","interface":{"iconSmall":"relative","displayName":42}}`, "displayName", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var dst any = &p.SkillMetadata{Name: "old"}
			if tc.summary {
				dst = &p.SkillSummary{Name: "old"}
			}
			before, _ := json.Marshal(dst)
			err := json.Unmarshal([]byte(tc.body), dst)
			after, _ := json.Marshal(dst)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "iconSmall:") || string(before) != string(after) {
				t.Fatalf("old check priority/receiver changed: %v %s -> %s", err, before, after)
			}
		})
	}
}

func TestSkillIconDiagnosticAndPlainRecord(t *testing.T) {
	for _, route := range []string{"skills/list", "plugin/read", "plugin/onboarding"} {
		for _, field := range []string{"iconSmall", "iconLarge"} {
			_, err := callSkillIconResponse(t, route, field, "relative/"+strings.Repeat("peer-", 20000))
			if err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), field) {
				t.Fatal("unbounded or missing icon diagnostic")
			}
		}
	}
	for _, v := range []any{p.SkillInterface{}, &p.SkillInterface{}} {
		if _, ok := v.(json.Unmarshaler); ok {
			t.Fatal("public icon record gained decoder")
		}
		if _, ok := v.(json.Marshaler); ok {
			t.Fatal("public icon record gained encoder")
		}
	}
	var anonymous struct {
		p.SkillInterface
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"iconSmall":"relative","Extra":"kept"}`), &anonymous); err != nil || anonymous.IconSmall == nil || *anonymous.IconSmall != "relative" || anonymous.Extra != "kept" {
		t.Fatal("plain anonymous record changed")
	}
	var named struct {
		Face  p.SkillInterface
		Extra string
	}
	if err := json.Unmarshal([]byte(`{"Face":{"iconLarge":"relative"},"Extra":"kept"}`), &named); err != nil || named.Face.IconLarge == nil || *named.Face.IconLarge != "relative" || named.Extra != "kept" {
		t.Fatal("plain named record changed")
	}
}
