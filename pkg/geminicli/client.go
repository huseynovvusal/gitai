package geminicli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Constants
const (
	// Gemini command related
	GeminiCommand    = "gemini"
	GeminiPromptFlag = "-p"
	GeminiModelFlag  = "-m"
	DefaultTimeout   = 30 * time.Second
	DefaultModel     = "gemini-3-flash"
	MaxRetries       = 3

	// Error messages
	ErrEmptyPrompt        = "prompt cannot be empty"
	ErrCommandNotFound    = "Gemini command not found in PATH"
	ErrCommandFailed      = "failed to execute Gemini command"
	ErrCommandTimeout     = "command timed out"
	ErrCommandStart       = "failed to start command"
	ErrParseOutput        = "failed to parse Gemini output"
	ErrEmptyOutput        = "empty output from Gemini command"
	ErrAuthFailed         = "authentication error: please check your Gemini API credentials"
	ErrServiceUnavailable = "service unavailable: Gemini API is currently overloaded or down"
)

// Client represents a Gemini CLI client
type Client struct {
	logger           Logger
	timeout          time.Duration
	model            string // Model name to use
	workingDirectory string // Working directory for command execution
}

// Config represents configuration options for the client
type Config struct {
	Logger           Logger
	Timeout          time.Duration
	Model            string // Model name (e.g., "gemini-3-flash-preview", "gemini-3-pro-preview")
	WorkingDirectory string // Working directory for command execution
}

// NewClient creates a new Gemini CLI client with default configuration
func NewClient() *Client {
	return &Client{
		logger:  NewNoOpLogger(),
		timeout: DefaultTimeout,
		model:   DefaultModel,
	}
}

// NewClientWithConfig creates a new Gemini CLI client with custom configuration
func NewClientWithConfig(config Config) *Client {
	client := &Client{
		timeout: DefaultTimeout,
		model:   DefaultModel,
	}

	if config.Logger != nil {
		client.logger = config.Logger
	} else {
		client.logger = NewNoOpLogger()
	}

	if config.Timeout > 0 {
		client.timeout = config.Timeout
	}

	if config.Model != "" {
		client.model = config.Model
	}

	if config.WorkingDirectory != "" {
		client.workingDirectory = config.WorkingDirectory
	}

	return client
}

// Execute executes a Gemini command with the given prompt
func (c *Client) Execute(prompt string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf(ErrEmptyPrompt)
	}

	// Resolve relative paths if the working directory is set
	resolvedPrompt := prompt
	if c.workingDirectory != "" {
		currentDir, err := os.Getwd()
		if err != nil {
			c.logger.WarnWith("Failed to get current directory for path resolution", "error", err)
		} else {
			resolvedPrompt, err = c.resolveRelativePaths(prompt, currentDir)
			if err != nil {
				c.logger.WarnWith("Failed to resolve relative paths", "error", err)
				resolvedPrompt = prompt // Use original prompt if resolution fails
			}
		}
	}

	// Build command
	cmdArgs := c.buildGeminiCommandWithModel(resolvedPrompt)

	// Log command execution for debugging
	c.logger.DebugWith("Executing Gemini command", "command", cmdArgs[0], "args", cmdArgs[1:])

	// Create command with full path to avoid module resolution issues
	geminiPath, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		c.logger.ErrorWith("Failed to find gemini command", "error", err)
		return "", fmt.Errorf("gemini command not found: %w", err)
	}

	c.logger.DebugWith("Using gemini path", "path", geminiPath)
	cmd := exec.Command(geminiPath, cmdArgs[1:]...)

	// Set working directory based on configuration or fallback to current directory
	if c.workingDirectory != "" {
		cmd.Dir = c.workingDirectory
		c.logger.DebugWith("Using configured working directory", "dir", cmd.Dir)
	} else {
		// Use current working directory as default
		cmd.Dir, err = os.Getwd()
		if err != nil || cmd.Dir == "" {
			// Fallback to home directory if current directory cannot be determined
			cmd.Dir = os.Getenv("HOME")
			if cmd.Dir == "" {
				// Final fallback to current user's home directory
				if user, err := user.Current(); err == nil {
					cmd.Dir = user.HomeDir
				}
			}
		}
		c.logger.DebugWith("Using current/default directory", "dir", cmd.Dir)
	}

	// Execute with timeout
	output, err := c.runCommandWithTimeout(cmd, c.timeout)
	if err != nil {
		c.logger.ErrorWith("Gemini command execution failed", "error", err)
		return "", fmt.Errorf("%s: %w", ErrCommandFailed, err)
	}

	// Parse output
	result, err := c.parseGeminiOutput(output)
	if err != nil {
		c.logger.ErrorWith("Failed to parse Gemini output", "error", err, "output_length", len(output))
		return "", fmt.Errorf("%s: %w", ErrParseOutput, err)
	}

	c.logger.DebugWith("Gemini command completed successfully", "response_length", len(result))
	return result, nil
}

