package provider

import (
	"context"
)

// AIProvider defines the interface for underlying AI services.
type AIProvider interface {
	GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error)
}

// NewAIProvider creates a new AIProvider based on the provider type and configuration.
func NewAIProvider(p Provider, cfg Config) (AIProvider, error) {
	switch p {
	case ProviderGPT:
		return NewGPTProvider(cfg.APIKey, cfg.BaseUrl, cfg.MaxTokens, cfg.Temperature, cfg.Model), nil
	case ProviderGemini:
		return NewGeminiProvider(cfg.APIKey, int32(cfg.MaxTokens), float32(cfg.Temperature)), nil
	case ProviderOllama:
		return NewOllamaProvider(cfg.OllamaPath), nil
	case ProvideGeminiCLI:
		return NewGeminiCLIProvider(), nil
	case ProviderAnthropic:
		return NewAnthropicProvider(cfg.APIKey, int(cfg.MaxTokens), cfg.Temperature), nil
	case ProviderGroq:
		baseUrl := cfg.BaseUrl
		if baseUrl == "" {
			baseUrl = "https://api.groq.com/openai/v1"
		}
		model := cfg.Model
		if model == "" {
			model = "llama3-70b-8192"
		}
		return NewGPTProvider(cfg.APIKey, baseUrl, cfg.MaxTokens, cfg.Temperature, model), nil
	case ProviderDeepSeek:
		baseUrl := cfg.BaseUrl
		if baseUrl == "" {
			baseUrl = "https://api.deepseek.com"
		}
		model := cfg.Model
		if model == "" {
			model = "deepseek-chat"
		}
		return NewGPTProvider(cfg.APIKey, baseUrl, cfg.MaxTokens, cfg.Temperature, model), nil
	case ProviderXAI:
		baseUrl := cfg.BaseUrl
		if baseUrl == "" {
			baseUrl = "https://api.x.ai/v1"
		}
		model := cfg.Model
		if model == "" {
			model = "grok-beta"
		}
		return NewGPTProvider(cfg.APIKey, baseUrl, cfg.MaxTokens, cfg.Temperature, model), nil
	default:
		return nil, &InvalidProviderError{Provider: string(p)}
	}
}
