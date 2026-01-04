package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"huseynovvusal/gitai/internal/git"
)

type Config struct {
	AI       AIConfig
	Suggest  SuggestConfig
	Security SecurityConfig
	Ollama   OllamaConfig
}

type AIConfig struct {
	Provider    string
	APIKey      string
	Temperature float64
	MaxTokens   int64
}

type SuggestConfig struct {
	Editor      string
	BulletPoint string
	Hint        string
	NoHint      bool
}

type SecurityConfig struct {
	Keywords []string
}

type OllamaConfig struct {
	Path string
}

func LoadConfig(v *viper.Viper) (*Config, error) {
	v.SetConfigName("gitai")

	v.AddConfigPath("/etc/gitai/")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "gitai"))
		v.AddConfigPath(filepath.Join(home, ".gitai"))
	}
	if gitRoot, err := git.GetGitRoot(); err == nil {
		v.AddConfigPath(gitRoot)
	}
	v.AddConfigPath(".")

	v.SetEnvPrefix("gitai")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bindings
	_ = v.BindEnv("ollama.path", "OLLAMA_API_PATH")
	_ = v.BindEnv("ai.api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("ai.api_key", "GEMINI_API_KEY")
	_ = v.BindEnv("ai.api_key", "GOOGLE_API_KEY")
	_ = v.BindEnv("ai.api_key", "GITAI_API_KEY")
	_ = v.BindEnv("security.keywords", "GITAI_SENSITIVE_KEYWORDS")

	// Defaults
	v.SetDefault("security.keywords", []string{
		"password", "passwd", "pwd", "secret", "api_key", "apikey",
		"access_token", "private_key", "ssh-rsa", "begin private key",
		"aws_access_key_id", "aws_secret_access_key", "client_secret",
		"jwt", "encryption_key",
	})
	v.SetDefault("suggest.bullet_point", "-")
	v.SetDefault("ai.temperature", 0.7)
	v.SetDefault("ai.max_tokens", 256)
	v.SetDefault("suggest.editor", "system")

	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Post-process security keywords to handle CSV in environment variables
	cfg.Security.Keywords = v.GetStringSlice("security.keywords")
	if len(cfg.Security.Keywords) == 1 && strings.Contains(cfg.Security.Keywords[0], ",") {
		cfg.Security.Keywords = ParseKeywordsCSV(cfg.Security.Keywords[0])
	}

	return &cfg, nil
}

func ParseKeywordsCSV(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