// ExecuteWithTimeout executes Gemini command with custom timeout
func (c *Client) ExecuteWithTimeout(prompt string, timeout time.Duration) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf(ErrEmptyPrompt)
	}

	// Resolve relative paths if working directory is set
	resolvedPrompt := prompt
	if c.workingDirectory != "" {
		currentDir, err := os.Getwd()
		if err != nil {
			c.logger.WarnWith("Failed to get current directory for path resolution", "error", err)
		} else {
			resolvedPrompt, err = c.resolveRelativePaths(prompt, currentDir)
			if err != nil {
				c.logger.WarnWith("Failed to resolve relative paths", "error", err)
				resolvedPrompt = prompt // Use original prompt if resolution fails
			}
		}
	}

	// Build command
	cmdArgs := c.buildGeminiCommandWithModel(resolvedPrompt)

	// Log command execution for debugging
	c.logger.DebugWith("Executing Gemini command with timeout", "command", cmdArgs[0], "args", cmdArgs[1:], "timeout", timeout)

	// Create command with full path to avoid module resolution issues
	geminiPath, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		c.logger.ErrorWith("Failed to find gemini command", "error", err)
		return "", fmt.Errorf("gemini command not found: %w", err)
	}

	c.logger.DebugWith("Using gemini path", "path", geminiPath)
	cmd := exec.Command(geminiPath, cmdArgs[1:]...)

	// Set working directory based on configuration or fallback to current directory
	if c.workingDirectory != "" {
		cmd.Dir = c.workingDirectory
		c.logger.DebugWith("Using configured working directory", "dir", cmd.Dir)
	} else {
		// Use current working directory as default
		cmd.Dir, err = os.Getwd()
		if err != nil || cmd.Dir == "" {
			// Fallback to home directory if current directory cannot be determined
			cmd.Dir = os.Getenv("HOME")
			if cmd.Dir == "" {
				// Final fallback to current user's home directory
				if user, err := user.Current(); err == nil {
					cmd.Dir = user.HomeDir
				}
			}
		}
		c.logger.DebugWith("Using current/default directory", "dir", cmd.Dir)
	}

	// Execute with custom timeout
	output, err := c.runCommandWithTimeout(cmd, timeout)
	if err != nil {
		c.logger.ErrorWith("Gemini command execution failed", "error", err)
		return "", fmt.Errorf("%s: %w", ErrCommandFailed, err)
	}

	// Parse output
	result, err := c.parseGeminiOutput(output)
	if err != nil {
		c.logger.ErrorWith("Failed to parse Gemini output", "error", err, "output_length", len(output))
		return "", fmt.Errorf("%s: %w", ErrParseOutput, err)
	}

	c.logger.DebugWith("Gemini command completed successfully", "response_length", len(result))
	return result, nil
}

// ValidateAvailable checks if Gemini command is available
func (c *Client) ValidateAvailable() error {
	_, err := exec.LookPath(GeminiCommand)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrCommandNotFound, err)
	}
	return nil
}

// buildGeminiCommand builds the command arguments for Gemini
func (c *Client) buildGeminiCommand(prompt string) []string {
	return []string{GeminiCommand, GeminiPromptFlag, prompt}
}

// buildGeminiCommandWithModel builds the command arguments for Gemini with model specification
func (c *Client) buildGeminiCommandWithModel(prompt string) []string {
	return []string{GeminiCommand, GeminiModelFlag, c.model, GeminiPromptFlag, prompt}
}

