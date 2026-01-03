package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/param"
	geminicli "github.com/yubiquita/gemini-cli-wrapper"
	"google.golang.org/genai"
	"os/exec"
	"strings"
	"time"
)

// ParseProvider parses a string into a Provider (case-insensitive).
func ParseProvider(s string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "gpt", "openai", "gpt3", "gpt3.5", "gpt4":
		return ProviderGPT, nil
	case "gemini", "google":
		return ProviderGemini, nil
	case "geminicli", "gemini_cli", "gemini_wrapper", "gemini-cli", "gemini-wrapper":
		return ProvideGeminiCLI, nil
	case "ollama", "local":
		return ProviderOllama, nil
	case "", "none":
		return ProviderNone, nil
	default:
		return ProviderNone, fmt.Errorf("unknown provider: %s", s)
	}
}

// GPTProvider implements AIProvider for OpenAI.
type GPTProvider struct {
	apiKey      string
	maxTokens   int64
	temperature float64
}

func NewGPTProvider(apiKey string, maxTokens int64, temperature float64) *GPTProvider {
	return &GPTProvider{apiKey: apiKey, maxTokens: maxTokens, temperature: temperature}
}

func (p *GPTProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client := openai.NewClient(option.WithAPIKey(p.apiKey))

	res, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT3_5Turbo,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(userMessage),
		},
		MaxTokens:   param.NewOpt(p.maxTokens),
		Temperature: param.NewOpt(p.temperature),
	})

	if err != nil {
		return "", err
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

func NewGeminiProvider(apiKey string, maxTokens int32, temperature float32) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey, maxTokens: maxTokens, temperature: temperature}
}

func (p *GeminiProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: p.apiKey,
	})
	if err != nil {
		return "", err
	}

	parts := []*genai.Part{
		{
			Text: systemMessage,
		},
		{
			Text: userMessage,
		},
	}
	modelConfig := genai.GenerateContentConfig{Temperature: &p.temperature, MaxOutputTokens: p.maxTokens}

	result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", []*genai.Content{
		{
			Parts: parts,
		},
	}, &modelConfig)
	if err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 {
		return "", ErrNoResponse
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// OllamaProvider implements AIProvider for Ollama.
type OllamaProvider struct {
	apiPath string
}

func NewOllamaProvider(apiPath string) *OllamaProvider {
	return &OllamaProvider{apiPath: apiPath}
}

func (p *OllamaProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error) {
	if p.apiPath == "" {
		return "", ErrOllamaPathMissing
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	prompt := strings.Join([]string{systemMessage, userMessage}, "\n\n")

	cmd := exec.CommandContext(ctx, p.apiPath, "run", "llama3.1:8b", prompt)

	out, err := cmd.CombinedOutput()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("ollama command timed out")
	}

	if err != nil {
		return "", fmt.Errorf("ollama command failed: %v, output: %s", err, string(out))
	}

	return string(out), nil
}

// GeminiCLIProvider implements AIProvider for Gemini CLI Wrapper.
type GeminiCLIProvider struct {
}

func NewGeminiCLIProvider() *GeminiCLIProvider {
	return &GeminiCLIProvider{}
}

func (p *GeminiCLIProvider) GenerateContent(_ context.Context, systemMessage, userMessage string) (string, error) {
	prompt := fmt.Sprintf("System: %s\nUser: %s", systemMessage, userMessage)

	client := geminicli.NewClient()

	resp, err := client.Execute(prompt)
	if err != nil {
		return "", err
	}

	return resp, nil
}
