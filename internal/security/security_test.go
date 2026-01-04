package security

import (
	"strings"
	"testing"
)

func TestCheckDiffSafety_Absolute(t *testing.T) {
	keywords := []string{"secret"}
	// Valid unified diff with @@ header
	diff := "diff --git b/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,1 @@\n+my secret"
	absPath := "/tmp/a.go"
	diffAbs := strings.ReplaceAll(diff, "a.go", absPath)

	err := CheckDiffSafety(diffAbs, keywords)
	if err == nil {
		t.Error("expected error for absolute path diff containing secret")
	}
}

func TestContainsKeyword(t *testing.T) {
	keywords := []string{"api_key", "password"}
	tests := []struct {
		input    string
		expected bool
	}{
		{"nothing here", false},
		{"here is an api_key", true},
		{"API_KEY in caps", true},
		{"password123", true},
		{"other stuff", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if containsKeyword(tt.input, keywords) != tt.expected {
				t.Errorf("expected %v for %q", tt.expected, tt.input)
			}
		})
	}
}

func TestCheckDiffSafety(t *testing.T) {
	keywords := []string{"api_key", "secret"}

	tests := []struct {
		name        string
		diff        string
		expectError bool
		errContains string
	}{
		{
			name:        "Empty diff",
			diff:        "",
			expectError: false,
		},
		{
			name: "Safe diff",
			diff: `diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,1 +1,1 @@\n-old code\n+new code`,
			expectError: false,
		},
		{
			name: "Keyword in added line",
			diff: `diff --git a/config.go b/config.go\n--- a/config.go\n+++ b/config.go\n@@ -1,1 +1,2 @@\n+package config\n+const API_KEY = "12345"`,
			expectError: true,
			errContains: "config.go",
		},
		{
			name: "Keyword in removed line (safe)",
			diff: `diff --git a/config.go b/config.go\n--- a/config.go\n+++ b/config.go\n@@ -1,2 +1,1 @@\n-const API_KEY = "12345"\n+const SAFE = "ok"`,
			expectError: false,
		},
		{
			name: "Multiple findings",
			diff: `diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,1 @@\n+my secret\ndiff --git b/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1,1 +1,1 @@\n+my api_key`,
			expectError: true,
			errContains: "a.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckDiffSafety(tt.diff, keywords)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}