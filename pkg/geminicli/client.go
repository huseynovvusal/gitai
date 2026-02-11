package geminicli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	GeminiCommand    = "gemini"
	GeminiPromptFlag = "-p"
	GeminiModelFlag  = "-m"
	DefaultModel     = "gemini-3-flash-preview"
	MaxRetries       = 3
)

var (
	ErrEmptyPrompt        = errors.New("prompt cannot be empty")
	ErrCommandNotFound    = errors.New("gemini command not found in PATH")
	ErrCommandFailed      = errors.New("failed to execute Gemini command")
	ErrParseOutput        = errors.New("failed to parse Gemini output")
	ErrEmptyOutput        = errors.New("empty output from Gemini command")
	ErrAuthFailed         = errors.New("authentication error: please check your Gemini API credentials")
	ErrServiceUnavailable = errors.New("service unavailable: Gemini API is currently overloaded or down")
)

// TokenUsage represents token usage statistics
type TokenUsage struct {
	Input      int `json:"input"`
	Prompt     int `json:"prompt"`
	Candidates int `json:"candidates"`
	Total      int `json:"total"`
	Cached     int `json:"cached"`
	Thoughts   int `json:"thoughts"`
}

// DetailedResponse represents a response with additional metadata
type DetailedResponse struct {
	Response   string
	TokenUsage TokenUsage
}

// Client represents a Gemini CLI client
type Client struct {
	logger           Logger
	model            string
	workingDirectory string
}

// Config represents configuration options for the client
type Config struct {
	Logger           Logger
	Model            string
	WorkingDirectory string
}

// NewClient creates a new Gemini CLI client
func NewClient(config ...Config) *Client {
	client := &Client{
		logger: NewNoOpLogger(),
		model:  DefaultModel,
	}

	if len(config) > 0 {
		cfg := config[0]
		if cfg.Logger != nil {
			client.logger = cfg.Logger
		}
		if cfg.Model != "" {
			client.model = cfg.Model
		}
		if cfg.WorkingDirectory != "" {
			client.workingDirectory = cfg.WorkingDirectory
		}
	}

	return client
}

// Execute executes a Gemini command with the given prompt
func (c *Client) Execute(ctx context.Context, prompt string) (string, error) {
	resp, err := c.ExecuteDetailed(ctx, prompt)
	if err != nil {
		return "", err
	}
	return resp.Response, nil
}

// ExecuteDetailed executes a Gemini command and returns detailed response including token usage
func (c *Client) ExecuteDetailed(ctx context.Context, prompt string) (*DetailedResponse, error) {
	if prompt == "" {
		return nil, ErrEmptyPrompt
	}

	geminiPath, err := exec.LookPath(GeminiCommand)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCommandNotFound, err)
	}

	resolvedPrompt := prompt
	if c.workingDirectory != "" {
		if currentDir, err := os.Getwd(); err == nil {
			resolvedPrompt = c.resolveRelativePaths(prompt, currentDir)
		}
	}

	// Use JSON output format to get stats
	cmdArgs := []string{GeminiModelFlag, c.model, "-o", "json", GeminiPromptFlag, resolvedPrompt}
	var detailedResp *DetailedResponse

	retryer := &Retryer{
		MaxRetries: MaxRetries,
		Logger:     c.logger,
		ShouldRetry: func(err error) bool {
			return errors.Is(err, ErrServiceUnavailable)
		},
	}

	err = retryer.Do(ctx, func() error {
		cmd := exec.CommandContext(ctx, geminiPath, cmdArgs...)
		c.setupCommandDir(cmd)

		output, err := cmd.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				stderr := string(exitErr.Stderr)
				if c.detectAuthError(stderr) {
					return ErrAuthFailed
				}
				if c.isRetryableError(stderr) {
					return ErrServiceUnavailable
				}
				return fmt.Errorf("%w: %s", ErrCommandFailed, stderr)
			}
			return fmt.Errorf("%w: %w", ErrCommandFailed, err)
		}

		res, err := c.parseDetailedOutput(output)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrParseOutput, err)
		}

		detailedResp = res
		return nil
	})

	return detailedResp, err
}