// runCommandWithTimeout executes a command with the specified timeout
func (c *Client) runCommandWithTimeout(cmd *exec.Cmd, timeout time.Duration) ([]byte, error) {
	// Start the command
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrCommandStart, err)
	}

	// Channel to signal command completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		if err != nil {
			// Capture both stdout and stderr for detailed error reporting
			stdoutStr := strings.TrimSpace(string(stdout.Bytes()))
			stderrStr := strings.TrimSpace(string(stderr.Bytes()))
			combined := append(stdout.Bytes(), stderr.Bytes()...)

			// Check if it's an authentication error
			if c.detectAuthError(combined) {
				return nil, fmt.Errorf(ErrAuthFailed)
			}

			// Check if it's a service unavailable error
			combinedStr := strings.ToLower(string(combined))
			if strings.Contains(combinedStr, "service unavailable") ||
				strings.Contains(combinedStr, "overloaded") {
				return nil, fmt.Errorf(ErrServiceUnavailable)
			}

			// Create detailed error message
			errorMsg := fmt.Sprintf("command failed: %v", err)
			if stderrStr != "" {
				errorMsg += fmt.Sprintf(" | stderr: %s", stderrStr)
			}
			if stdoutStr != "" {
				errorMsg += fmt.Sprintf(" | stdout: %s", stdoutStr)
			}

			return nil, fmt.Errorf("%s", errorMsg)
		}
		return stdout.Bytes(), nil
	case <-time.After(timeout):
		// Kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("%s after %v", ErrCommandTimeout, timeout)
	}
}

// parseGeminiOutput parses the output from Gemini command
func (c *Client) parseGeminiOutput(output []byte) (string, error) {
	if len(output) == 0 {
		return "", fmt.Errorf(ErrEmptyOutput)
	}

	// Convert to string and trim whitespace
	result := strings.TrimSpace(string(output))

	// Filter out authentication and system messages
	result = c.filterGeminiOutput(result)

	if result == "" {
		return "", fmt.Errorf(ErrEmptyOutput)
	}

	return result, nil
}

// detectAuthError detects authentication-related errors in command output
func (c *Client) detectAuthError(output []byte) bool {
	return c.containsAnyKeyword(string(output), c.getAuthErrorKeywords())
}

// getAuthErrorKeywords returns list of authentication error keywords
func (c *Client) getAuthErrorKeywords() []string {
	return []string{
		"authentication failed",
		"invalid api key",
		"permission denied",
		"unauthorized",
		"access denied",
	}
}

// containsAnyKeyword checks if text contains any of the specified keywords (case-insensitive)
func (c *Client) containsAnyKeyword(text string, keywords []string) bool {
	lowerText := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}
	return false
}

// filterGeminiOutput filters out authentication and system messages from Gemini output
func (c *Client) filterGeminiOutput(output string) string {
	// Split output into lines
	lines := strings.Split(output, "\n")
	var filteredLines []string

	// Filter patterns that should be removed
	filterPatterns := []string{
		"Loaded cached credentials.",
		"Loading cached credentials",
		"Authenticating",
		"Authentication successful",
		"Connected to Gemini API",
		"Using cached token",
		"Token refreshed",
		"Trusting folder",
		"Activating skill",
		"Executing hook",
		"Gemini context",
	}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		shouldFilter := false

		// Check if line matches any filter pattern
		for _, pattern := range filterPatterns {
			if strings.Contains(trimmedLine, pattern) {
				shouldFilter = true
				break
			}
		}

		// Keep the line if it doesn't match filter patterns and isn't empty
		if !shouldFilter && trimmedLine != "" {
			filteredLines = append(filteredLines, line)
		}
	}

	// Join filtered lines and normalize whitespace
	result := strings.Join(filteredLines, "\n")
	return strings.TrimSpace(result)
}

