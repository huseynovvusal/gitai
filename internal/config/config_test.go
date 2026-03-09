package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfig_Defaults(t *testing.T) {
	v := viper.New()
	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	t.Logf("Viper ai.provider: %v", v.Get("ai.provider"))
	t.Logf("Config AI.Provider: %q", cfg.AI.Provider)

	// Currently it defaults to "" but we want it to default to "gemini"
	if cfg.AI.Provider != "gemini" {
		t.Errorf("expected default provider 'gemini', got %q", cfg.AI.Provider)
	}
	if cfg.AI.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", cfg.AI.Temperature)
	}
	if cfg.AI.MaxTokens != 256 {
		t.Errorf("expected default max tokens 256, got %d", cfg.AI.MaxTokens)
	}
}

func TestParseKeywordsCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Empty string", "", []string{}},
		{"Single keyword", "api_key", []string{"api_key"}},
		{"Multiple keywords", "api_key,password,secret", []string{"api_key", "password", "secret"}},
		{"Keywords with spaces", " api_key , password ", []string{"api_key", "password"}},
		{"Mixed case", "API_KEY,PassWord", []string{"api_key", "password"}},
		{"Trailing comma", "key1,key2,", []string{"key1", "key2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseKeywordsCSV(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("expected %q, got %q", tt.expected[i], result[i])
				}
			}
		})
	}
}
