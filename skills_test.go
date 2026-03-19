package codex_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestSkillsList(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.SkillsListParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.SkillsListResponse)
	}{
		{
			name:   "minimal list - default cwd",
			params: codex.SkillsListParams{},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"cwd":    "/home/user/project",
						"errors": []interface{}{},
						"skills": []interface{}{
							map[string]interface{}{
								"name":        "example-skill",
								"description": "An example skill",
								"path":        "/home/user/project/.claude/skills/example-skill",
								"enabled":     true,
								"scope":       "repo",
							},
						},
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.SkillsListResponse) {
				if len(resp.Data) != 1 {
					t.Errorf("expected 1 entry, got %d", len(resp.Data))
				}
				entry := resp.Data[0]
				if entry.Cwd != "/home/user/project" {
					t.Errorf("expected cwd = /home/user/project, got %s", entry.Cwd)
				}
				if len(entry.Skills) != 1 {
					t.Errorf("expected 1 skill, got %d", len(entry.Skills))
				}
				skill := entry.Skills[0]
				if skill.Name != "example-skill" {
					t.Errorf("expected name = example-skill, got %s", skill.Name)
				}
				if skill.Scope != "repo" {
					t.Errorf("expected scope = repo, got %s", skill.Scope)
				}
				if skill.Enabled != true {
					t.Errorf("expected enabled = true, got %v", skill.Enabled)
				}
			},
		},
		{
			name: "list with multiple cwds and force reload",
			params: codex.SkillsListParams{
				Cwds:        []string{"/home/user/project1", "/home/user/project2"},
				ForceReload: ptr(true),
			},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"cwd":    "/home/user/project1",
						"errors": []interface{}{},
						"skills": []interface{}{
							map[string]interface{}{
								"name":             "skill-one",
								"description":      "First skill",
								"path":             "/home/user/project1/.claude/skills/skill-one",
								"enabled":          true,
								"scope":            "user",
								"shortDescription": "Legacy short desc",
								"interface": map[string]interface{}{
									"displayName":      "Skill One",
									"shortDescription": "Short description",
									"defaultPrompt":    "Default prompt",
									"brandColor":       "#FF5733",
								},
							},
						},
					},
					map[string]interface{}{
						"cwd": "/home/user/project2",
						"errors": []interface{}{
							map[string]interface{}{
								"path":    "/home/user/project2/.claude/skills/broken-skill",
								"message": "Syntax error in SKILL.md",
							},
						},
						"skills": []interface{}{},
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.SkillsListResponse) {
				if len(resp.Data) != 2 {
					t.Errorf("expected 2 entries, got %d", len(resp.Data))
				}
				// First entry
				entry1 := resp.Data[0]
				if len(entry1.Skills) != 1 {
					t.Errorf("expected 1 skill in first entry, got %d", len(entry1.Skills))
				}
				skill := entry1.Skills[0]
				if skill.Interface == nil {
					t.Fatal("expected interface to be set")
				}
				if skill.Interface.DisplayName == nil || *skill.Interface.DisplayName != "Skill One" {
					t.Errorf("expected interface.displayName = Skill One")
				}
				if skill.ShortDescription == nil || *skill.ShortDescription != "Legacy short desc" {
					t.Errorf("expected shortDescription = Legacy short desc")
				}
				// Second entry
				entry2 := resp.Data[1]
				if len(entry2.Errors) != 1 {
					t.Errorf("expected 1 error in second entry, got %d", len(entry2.Errors))
				}
				if entry2.Errors[0].Message != "Syntax error in SKILL.md" {
					t.Errorf("expected error message = Syntax error in SKILL.md, got %s", entry2.Errors[0].Message)
				}
			},
		},
		{
			name: "list with dependencies",
			params: codex.SkillsListParams{
				Cwds: []string{"/home/user/project"},
			},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"cwd":    "/home/user/project",
						"errors": []interface{}{},
						"skills": []interface{}{
							map[string]interface{}{
								"name":        "skill-with-deps",
								"description": "Skill with tool dependencies",
								"path":        "/home/user/project/.claude/skills/skill-with-deps",
								"enabled":     true,
								"scope":       "repo",
								"dependencies": map[string]interface{}{
									"tools": []interface{}{
										map[string]interface{}{
											"type":        "mcp",
											"value":       "example-mcp",
											"command":     "npx example-mcp",
											"description": "Example MCP tool",
											"url":         "https://github.com/example/example-mcp",
											"transport":   "stdio",
										},
									},
								},
							},
						},
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.SkillsListResponse) {
				if len(resp.Data) != 1 {
					t.Errorf("expected 1 entry, got %d", len(resp.Data))
				}
				entry := resp.Data[0]
				if len(entry.Skills) != 1 {
					t.Errorf("expected 1 skill, got %d", len(entry.Skills))
				}
				skill := entry.Skills[0]
				if skill.Dependencies == nil {
					t.Fatal("expected dependencies to be set")
				}
				if len(skill.Dependencies.Tools) != 1 {
					t.Errorf("expected 1 tool dependency, got %d", len(skill.Dependencies.Tools))
				}
				tool := skill.Dependencies.Tools[0]
				if tool.Type != "mcp" {
					t.Errorf("expected tool type = mcp, got %s", tool.Type)
				}
				if tool.Value != "example-mcp" {
					t.Errorf("expected tool value = example-mcp, got %s", tool.Value)
				}
				if tool.Command == nil || *tool.Command != "npx example-mcp" {
					t.Errorf("expected tool command = npx example-mcp")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("skills/list", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Skills.List(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Skills.List failed: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify correct JSON-RPC method was called
			if len(mock.SentRequests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(mock.SentRequests))
			}
			if mock.SentRequests[0].Method != "skills/list" {
				t.Errorf("expected method = skills/list, got %s", mock.SentRequests[0].Method)
			}
		})
	}
}

