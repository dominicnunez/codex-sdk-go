package codex_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	codex "github.com/dominicnunez/codex-sdk-go"
)

func TestSecurityMdExists(t *testing.T) {
	content, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatalf("SECURITY.md should exist: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("SECURITY.md should not be empty")
	}
}

func TestSecurityMdContainsRequiredSections(t *testing.T) {
	content, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatalf("failed to read SECURITY.md: %v", err)
	}

	text := string(content)

	requiredSections := []struct {
		name    string
		content string
	}{
		{"Security Policy header", "# Security Policy"},
		{"Reporting section", "## Reporting a Vulnerability"},
		{"GitHub issues link", "https://github.com/dominicnunez/codex-sdk-go/issues"},
		{"Security Updates section", "## Security Updates"},
		{"Scope section", "## Scope"},
		{"Dependencies section", "## Dependencies"},
		{"Contact section", "## Contact"},
	}

	for _, section := range requiredSections {
		if !strings.Contains(text, section.content) {
			t.Errorf("SECURITY.md should contain %s: expected to find %q", section.name, section.content)
		}
	}
}

func TestSecurityMdReportingGuidance(t *testing.T) {
	content, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatalf("failed to read SECURITY.md: %v", err)
	}

	text := string(content)

	// Verify reporting guidance includes what to include
	requiredGuidance := []string{
		"description of the vulnerability",
		"Steps to reproduce",
		"impact",
		"suggested fixes",
	}

	for _, guidance := range requiredGuidance {
		if !strings.Contains(text, guidance) {
			t.Errorf("SECURITY.md should include reporting guidance for: %s", guidance)
		}
	}
}

func TestSecurityMdSecurityScope(t *testing.T) {
	content, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatalf("failed to read SECURITY.md: %v", err)
	}

	text := string(content)

	// Verify security scope covers key areas
	securityAreas := []string{
		"Input Validation",
		"Transport Security",
		"Error Handling",
		"Concurrency",
	}

	for _, area := range securityAreas {
		if !strings.Contains(text, area) {
			t.Errorf("SECURITY.md should cover security area: %s", area)
		}
	}
}

func TestSecurityMdZeroDependencies(t *testing.T) {
	content, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatalf("failed to read SECURITY.md: %v", err)
	}

	text := string(content)

	if !strings.Contains(text, "zero external dependencies") {
		t.Error("SECURITY.md should mention zero external dependencies")
	}

	if !strings.Contains(text, "standard library") {
		t.Error("SECURITY.md should mention Go standard library")
	}
}

func TestSecurityRejectsTurnCompletedMissingTurnID(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		err error
	}
	ch := make(chan runResult, 1)

	go func() {
		_, err := proc.Run(ctx, codex.RunOptions{Prompt: "missing turn id"})
		ch <- runResult{err: err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"status":"completed","items":[]}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from missing turn.id")
	}
	if !strings.Contains(result.err.Error(), "invalid turn/completed notification") {
		t.Errorf("error = %q, want invalid turn/completed notification", result.err)
	}
}

func TestSecurityRejectsTurnCompletedNonTerminalStatus(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type runResult struct {
		err error
	}
	ch := make(chan runResult, 1)

	go func() {
		_, err := proc.Run(ctx, codex.RunOptions{Prompt: "invalid terminal status"})
		ch <- runResult{err: err}
	}()

	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress","items":[]}}`),
	})

	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from non-terminal turn.status")
	}
	if !strings.Contains(result.err.Error(), "invalid turn/completed notification") {
		t.Errorf("error = %q, want invalid turn/completed notification", result.err)
	}
}

func TestSecurityStreamRejectsTurnCompletedMissingThreadID(t *testing.T) {
	proc, mock := mockProcess(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := proc.RunStreamed(ctx, codex.RunOptions{Prompt: "missing thread id"})
	time.Sleep(50 * time.Millisecond)

	mock.InjectServerNotification(ctx, codex.Notification{
		JSONRPC: "2.0",
		Method:  "turn/completed",
		Params:  json.RawMessage(`{"turn":{"id":"turn-1","status":"completed","items":[]}}`),
	})

	var gotErr error
	for _, err := range stream.Events() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("expected streamed error from missing threadId")
	}
	if !strings.Contains(gotErr.Error(), "invalid turn/completed notification") {
		t.Errorf("error = %q, want invalid turn/completed notification", gotErr)
	}
}
