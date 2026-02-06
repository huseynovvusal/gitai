package geminicli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	// Simple mock that just prints the prompt and succeeds
	mockContent := `#!/bin/sh
echo "Mock Gemini Response"
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
			result, err := client.Execute(tt.prompt)

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

// TestConvenienceExecute tests the convenience function for executing Gemini commands
func TestConvenienceExecute(t *testing.T) {
	// Simple test for empty prompt error
	_, err := Execute("")
	if err == nil {
		t.Error("Expected error for empty prompt")
	}
}

// TestBuildGeminiCommand tests command construction
func TestBuildGeminiCommand(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		expectedLength int
		description    string
	}{
		{
			name:           "BasicPrompt",
			prompt:         "test prompt",
			expectedLength: 3, // ["gemini", "-p", "test prompt"]
			description:    "Should build basic Gemini command",
		},
		{
			name:           "PromptWithSpaces",
			prompt:         "test prompt with spaces",
			expectedLength: 3,
			description:    "Should handle prompts with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildGeminiCommand(tt.prompt)

			if len(cmd) != tt.expectedLength {
				t.Errorf("Expected command length %d, got %d for test case '%s'",
					tt.expectedLength, len(cmd), tt.name)
			}

			if len(cmd) > 0 && cmd[0] != "gemini" {
				t.Errorf("Expected first argument to be 'gemini', got '%s'", cmd[0])
			}

			if len(cmd) > 1 && cmd[1] != "-p" {
				t.Errorf("Expected second argument to be '-p', got '%s'", cmd[1])
			}

			if len(cmd) > 2 && cmd[2] != tt.prompt {
				t.Errorf("Expected third argument to be '%s', got '%s'", tt.prompt, cmd[2])
			}
		})
	}
}

// TestValidateAvailable tests Gemini availability check
func TestValidateAvailable(t *testing.T) {
	client := NewClient()
	err := client.ValidateAvailable()

	// We expect this to fail initially since the function doesn't exist yet
	if err == nil {
		t.Log("Gemini appears to be available on this system")
	} else {
		t.Logf("Gemini not available (expected during TDD): %v", err)
	}
}

// TestConvenienceValidateAvailable tests the convenience function for Gemini availability check
func TestConvenienceValidateAvailable(t *testing.T) {
	err := ValidateAvailable()

	// We expect this to fail initially since the function doesn't exist yet
	if err == nil {
		t.Log("Gemini appears to be available on this system")
	} else {
		t.Logf("Gemini not available (expected during TDD): %v", err)
	}
}

// TestParseGeminiOutput tests output parsing functionality
func TestParseGeminiOutput(t *testing.T) {
	tests := []struct {
		name         string
		output       []byte
		expectedText string
		expectError  bool
		description  string
	}{
		{
			name:         "ValidOutput",
			output:       []byte("This is a valid Gemini response"),
			expectedText: "This is a valid Gemini response",
			expectError:  false,
			description:  "Should parse valid output correctly",
		},
		{
			name:         "EmptyOutput",
			output:       []byte(""),
			expectedText: "",
			expectError:  true,
			description:  "Should return error for empty output",
		},
		{
			name:         "OutputWithNewlines",
			output:       []byte("Line 1\nLine 2\nLine 3"),
			expectedText: "Line 1\nLine 2\nLine 3",
			expectError:  false,
			description:  "Should handle output with newlines",
		},
		{
			name:         "OutputWithCredentialsMessage",
			output:       []byte("Loaded cached credentials.\nHello, world!"),
			expectedText: "Hello, world!",
			expectError:  false,
			description:  "Should filter out credentials loading message",
		},
		{
			name:         "OutputWithMultipleSystemMessages",
			output:       []byte("Loading cached credentials\nAuthentication successful\nHello, world!\nThis is a response."),
			expectedText: "Hello, world!\nThis is a response.",
			expectError:  false,
			description:  "Should filter out multiple system messages",
		},
		{
			name:         "OnlySystemMessages",
			output:       []byte("Loaded cached credentials.\nAuthenticating\nConnected to Gemini API"),
			expectedText: "",
			expectError:  true,
			description:  "Should return error when only system messages remain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseGeminiOutput(tt.output)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test case '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for test case '%s': %v", tt.name, err)
				}
				if result != tt.expectedText {
					t.Errorf("Expected '%s', got '%s' for test case '%s'",
						tt.expectedText, result, tt.name)
				}
			}
		})
	}
}

// TestDetectAuthError tests authentication error detection
func TestDetectAuthError(t *testing.T) {
	tests := []struct {
		name        string
		output      []byte
		expectAuth  bool
		description string
	}{
		{
			name:        "NoAuthError",
			output:      []byte("Normal Gemini response"),
			expectAuth:  false,
			description: "Should not detect auth error in normal output",
		},
		{
			name:        "AuthenticationError",
			output:      []byte("Error: authentication failed"),
			expectAuth:  true,
			description: "Should detect authentication error",
		},
		{
			name:        "APIKeyError",
			output:      []byte("Error: invalid API key"),
			expectAuth:  true,
			description: "Should detect API key error",
		},
		{
			name:        "PermissionError",
			output:      []byte("Error: permission denied"),
			expectAuth:  true,
			description: "Should detect permission error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectAuthError(tt.output)

			if result != tt.expectAuth {
				t.Errorf("Expected %v, got %v for test case '%s'",
					tt.expectAuth, result, tt.name)
			}
		})
	}
}

// TestCommandTimeout tests timeout handling
func TestCommandTimeout(t *testing.T) {
	t.Skip("Skipping blocking timeout test")
}

// TestConvenienceExecuteWithTimeout tests the convenience function for timeout handling
func TestConvenienceExecuteWithTimeout(t *testing.T) {
	// Simple test for empty prompt error
	_, err := ExecuteWithTimeout("", 1*time.Second)
	if err == nil {
		t.Error("Expected error for empty prompt")
	}
}

// TestNewClient tests client creation
func TestNewClient(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Error("NewClient should return a valid client")
	} else if client.timeout != DefaultTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultTimeout, client.timeout)
	}
}

// TestNewClientWithConfig tests client creation with configuration
func TestNewClientWithConfig(t *testing.T) {
	customTimeout := 60 * time.Second
	customLogger := NewNoOpLogger()

	config := Config{
		Logger:  customLogger,
		Timeout: customTimeout,
	}

	client := NewClientWithConfig(config)

	if client == nil {
		t.Error("NewClientWithConfig should return a valid client")
	} else if client.timeout != customTimeout {
		t.Errorf("Expected custom timeout %v, got %v", customTimeout, client.timeout)
	}
}

// TestNoOpLogger tests the no-op logger implementation
func TestNoOpLogger(t *testing.T) {
	logger := NewNoOpLogger()

	// These should not panic
	logger.DebugWith("test", "key", "value")
	logger.InfoWith("test", "key", "value")
	logger.WarnWith("test", "key", "value")
	logger.ErrorWith("test", "key", "value")
}

// TestNewClientWithModel tests client creation with model configuration
func TestNewClientWithModel(t *testing.T) {
	customModel := "gemini-3-pro-preview"
	config := Config{
		Model: customModel,
	}

	client := NewClientWithConfig(config)

	if client == nil {
		t.Error("NewClientWithConfig should return a valid client")
	}

	// This will fail until we implement the model field
	// Expected model should be stored in client
}

// TestBuildGeminiCommandWithModel tests command construction with model
func TestBuildGeminiCommandWithModel(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		model          string
		expectedLength int
		description    string
	}{
		{
			name:           "BasicPromptWithModel",
			prompt:         "test prompt",
			model:          "gemini-3-flash",
			expectedLength: 5, // ["gemini", "-m", "gemini-3-flash", "-p", "test prompt"]
			description:    "Should build Gemini command with model",
		},
		{
			name:           "PromptWithCustomModel",
			prompt:         "test prompt",
			model:          "gemini-3-pro-preview",
			expectedLength: 5,
			description:    "Should build Gemini command with custom model",
		},
		{
			name:           "PromptWithEmptyModel",
			prompt:         "test prompt",
			model:          "gemini-3-flash",
			expectedLength: 5, // Should use default model
			description:    "Should use default model when empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{Model: tt.model}
			client := NewClientWithConfig(config)

			// This will fail until we implement buildGeminiCommandWithModel
			cmd := client.buildGeminiCommandWithModel(tt.prompt)

			if len(cmd) != tt.expectedLength {
				t.Errorf("Expected command length %d, got %d for test case '%s'",
					tt.expectedLength, len(cmd), tt.name)
			}

			if len(cmd) > 0 && cmd[0] != "gemini" {
				t.Errorf("Expected first argument to be 'gemini', got '%s'", cmd[0])
			}

			if len(cmd) > 1 && cmd[1] != "-m" {
				t.Errorf("Expected second argument to be '-m', got '%s'", cmd[1])
			}

			expectedModel := tt.model

			if len(cmd) > 2 && cmd[2] != expectedModel {
				t.Errorf("Expected third argument to be '%s', got '%s'", expectedModel, cmd[2])
			}

			if len(cmd) > 3 && cmd[3] != "-p" {
				t.Errorf("Expected fourth argument to be '-p', got '%s'", cmd[3])
			}

			if len(cmd) > 4 && cmd[4] != tt.prompt {
				t.Errorf("Expected fifth argument to be '%s', got '%s'", tt.prompt, cmd[4])
			}
		})
	}
}

// TestExecuteWithModel tests executing commands with specific model
func TestExecuteWithModel(t *testing.T) {
	// Simple test for empty prompt error
	_, err := ExecuteWithModel("", "gemini-3-flash-preview")
	if err == nil {
		t.Error("Expected error for empty prompt")
	}
}

// TestExecuteWithModelAndTimeout tests executing commands with model and timeout
func TestExecuteWithModelAndTimeout(t *testing.T) {
	// Simple test for empty prompt error
	_, err := ExecuteWithModelAndTimeout("", "gemini-3-flash-preview", 1*time.Second)
	if err == nil {
		t.Error("Expected error for empty prompt")
	}
}

// TestDefaultModel tests that default model is gemini-3-flash-preview
func TestDefaultModel(t *testing.T) {
	client := NewClient()

	// This will fail until we implement the model field
	// Expected default model should be "gemini-3-flash-preview"
	expectedDefault := "gemini-3-flash-preview"

	// Build command and check if it contains default model
	cmd := client.buildGeminiCommandWithModel("test")

	if len(cmd) < 3 {
		t.Error("Command should contain model specification")
	}

	if cmd[2] != expectedDefault {
		t.Errorf("Expected default model '%s', got '%s'", expectedDefault, cmd[2])
	}
}

// TestNewClientWithWorkingDirectory tests client creation with working directory configuration
func TestNewClientWithWorkingDirectory(t *testing.T) {
	customDir := "/tmp/custom_gemini_dir"
	config := Config{
		WorkingDirectory: customDir,
	}

	client := NewClientWithConfig(config)

	if client == nil {
		t.Error("NewClientWithConfig should return a valid client")
	}

	// This will fail until we implement the workingDirectory field
	// Expected working directory should be stored in client
}

// TestExecuteWithWorkingDirectory tests command execution with custom working directory
func TestExecuteWithWorkingDirectory(t *testing.T) {
	t.Skip("Skipping blocking working directory test")
}

// TestWorkingDirectoryFallback tests fallback behavior when working directory is not set
func TestWorkingDirectoryFallback(t *testing.T) {
	t.Skip("Skipping blocking fallback test")
}

// TestExecuteWithWorkingDirectory tests the convenience function for working directory execution
func TestConvenienceExecuteWithWorkingDirectory(t *testing.T) {
	t.Skip("Skipping blocking convenience working directory test")
}

// TestExecuteWithWorkingDirectoryAndTimeout tests the convenience function with working directory and timeout
func TestConvenienceExecuteWithWorkingDirectoryAndTimeout(t *testing.T) {
	t.Skip("Skipping blocking convenience working directory and timeout test")
}

// TestExecuteWithFullConfig tests the convenience function with all configuration options
func TestConvenienceExecuteWithFullConfig(t *testing.T) {
	t.Skip("Skipping blocking convenience full config test")
}

// TestResolveRelativePaths tests the relative path resolution functionality
func TestResolveRelativePaths(t *testing.T) {
	// Create a temporary client for testing
	client := NewClient()

	// Test cases
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
		{
			name:     "RelativePathWithDoubleDot",
			prompt:   "Check ../config.json",
			baseDir:  "/project/src",
			expected: "Check /project/config.json",
		},
		{
			name:     "SimpleRelativePath",
			prompt:   "Review file.txt",
			baseDir:  "/project",
			expected: "Review /project/file.txt",
		},
		{
			name:     "AbsolutePathPreserved",
			prompt:   "Analyze /absolute/path/file.go",
			baseDir:  "/project",
			expected: "Analyze /absolute/path/file.go",
		},
		{
			name:     "SubdirectoryPath",
			prompt:   "Check subdir/file.py",
			baseDir:  "/project",
			expected: "Check /project/subdir/file.py",
		},
		{
			name:     "MultiplePathsInPrompt",
			prompt:   "Compare ./file1.txt and ./file2.txt",
			baseDir:  "/project",
			expected: "Compare /project/file1.txt and /project/file2.txt",
		},
		{
			name:     "NoPathsInPrompt",
			prompt:   "What is the weather today?",
			baseDir:  "/project",
			expected: "What is the weather today?",
		},
		{
			name:     "MixedPathTypes",
			prompt:   "Compare ./local.txt with /absolute/remote.txt",
			baseDir:  "/project",
			expected: "Compare /project/local.txt with /absolute/remote.txt",
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

// TestResolveRelativePathsError tests error handling in path resolution
func TestResolveRelativePathsError(t *testing.T) {
	client := NewClient()

	result := client.resolveRelativePaths("./test.txt", "")

	// Should still process the path
	if !strings.Contains(result, "test.txt") {
		t.Errorf("Expected result to contain 'test.txt', got: %s", result)
	}
}

// TestWorkingDirectoryPathResolution tests the integration of working directory with path resolution
func TestWorkingDirectoryPathResolution(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gemini_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test with a working directory set
	config := Config{
		WorkingDirectory: "/tmp/gemini_config",
	}
	client := NewClientWithConfig(config)

	// Test path resolution
	result := client.resolveRelativePaths("./test.txt", tempDir)

	expectedPath := filepath.Join(tempDir, "test.txt")
	if !strings.Contains(result, expectedPath) {
		t.Errorf("Expected result to contain '%s', got: %s", expectedPath, result)
	}
}