func TestSkillsConfigWrite(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.SkillsConfigWriteParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.SkillsConfigWriteResponse)
		wantJSON      map[string]interface{}
	}{
		{
			name: "enable skill",
			params: codex.SkillsConfigWriteParams{
				Path:    "/home/user/.claude/skills/my-skill",
				Enabled: true,
			},
			mockResponse: map[string]interface{}{
				"effectiveEnabled": true,
			},
			checkResponse: func(t *testing.T, resp codex.SkillsConfigWriteResponse) {
				if resp.EffectiveEnabled != true {
					t.Errorf("expected effectiveEnabled = true, got %v", resp.EffectiveEnabled)
				}
			},
			wantJSON: map[string]interface{}{
				"path":    "/home/user/.claude/skills/my-skill",
				"enabled": true,
			},
		},
		{
			name: "disable skill",
			params: codex.SkillsConfigWriteParams{
				Path:    "/home/user/.claude/skills/my-skill",
				Enabled: false,
			},
			mockResponse: map[string]interface{}{
				"effectiveEnabled": false,
			},
			checkResponse: func(t *testing.T, resp codex.SkillsConfigWriteResponse) {
				if resp.EffectiveEnabled != false {
					t.Errorf("expected effectiveEnabled = false, got %v", resp.EffectiveEnabled)
				}
			},
			wantJSON: map[string]interface{}{
				"path":    "/home/user/.claude/skills/my-skill",
				"enabled": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("skills/config/write", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Skills.ConfigWrite(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Skills.ConfigWrite failed: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify correct JSON-RPC method was called
			if len(mock.SentRequests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(mock.SentRequests))
			}
			if mock.SentRequests[0].Method != "skills/config/write" {
				t.Errorf("expected method = skills/config/write, got %s", mock.SentRequests[0].Method)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(mock.SentRequests[0].Params, &got); err != nil {
				t.Fatalf("request params decode failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantJSON) {
				t.Errorf("request params = %#v, want %#v", got, tt.wantJSON)
			}
		})
	}
}

func TestSkillsList_RPCError_ReturnsRPCError(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	mock.SetResponse("skills/list", codex.Response{
		JSONRPC: "2.0",
		Error: &codex.Error{
			Code:    codex.ErrCodeInternalError,
			Message: "skills backend unreachable",
		},
	})

	_, err := client.Skills.List(context.Background(), codex.SkillsListParams{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rpcErr *codex.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected error to unwrap to *RPCError, got %T", err)
	}
	if rpcErr.RPCError().Code != codex.ErrCodeInternalError {
		t.Errorf("expected error code %d, got %d", codex.ErrCodeInternalError, rpcErr.RPCError().Code)
	}
}

func TestSkillsListRejectsInvalidScope(t *testing.T) {
	mock := NewMockTransport()
	_ = mock.SetResponseData("skills/list", map[string]interface{}{
		"data": []interface{}{
			map[string]interface{}{
				"cwd":    "/home/user/project",
				"errors": []interface{}{},
				"skills": []interface{}{
					map[string]interface{}{
						"name":        "example-skill",
						"description": "An example skill",
						"path":        "/home/user/project/.claude/skills/example-skill",
						"enabled":     true,
						"scope":       "team",
					},
				},
			},
		},
	})
	client := codex.NewClient(mock)

	_, err := client.Skills.List(context.Background(), codex.SkillsListParams{})
	if err == nil || !strings.Contains(err.Error(), "invalid skill.scope") {
		t.Fatalf("Skills.List error = %v; want invalid scope failure", err)
	}
}
