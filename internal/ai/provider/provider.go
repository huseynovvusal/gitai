package provider

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

type Config struct {
	APIKey      string
	BaseUrl     string
	MaxTokens   int64
	Temperature float64
	OllamaPath  string
}
