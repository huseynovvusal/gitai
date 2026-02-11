package geminicli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecute tests the main function for executing Gemini commands
func TestExecute(t *testing.T) {
	// Setup a mock gemini command
	tmpDir, err := os.MkdirTemp("", "gemini-mock")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mockPath := filepath.Join(tmpDir, "gemini")
	// Simple mock that returns JSON if requested, otherwise plain text
	// Our client now always requests -o json
	mockContent := `#!/bin/sh
echo '{"response": "Mock Gemini Response", "stats": {"models": {"gemini-3-flash-preview": {"tokens": {"input": 10, "prompt": 10, "candidates": 5, "total": 15}}}}}'
`
	if err := os.WriteFile(mockPath, []byte(mockContent), 0755); err != nil {
		t.Fatalf("Failed to create mock gemini: %v", err)
	}

	// Update PATH to include our mock
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+originalPath)
	defer os.Setenv("PATH", originalPath)

	tests := []struct {
		name        string
		prompt      string
		expectError bool
		description string
	}{
		{
			name:        "EmptyPrompt",
			prompt:      "",
			expectError: true,
			description: "Should return error for empty prompt",
		},
		{
			name:        "ValidPrompt",
			prompt:      "test prompt",
			expectError: false,
			description: "Should succeed with mock gemini",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient()
			ctx := context.Background()
			result, err := client.Execute(ctx, tt.prompt)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test case '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test case '%s': %v", tt.name, err)
				}
				if !strings.Contains(result, "Mock Gemini Response") {
					t.Errorf("Expected mock response, got: %s", result)
				}
			}
		})
	}
}

// TestExecuteDetailed tests token usage reporting
func TestExecuteDetailed(t *testing.T) {
	// Setup a mock gemini command
	tmpDir, err := os.MkdirTemp("", "gemini-mock-detailed")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mockPath := filepath.Join(tmpDir, "gemini")
	mockContent := `#!/bin/sh
echo '{"response": "Mock Response", "stats": {"models": {"gemini-3-flash-preview": {"tokens": {"input": 100, "prompt": 100, "candidates": 50, "total": 150}}}}}'
`
	if err := os.WriteFile(mockPath, []byte(mockContent), 0755); err != nil {
		t.Fatalf("Failed to create mock gemini: %v", err)
	}

	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+originalPath)
	defer os.Setenv("PATH", originalPath)

	client := NewClient()
	resp, err := client.ExecuteDetailed(context.Background(), "test")
	if err != nil {
		t.Fatalf("ExecuteDetailed failed: %v", err)
	}

	if resp.Response != "Mock Response" {
		t.Errorf("Expected 'Mock Response', got '%s'", resp.Response)
	}
	if resp.TokenUsage.Total != 150 {
		t.Errorf("Expected total tokens 150, got %d", resp.TokenUsage.Total)
	}
	if resp.TokenUsage.Input != 100 {
		t.Errorf("Expected input tokens 100, got %d", resp.TokenUsage.Input)
	}
}

// TestParseDetailedOutput tests JSON parsing
func TestParseDetailedOutput(t *testing.T) {
	client := NewClient()
	jsonIn := []byte(`{"response": "Hello", "stats": {"models": {"gemini-3-flash-preview": {"tokens": {"total": 42}}}}}`)

	resp, err := client.parseDetailedOutput(jsonIn)
	if err != nil {
		t.Fatalf("parseDetailedOutput failed: %v", err)
	}

	if resp.Response != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", resp.Response)
	}
	if resp.TokenUsage.Total != 42 {
		t.Errorf("Expected 42 tokens, got %d", resp.TokenUsage.Total)
	}
}

// TestDetectAuthError tests authentication error detection
func TestDetectAuthError(t *testing.T) {
	client := NewClient()
	tests := []struct {
		name        string
		output      string
		expectAuth  bool
		description string
	}{
		{
			name:        "NoAuthError",
			output:      "Normal Gemini response",
			expectAuth:  false,
			description: "Should not detect auth error in normal output",
		},
		{
			name:        "AuthenticationError",
			output:      "Error: authentication failed",
			expectAuth:  true,
			description: "Should detect authentication error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.detectAuthError(tt.output)

			if result != tt.expectAuth {
				t.Errorf("Expected %v, got %v for test case '%s'",
					tt.expectAuth, result, tt.name)
			}
		})
	}
}

// TestNewClient tests client creation
func TestNewClient(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Error("NewClient should return a valid client")
	}
}

// TestResolveRelativePaths tests the relative path resolution functionality
func TestResolveRelativePaths(t *testing.T) {
	client := NewClient()
	tests := []struct {
		name     string
		prompt   string
		baseDir  string
		expected string
	}{
		{
			name:     "RelativePathWithDot",
			prompt:   "Analyze ./main.go",
			baseDir:  "/project/src",
			expected: "Analyze /project/src/main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.resolveRelativePaths(tt.prompt, tt.baseDir)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
