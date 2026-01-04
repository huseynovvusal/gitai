package security

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestParseKeywordsCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Empty string", "", []string{}},
		{"Single keyword", "api_key", []string{"api_key"}},
		{"Multiple keywords", "api_key,password,secret", []string{"api_key", "password", "secret"}},
		{"Keywords with spaces", " api_key , password ", []string{"api_key", "password"}},
		{"Mixed case", "API_KEY,PassWord", []string{"api_key", "password"}},
		{"Trailing comma", "key1,key2,", []string{"key1", "key2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseKeywordsCSV(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %q, got %q", tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestGetSensitiveKeywords(t *testing.T) {
	viper.Reset()
	
	// 1. Default/Empty
	kw := GetSensitiveKeywords()
	if len(kw) != 0 {
		t.Errorf("expected empty keywords, got %v", kw)
	}

	// 2. Slice from config
	viper.Set("security.keywords", []string{"key1", "key2"})
	kw = GetSensitiveKeywords()
	if len(kw) != 2 || kw[0] != "key1" {
		t.Errorf("expected [key1 key2], got %v", kw)
	}

	// 3. Single string with commas (Env var case)
	viper.Set("security.keywords", "env1,env2")
	kw = GetSensitiveKeywords()
	if len(kw) != 2 || kw[0] != "env1" {
		t.Errorf("expected [env1 env2], got %v", kw)
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
	viper.Reset()
	viper.Set("security.keywords", []string{"api_key", "secret"})

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
			diff: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,1 +1,1 @@
-old code
+new code`,
			expectError: false,
		},
		{
			name: "Keyword in added line",
			diff: `diff --git a/config.go b/config.go
--- a/config.go
+++ b/config.go
@@ -1,1 +1,2 @@
+package config
+const API_KEY = "12345"`,
			expectError: true,
			errContains: "config.go",
		},
		{
			name: "Keyword in removed line (safe)",
			diff: `diff --git a/config.go b/config.go
--- a/config.go
+++ b/config.go
@@ -1,2 +1,1 @@
-const API_KEY = "12345"
+const SAFE = "ok"`,
			expectError: false,
		},
		{
			name: "Keyword in context line (safe)",
			diff: `diff --git a/config.go b/config.go
--- a/config.go
+++ b/config.go
@@ -1,3 +1,3 @@
 package config
-const OLD = "1"
+const NEW = "2"
 const API_KEY = "keep"`,
			expectError: false,
		},
		{
			name: "Safe diff with context lines",
			diff: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,3 @@
 package main
-func old() {}
+func new() {}
 func context() {}`,
			expectError: false,
		},
		{
			name: "Multiple findings",
			diff: `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,1 @@
+my secret
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,1 +1,1 @@
+my api_key`,
			expectError: true,
			errContains: "a.go",
		},
		{
			name: "Absolute path diff",
			diff: `diff --git a/tmp/a.go b/tmp/a.go
--- a/tmp/a.go
+++ b/tmp/a.go
@@ -1,1 +1,1 @@
+my secret`,
			expectError: true,
			errContains: "/tmp/a.go",
		},
		{
			name:        "Invalid diff format",
			diff:        "this is not a diff",
			expectError: false,
		},
		{
			name: "Unknown prefix in hunk",
			diff: `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -1,1 +1,1 @@
!unknown prefix`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckDiffSafety(tt.diff)
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

func TestCheckDiffSafety_InvalidParse(t *testing.T) {
	err := CheckDiffSafety("diff --git\n--- a/file\n+++ b/file\n@@ -invalid")
	if err != nil {
		t.Logf("Parse error hit: %v", err)
	}
}