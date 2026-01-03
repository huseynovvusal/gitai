package provider

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
)

// AIProvider defines the interface for underlying AI services.
type AIProvider interface {
	GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, error)
}

// NewAIProvider creates a new AIProvider based on the provider type.
func NewAIProvider(p Provider) (AIProvider, error) {
	apiKey := viper.GetString("ai.api_key")
	maxTokens := viper.GetInt64("ai.max_tokens")
	temperature := viper.GetFloat64("ai.temperature")

	switch p {
	case ProviderGPT:
		return NewGPTProvider(apiKey, maxTokens, temperature), nil
	case ProviderGemini:
		return NewGeminiProvider(apiKey, int32(maxTokens), float32(temperature)), nil
	case ProviderOllama:
		apiPath := viper.GetString("ollama.path")
		return NewOllamaProvider(apiPath), nil
	case ProvideGeminiCLI:
		return NewGeminiCLIProvider(), nil
	default:
		return nil, fmt.Errorf("invalid AI provider: %s", p)
	}
}
