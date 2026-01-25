package provider

type Provider string

const (
	ProviderGPT      Provider = "gpt"
	ProviderGemini   Provider = "gemini"
	ProviderOllama   Provider = "ollama"
	ProvideGeminiCLI Provider = "geminicli"
	ProviderAnthropic Provider = "anthropic"
	ProviderGroq      Provider = "groq"
	ProviderDeepSeek  Provider = "deepseek"
	ProviderXAI       Provider = "xai"
	ProviderNone     Provider = ""
)

func (p Provider) IsValid() bool {
	switch p {
	case ProviderGPT, ProviderGemini, ProviderOllama, ProviderNone, ProvideGeminiCLI, ProviderAnthropic, ProviderGroq, ProviderDeepSeek, ProviderXAI:
		return true
	default:
		return false
	}
}

type Config struct {
	APIKey      string
	BaseUrl     string
	Model       string
	MaxTokens   int64
	Temperature float64
	OllamaPath  string
}