// resolveRelativePaths resolves relative paths in the prompt to absolute paths
func (c *Client) resolveRelativePaths(prompt string, baseDir string) (string, error) {
	// Regular expression to match file paths
	// This pattern matches:
	// - ./file.txt, ../file.txt (explicit relative paths)
	// - file.txt, subdir/file.txt (files with common extensions)
	// - /absolute/path/file.txt (absolute paths, preserved)
	pathPattern := regexp.MustCompile(`(?:\./|\.\./)[\w\-./]+|[\w\-.\/]*\.(?:txt|md|go|js|py|json|yaml|yml|xml|html|css|sh|conf|cfg|ini|log|out|err|csv|tsv|sql|db|lock|mod|sum|env|toml|proto|pb|rs|c|cpp|h|hpp|java|kt|php|rb|swift|dart|scala|clj|hs|elm|ml|fs|pl|r|m|mm|vue|jsx|tsx|svelte|astro|wasm|zip|tar|gz|bz2|xz|7z|rar|pdf|doc|docx|xls|xlsx|ppt|pptx|png|jpg|jpeg|gif|bmp|svg|webp|ico|mp3|mp4|avi|mov|wmv|flv|mkv|webm|wav|ogg|flac|aac|m4a|ttf|otf|woff|woff2|eot)\b`)

	// Replace matches with resolved paths
	result := pathPattern.ReplaceAllStringFunc(prompt, func(match string) string {
		match = strings.TrimSpace(match)
		if match == "" {
			return match
		}

		// Skip if already absolute path
		if filepath.IsAbs(match) {
			return match
		}

		// Resolve relative path
		resolvedPath := filepath.Join(baseDir, match)
		cleanPath := filepath.Clean(resolvedPath)

		c.logger.DebugWith("Resolved relative path", "original", match, "resolved", cleanPath)
		return cleanPath
	})

	return result, nil
}

// Convenience functions for backward compatibility

// Execute executes a Gemini command with the given prompt using default client
func Execute(prompt string) (string, error) {
	client := NewClient()
	return client.Execute(prompt)
}

// ExecuteWithTimeout executes Gemini command with custom timeout using default client
func ExecuteWithTimeout(prompt string, timeout time.Duration) (string, error) {
	client := NewClient()
	return client.ExecuteWithTimeout(prompt, timeout)
}

// ValidateAvailable checks if Gemini command is available using default client
func ValidateAvailable() error {
	client := NewClient()
	return client.ValidateAvailable()
}

// BuildGeminiCommand builds the command arguments for Gemini
func BuildGeminiCommand(prompt string) []string {
	client := NewClient()
	return client.buildGeminiCommand(prompt)
}

// DetectAuthError detects authentication-related errors in command output
func DetectAuthError(output []byte) bool {
	client := NewClient()
	return client.detectAuthError(output)
}

// ParseGeminiOutput parses the output from Gemini command
func ParseGeminiOutput(output []byte) (string, error) {
	client := NewClient()
	return client.parseGeminiOutput(output)
}

// ExecuteWithModel executes a Gemini command with the specified model
func ExecuteWithModel(prompt, model string) (string, error) {
	config := Config{Model: model}
	client := NewClientWithConfig(config)
	return client.Execute(prompt)
}

// ExecuteWithModelAndTimeout executes a Gemini command with the specified model and timeout
func ExecuteWithModelAndTimeout(prompt, model string, timeout time.Duration) (string, error) {
	config := Config{Model: model, Timeout: timeout}
	client := NewClientWithConfig(config)
	return client.Execute(prompt)
}

// ExecuteWithWorkingDirectory executes a Gemini command with the specified working directory
func ExecuteWithWorkingDirectory(prompt, workingDirectory string) (string, error) {
	config := Config{WorkingDirectory: workingDirectory}
	client := NewClientWithConfig(config)
	return client.Execute(prompt)
}

// ExecuteWithWorkingDirectoryAndTimeout executes a Gemini command with the specified working directory and timeout
func ExecuteWithWorkingDirectoryAndTimeout(prompt, workingDirectory string, timeout time.Duration) (string, error) {
	config := Config{WorkingDirectory: workingDirectory, Timeout: timeout}
	client := NewClientWithConfig(config)
	return client.Execute(prompt)
}

// ExecuteWithFullConfig executes a Gemini command with all configuration options
func ExecuteWithFullConfig(prompt, model, workingDirectory string, timeout time.Duration) (string, error) {
	config := Config{
		Model:            model,
		WorkingDirectory: workingDirectory,
		Timeout:          timeout,
	}
	client := NewClientWithConfig(config)
	return client.Execute(prompt)
}
