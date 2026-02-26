package codex_test

import (
	"testing"

	"github.com/dominicnunez/codex-sdk-go"
)

func TestPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "int",
			input:    42,
			expected: 42,
		},
		{
			name:     "bool true",
			input:    true,
			expected: true,
		},
		{
			name:     "bool false",
			input:    false,
			expected: false,
		},
		{
			name:     "uint32",
			input:    uint32(100),
			expected: uint32(100),
		},
		{
			name:     "int64",
			input:    int64(9999),
			expected: int64(9999),
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: 3.14159,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "zero int",
			input:    0,
			expected: 0,
		},
		{
			name:     "struct",
			input:    struct{ Name string }{Name: "test"},
			expected: struct{ Name string }{Name: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch v := tt.input.(type) {
			case string:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if *result != tt.expected.(string) {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			case int:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if *result != tt.expected.(int) {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			case bool:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if *result != tt.expected.(bool) {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			case uint32:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if *result != tt.expected.(uint32) {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			case int64:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if *result != tt.expected.(int64) {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			case float64:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if *result != tt.expected.(float64) {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			case struct{ Name string }:
				result := codex.Ptr(v)
				if result == nil {
					t.Fatal("Ptr returned nil")
				}
				if result.Name != tt.expected.(struct{ Name string }).Name {
					t.Errorf("Ptr() = %v, want %v", *result, tt.expected)
				}
			}
		})
	}
}

func TestPtrNilCheck(t *testing.T) {
	// Test that Ptr returns a valid pointer that can be dereferenced
	strPtr := codex.Ptr("test")
	if strPtr == nil {
		t.Fatal("Ptr returned nil for string")
	}

	intPtr := codex.Ptr(123)
	if intPtr == nil {
		t.Fatal("Ptr returned nil for int")
	}

	// Test that the pointer addresses are different for different calls
	str1 := codex.Ptr("same")
	str2 := codex.Ptr("same")
	if str1 == str2 {
		t.Error("Ptr returned same pointer address for different calls")
	}
}

func TestPtrWithZeroValues(t *testing.T) {
	// Test that Ptr works with zero values (important for optional fields)
	zeroInt := codex.Ptr(0)
	if zeroInt == nil {
		t.Fatal("Ptr returned nil for zero int")
	}
	if *zeroInt != 0 {
		t.Errorf("Ptr(0) = %v, want 0", *zeroInt)
	}

	emptyStr := codex.Ptr("")
	if emptyStr == nil {
		t.Fatal("Ptr returned nil for empty string")
	}
	if *emptyStr != "" {
		t.Errorf("Ptr(\"\") = %v, want \"\"", *emptyStr)
	}

	falseBool := codex.Ptr(false)
	if falseBool == nil {
		t.Fatal("Ptr returned nil for false")
	}
	if *falseBool != false {
		t.Errorf("Ptr(false) = %v, want false", *falseBool)
	}
}

func TestPtrUsageExample(t *testing.T) {
	// Example: construct optional fields in a struct
	type Config struct {
		Name        string
		Title       *string
		Enabled     *bool
		MaxRetries  *int
		Timeout     *float64
		Description *string
	}

	cfg := Config{
		Name:        "my-config",
		Title:       codex.Ptr("My Config"),
		Enabled:     codex.Ptr(true),
		MaxRetries:  codex.Ptr(3),
		Timeout:     codex.Ptr(30.0),
		Description: nil, // explicitly nil
	}

	if cfg.Title == nil || *cfg.Title != "My Config" {
		t.Errorf("Title = %v, want %v", cfg.Title, "My Config")
	}
	if cfg.Enabled == nil || *cfg.Enabled != true {
		t.Errorf("Enabled = %v, want true", cfg.Enabled)
	}
	if cfg.MaxRetries == nil || *cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %v, want 3", cfg.MaxRetries)
	}
	if cfg.Timeout == nil || *cfg.Timeout != 30.0 {
		t.Errorf("Timeout = %v, want 30.0", cfg.Timeout)
	}
	if cfg.Description != nil {
		t.Errorf("Description = %v, want nil", cfg.Description)
	}
}
