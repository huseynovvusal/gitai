package provider

import (
	"context"
	"fmt"
)

// AIProvider defines the interface for underlying AI services.
type AIProvider interface {
	GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error)
}

// NewAIProvider creates a new AIProvider based on the provider type and configuration.
func NewAIProvider(p Provider, cfg Config) (AIProvider, error) {
	switch p {
	case ProviderGPT:
		return NewGPTProvider(cfg.APIKey, cfg.MaxTokens, cfg.Temperature), nil
	case ProviderGemini:
		return NewGeminiProvider(cfg.APIKey, int32(cfg.MaxTokens), float32(cfg.Temperature)), nil
	case ProviderOllama:
		return NewOllamaProvider(cfg.OllamaPath), nil
	case ProvideGeminiCLI:
		return NewGeminiCLIProvider(), nil
	default:
		return nil, fmt.Errorf("invalid AI provider: %s", p)
	}
}