func (c *Client) setupCommandDir(cmd *exec.Cmd) {
	if c.workingDirectory != "" {
		cmd.Dir = c.workingDirectory
	} else if currentDir, err := os.Getwd(); err == nil {
		cmd.Dir = currentDir
	} else if home := os.Getenv("HOME"); home != "" {
		cmd.Dir = home
	} else if u, err := user.Current(); err == nil {
		cmd.Dir = u.HomeDir
	}
}

func (c *Client) isRetryableError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "service unavailable") || strings.Contains(lower, "overloaded")
}

func (c *Client) detectAuthError(stderr string) bool {
	keywords := []string{"authentication failed", "invalid api key", "permission denied", "unauthorized", "access denied"}
	lower := strings.ToLower(stderr)
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// jsonResponse represents the structure of gemini -o json output
type jsonResponse struct {
	Response string `json:"response"`
	Stats    struct {
		Models map[string]struct {
			Tokens TokenUsage `json:"tokens"`
		} `json:"models"`
	} `json:"stats"`
}

func (c *Client) parseDetailedOutput(output []byte) (*DetailedResponse, error) {
	if len(output) == 0 {
		return nil, ErrEmptyOutput
	}

	var jr jsonResponse
	if err := json.Unmarshal(output, &jr); err != nil {
		return nil, err
	}

	// Filter system messages from the response text
	jr.Response = c.filterGeminiOutput(strings.TrimSpace(jr.Response))
	if jr.Response == "" {
		return nil, ErrEmptyOutput
	}

	detailed := &DetailedResponse{
		Response: jr.Response,
	}

	// Find token usage for the current model
	// The CLI might report stats for multiple models if it switched models internally,
	// but we'll try to find the one that matches our requested model, or just take the first one if only one exists.
	if len(jr.Stats.Models) > 0 {
		// Try exact match first
		if stats, ok := jr.Stats.Models[c.model]; ok {
			detailed.TokenUsage = stats.Tokens
		} else {
			// Fallback to the first available model stats
			for _, stats := range jr.Stats.Models {
				detailed.TokenUsage = stats.Tokens
				break
			}
		}
	}

	return detailed, nil
}

func (c *Client) filterGeminiOutput(output string) string {
	lines := strings.Split(output, "\n")
	var filtered []string
	patterns := []string{
		"Loaded cached credentials.", "Loading cached credentials", "Authenticating",
		"Authentication successful", "Connected to Gemini API", "Using cached token",
		"Token refreshed", "Trusting folder", "Activating skill", "Executing hook", "Gemini context",
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		shouldFilter := false
		for _, p := range patterns {
			if strings.Contains(trimmed, p) {
				shouldFilter = true
				break
			}
		}
		if !shouldFilter && trimmed != "" {
			filtered = append(filtered, line)
		}
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func (c *Client) resolveRelativePaths(prompt string, baseDir string) string {
	pathPattern := regexp.MustCompile(`(?:\./|\.\./)[\w\-./]+|[\w\-./]*\.(?:txt|md|go|js|py|json|yaml|yml|xml|html|css|sh|conf|cfg|ini|log|out|err|csv|tsv|sql|db|lock|mod|sum|env|toml|proto|pb|rs|c|cpp|h|hpp|java|kt|php|rb|swift|dart|scala|clj|hs|elm|ml|fs|pl|r|m|mm|vue|jsx|tsx|svelte|astro|wasm|zip|tar|gz|bz2|xz|7z|rar|pdf|doc|docx|xls|xlsx|ppt|pptx|png|jpg|jpeg|gif|bmp|svg|webp|ico|mp3|mp4|avi|mov|wmv|flv|mkv|webm|wav|ogg|flac|aac|m4a|ttf|otf|woff|woff2|eot)\b`)
	return pathPattern.ReplaceAllStringFunc(prompt, func(match string) string {
		match = strings.TrimSpace(match)
		if match == "" || filepath.IsAbs(match) {
			return match
		}
		cleanPath := filepath.Clean(filepath.Join(baseDir, match))
		c.logger.DebugWith("Resolved relative path", "original", match, "resolved", cleanPath)
		return cleanPath
	})
}
