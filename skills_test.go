package codex_test

import (
	"context"
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("skills/configWrite", tt.mockResponse)
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
			if mock.SentRequests[0].Method != "skills/configWrite" {
				t.Errorf("expected method = skills/configWrite, got %s", mock.SentRequests[0].Method)
			}
		})
	}
}

func TestSkillsRemoteRead(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.SkillsRemoteReadParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.SkillsRemoteReadResponse)
	}{
		{
			name:   "default params - only enabled example skills",
			params: codex.SkillsRemoteReadParams{},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"id":          "skill-id-1",
						"name":        "Example Skill 1",
						"description": "An example skill from the library",
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.SkillsRemoteReadResponse) {
				if len(resp.Data) != 1 {
					t.Errorf("expected 1 remote skill, got %d", len(resp.Data))
				}
				skill := resp.Data[0]
				if skill.ID != "skill-id-1" {
					t.Errorf("expected id = skill-id-1, got %s", skill.ID)
				}
				if skill.Name != "Example Skill 1" {
					t.Errorf("expected name = Example Skill 1, got %s", skill.Name)
				}
			},
		},
		{
			name: "all shared skills for chatgpt",
			params: codex.SkillsRemoteReadParams{
				Enabled:        ptr(true),
				HazelnutScope:  ptr(codex.HazelnutScopeAllShared),
				ProductSurface: ptr(codex.ProductSurfaceChatGPT),
			},
			mockResponse: map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"id":          "skill-id-2",
						"name":        "Shared Skill",
						"description": "A shared skill available on ChatGPT",
					},
					map[string]interface{}{
						"id":          "skill-id-3",
						"name":        "Another Shared Skill",
						"description": "Another shared skill",
					},
				},
			},
			checkResponse: func(t *testing.T, resp codex.SkillsRemoteReadResponse) {
				if len(resp.Data) != 2 {
					t.Errorf("expected 2 remote skills, got %d", len(resp.Data))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("skills/remoteRead", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Skills.RemoteRead(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Skills.RemoteRead failed: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify correct JSON-RPC method was called
			if len(mock.SentRequests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(mock.SentRequests))
			}
			if mock.SentRequests[0].Method != "skills/remoteRead" {
				t.Errorf("expected method = skills/remoteRead, got %s", mock.SentRequests[0].Method)
			}
		})
	}
}

func TestSkillsRemoteWrite(t *testing.T) {
	tests := []struct {
		name          string
		params        codex.SkillsRemoteWriteParams
		mockResponse  map[string]interface{}
		checkResponse func(t *testing.T, resp codex.SkillsRemoteWriteResponse)
	}{
		{
			name: "install remote skill",
			params: codex.SkillsRemoteWriteParams{
				HazelnutID: "remote-skill-id-123",
			},
			mockResponse: map[string]interface{}{
				"id":   "local-skill-id-456",
				"path": "/home/user/.claude/skills/remote-skill",
			},
			checkResponse: func(t *testing.T, resp codex.SkillsRemoteWriteResponse) {
				if resp.ID != "local-skill-id-456" {
					t.Errorf("expected id = local-skill-id-456, got %s", resp.ID)
				}
				if resp.Path != "/home/user/.claude/skills/remote-skill" {
					t.Errorf("expected path = /home/user/.claude/skills/remote-skill, got %s", resp.Path)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			_ = mock.SetResponseData("skills/remoteWrite", tt.mockResponse)
			client := codex.NewClient(mock)

			resp, err := client.Skills.RemoteWrite(context.Background(), tt.params)
			if err != nil {
				t.Fatalf("Skills.RemoteWrite failed: %v", err)
			}

			tt.checkResponse(t, resp)

			// Verify correct JSON-RPC method was called
			if len(mock.SentRequests) != 1 {
				t.Fatalf("expected 1 request, got %d", len(mock.SentRequests))
			}
			if mock.SentRequests[0].Method != "skills/remoteWrite" {
				t.Errorf("expected method = skills/remoteWrite, got %s", mock.SentRequests[0].Method)
			}
		})
	}
}

func TestSkillsServiceMethodSignatures(t *testing.T) {
	mock := NewMockTransport()
	client := codex.NewClient(mock)

	// Compile-time verification that all methods exist with correct signatures
	var _ func(context.Context, codex.SkillsListParams) (codex.SkillsListResponse, error) = client.Skills.List
	var _ func(context.Context, codex.SkillsConfigWriteParams) (codex.SkillsConfigWriteResponse, error) = client.Skills.ConfigWrite
	var _ func(context.Context, codex.SkillsRemoteReadParams) (codex.SkillsRemoteReadResponse, error) = client.Skills.RemoteRead
	var _ func(context.Context, codex.SkillsRemoteWriteParams) (codex.SkillsRemoteWriteResponse, error) = client.Skills.RemoteWrite
}
