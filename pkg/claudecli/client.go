package claudecli

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	ClaudeCommand  = "claude"
	DefaultTimeout = 60 * time.Second
	DefaultModel   = "sonnet"
)

// Client represents a Claude Code CLI client.
type Client struct {
	timeout time.Duration
	model   string
}

// Config represents configuration options for the client.
type Config struct {
	Timeout time.Duration
	Model   string
}

// NewClient creates a new Claude CLI client with default configuration.
func NewClient() *Client {
	return &Client{
		timeout: DefaultTimeout,
		model:   DefaultModel,
	}
}

// NewClientWithConfig creates a new Claude CLI client with custom configuration.
func NewClientWithConfig(config Config) *Client {
	client := &Client{
		timeout: DefaultTimeout,
		model:   DefaultModel,
	}
	if config.Timeout > 0 {
		client.timeout = config.Timeout
	}
	if config.Model != "" {
		client.model = config.Model
	}
	return client
}

// Execute runs the claude CLI with the given prompt in print mode.
func (c *Client) Execute(prompt string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}

	claudePath, err := exec.LookPath(ClaudeCommand)
	if err != nil {
		return "", fmt.Errorf("claude command not found: %w", err)
	}

	args := []string{"-p", prompt, "--model", c.model, "--output-format", "text"}
	cmd := exec.Command(claudePath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude command: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("claude command failed: %w | stderr: %s", err, strings.TrimSpace(stderr.String()))
		}
		result := strings.TrimSpace(stdout.String())
		if result == "" {
			return "", fmt.Errorf("empty output from claude command")
		}
		return result, nil
	case <-time.After(c.timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("claude command timed out after %v", c.timeout)
	}
}

// ValidateAvailable checks if the claude command is available in PATH.
func ValidateAvailable() error {
	_, err := exec.LookPath(ClaudeCommand)
	if err != nil {
		return fmt.Errorf("claude command not found in PATH: %w", err)
	}
	return nil
}
