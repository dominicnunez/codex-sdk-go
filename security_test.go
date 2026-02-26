package codex_test

import (
	"os"
	"strings"
	"testing"
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
