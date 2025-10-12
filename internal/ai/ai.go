package ai

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	geminicli "github.com/yubiquita/gemini-cli-wrapper"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/packages/param"
	"github.com/spf13/viper"
	"google.golang.org/genai"
)

const temperature = 0.7
const maxToken = 256

func CallGPT(ctx context.Context, systemMessage string, userMessage string, maxTokens int64, temperature float64) (string, error) {
	// Prefer Viper-loaded key (config file, env, flags). Allow legacy OPENAI_API_KEY as fallback.
	apiKey := viper.GetString("ai.api_key")
	if apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	res, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT3_5Turbo,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(userMessage),
		},
		MaxTokens:   param.NewOpt(maxTokens),
		Temperature: param.NewOpt(temperature),
	})

	if err != nil {
		return "", err
	}

	if len(res.Choices) == 0 {
		return "", ErrNoResponse
	}

	return res.Choices[0].Message.Content, nil

}

func CallGemini(ctx context.Context, systemMessage string, userMessage string, maxTokens int32, temperature float32) (string, error) {
	apiKey := viper.GetString("ai.api_key")
	if apiKey == "" {
		return "", ErrAPIKeyNotSet
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
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
	modelConfig := genai.GenerateContentConfig{Temperature: &temperature, MaxOutputTokens: maxTokens}

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

func CallOllama(ctx context.Context, systemMessage string, userMessage string) (string, error) {
	apiPath := viper.GetString("ollama.path")

	if apiPath == "" {
		return "", ErrOllamaPathMissing
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	prompt := strings.Join([]string{systemMessage, userMessage}, "\n\n")

	cmd := exec.CommandContext(ctx, apiPath, "run", "llama3.1:8b", prompt)

	out, err := cmd.CombinedOutput()

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("ollama command timed out")
	}

	if err != nil {
		return "", fmt.Errorf("ollama command failed: %v, output: %s", err, string(out))
	}

	return string(out), nil

}

func CallGeminiCLI(systemMessage, userMessage string) (string, error) {
	prompt := fmt.Sprintf("System: %s\nUser: %s", systemMessage, userMessage)

	client := geminicli.NewClient()

	resp, err := client.Execute(prompt)
	if err != nil {
		return "", err
	}

	return resp, nil
}

type Provider string

const (
	ProviderGPT      Provider = "gpt"
	ProviderGemini   Provider = "gemini"
	ProviderOllama   Provider = "ollama"
	ProvideGeminiCLI Provider = "geminicli"
	ProviderNone     Provider = ""
)

func (p Provider) IsValid() bool {
	switch p {
	case ProviderGPT, ProviderGemini, ProviderOllama, ProviderNone, ProvideGeminiCLI:
		return true
	default:
		return false
	}
}

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

var callGPT = CallGPT
var callGemini = CallGemini
var callOllama = CallOllama
var callGeminiCLI = CallGeminiCLI

func GenerateCommitMessage(ctx context.Context, provider Provider, diff string, status string) (string, error) {
	userMessage := "diff: " + diff + "\n\nstatus: " + status

	switch provider {
	case ProviderGPT:
		return callGPT(ctx, systemMessage, userMessage, maxToken, temperature)
	case ProviderGemini:
		return callGemini(ctx, systemMessage, userMessage, maxToken, temperature)
	case ProviderOllama:
		return callOllama(ctx, systemMessage, userMessage)
	case ProvideGeminiCLI:
		return callGeminiCLI(systemMessage, userMessage)

	default:
		return "", fmt.Errorf("invalid AI provider: %s", provider)
	}
}
