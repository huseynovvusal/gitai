package provider

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go/v2"
	openaiOption "github.com/openai/openai-go/v2/option"
	geminicli "github.com/yubiquita/gemini-cli-wrapper"
	"google.golang.org/genai"
)

// ParseProvider parses a string into a Provider (case-insensitive).
func ParseProvider(str string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(str)) {
	case "gpt", "openai", "gpt3", "gpt3.5", "gpt4":
		return ProviderGPT, nil
	case "gemini", "google":
		return ProviderGemini, nil
	case "geminicli", "gemini_cli", "gemini_wrapper", "gemini-cli", "gemini-wrapper":
		return ProvideGeminiCLI, nil
	case "ollama", "local":
		return ProviderOllama, nil
	case "anthropic", "claude":
		return ProviderAnthropic, nil
	case "groq":
		return ProviderGroq, nil
	case "deepseek":
		return ProviderDeepSeek, nil
	case "xai", "grok":
		return ProviderXAI, nil
	default:
		return ProviderNone, &InvalidProviderError{Provider: str}
	}
}

// GPTProvider implements AIProvider for OpenAI.
type GPTProvider struct {
	apiKey      string
	baseUrl     string
	maxTokens   int64
	temperature float64
	model       string
}

// NewGPTProvider creates a new GPTProvider.
func NewGPTProvider(
	apiKey string,
	baseUrl string,
	maxTokens int64,
	temperature float64,
	model string,
) *GPTProvider {
	if model == "" {
		model = openai.ChatModelGPT4oMini
	}
	return &GPTProvider{
		apiKey:      apiKey,
		baseUrl:     baseUrl,
		maxTokens:   maxTokens,
		temperature: temperature,
		model:       model,
	}
}

// GenerateContent generates content using OpenAI.
func (p *GPTProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiKey == "" {
		return "", ErrAPIKeyNotSet
	}
	args := []openaiOption.RequestOption{openaiOption.WithAPIKey(p.apiKey)}
	if p.baseUrl != "" {
		args = append(args, openaiOption.WithBaseURL(p.baseUrl))
	}
	client := openai.NewClient(
		args...,
	)

	res, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(p.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(userMessage),
		},
		MaxTokens:   openai.Int(p.maxTokens),
		Temperature: openai.Float(p.temperature),
	})

	if err != nil {
		return "", fmt.Errorf("openai request failed: %w", err)
	}

	if len(res.Choices) == 0 {
		return "", ErrNoResponse
	}

	return res.Choices[0].Message.Content, nil
}

// GeminiProvider implements AIProvider for Google Gemini.
type GeminiProvider struct {
	apiKey      string
	maxTokens   int32
	temperature float32
}

// NewGeminiProvider creates a new GeminiProvider.
func NewGeminiProvider(apiKey string, maxTokens int32, temperature float32) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey, maxTokens: maxTokens, temperature: temperature}
}

// GenerateContent generates content using Google Gemini.
func (p *GeminiProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: p.apiKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create gemini client: %w", err)
	}

	contents := []*genai.Content{
		{
			Role:  "system",
			Parts: []*genai.Part{{Text: systemMessage}},
		},
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: userMessage}},
		},
	}
	modelConfig := &genai.GenerateContentConfig{Temperature: &p.temperature, MaxOutputTokens: p.maxTokens}

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", contents, modelConfig)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", ErrNoResponse
	}

	return resp.Candidates[0].Content.Parts[0].Text, nil
}

// OllamaProvider implements AIProvider for Ollama.
type OllamaProvider struct {
	apiPath string
}

// NewOllamaProvider creates a new OllamaProvider.
func NewOllamaProvider(apiPath string) *OllamaProvider {
	return &OllamaProvider{apiPath: apiPath}
}

// GenerateContent generates content using local Ollama.
func (p *OllamaProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiPath == "" {
		return "", ErrOllamaPathMissing
	}

	tCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	prompt := fmt.Sprintf("%s\n\n%s", systemMessage, userMessage)
	cmd := exec.CommandContext(tCtx, p.apiPath, "run", "llama3.1:8b", prompt)
	out, err := cmd.CombinedOutput()

	if errors.Is(tCtx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("ollama command timed out: %w", tCtx.Err())
	}

	if err != nil {
		return "", fmt.Errorf("ollama command failed: %w, output: %s", err, string(out))
	}

	return string(out), nil
}

// GeminiCLIProvider implements AIProvider for Gemini CLI Wrapper.
type GeminiCLIProvider struct{}

// NewGeminiCLIProvider creates a new GeminiCLIProvider.
func NewGeminiCLIProvider() *GeminiCLIProvider {
	return &GeminiCLIProvider{}
}

// GenerateContent generates content using Gemini CLI.
func (p *GeminiCLIProvider) GenerateContent(_ context.Context, systemMessage, userMessage string) (string, error) {
	prompt := fmt.Sprintf("System: %s\nUser: %s", systemMessage, userMessage)
	client := geminicli.NewClient()
	resp, err := client.Execute(prompt)
	if err != nil {
		return "", fmt.Errorf("geminicli execution failed: %w", err)
	}
	return resp, nil
}

// AnthropicProvider implements AIProvider for Anthropic (Claude).
type AnthropicProvider struct {
	apiKey      string
	maxTokens   int
	temperature float64
}

// NewAnthropicProvider creates a new AnthropicProvider.
func NewAnthropicProvider(apiKey string, maxTokens int, temperature float64) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:      apiKey,
		maxTokens:   maxTokens,
		temperature: temperature,
	}
}

// GenerateContent generates content using Anthropic.
func (p *AnthropicProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client := anthropic.NewClient(option.WithAPIKey(p.apiKey))

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model: anthropic.Model("claude-3-5-sonnet-20240620"),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
		},
		System: []anthropic.TextBlockParam{
			{
				Text: systemMessage,
				Type: "text",
			},
		},
		MaxTokens:   int64(p.maxTokens),
		Temperature: anthropic.Float(p.temperature),
	})

	if err != nil {
		return "", fmt.Errorf("anthropic request failed: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", ErrNoResponse
	}

	return resp.Content[0].Text, nil
}
