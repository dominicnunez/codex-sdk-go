package codex_test

import (
	"os"
	"strings"
	"testing"
)

func TestLicenseMdExists(t *testing.T) {
	data, err := os.ReadFile("LICENSE.md")
	if err != nil {
		t.Fatalf("LICENSE.md should exist: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("LICENSE.md should not be empty")
	}
}

func TestLicenseMdContainsMITLicense(t *testing.T) {
	data, err := os.ReadFile("LICENSE.md")
	if err != nil {
		t.Fatalf("LICENSE.md should exist: %v", err)
	}
	content := string(data)

	requiredSections := []string{
		"MIT License",
		"Copyright (c) 2025 Dominic Nunez",
		"Permission is hereby granted, free of charge",
		"THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND",
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("LICENSE.md should contain %q", section)
		}
	}
}

func TestLicenseMdContainsStandardMITClauses(t *testing.T) {
	data, err := os.ReadFile("LICENSE.md")
	if err != nil {
		t.Fatalf("LICENSE.md should exist: %v", err)
	}
	content := string(data)

	// Normalize whitespace for multi-line matching
	normalizedContent := strings.ReplaceAll(content, "\n", " ")
	normalizedContent = strings.Join(strings.Fields(normalizedContent), " ")

	standardClauses := []string{
		"use, copy, modify, merge, publish, distribute, sublicense, and/or sell",
		"copies of the Software",
		"subject to the following conditions",
		"The above copyright notice and this permission notice shall be included",
		"EXPRESS OR IMPLIED",
		"MERCHANTABILITY",
		"FITNESS FOR A PARTICULAR PURPOSE",
		"NONINFRINGEMENT",
	}

	for _, clause := range standardClauses {
		if !strings.Contains(normalizedContent, clause) {
			t.Errorf("LICENSE.md should contain standard MIT clause: %q", clause)
		}
	}
}
