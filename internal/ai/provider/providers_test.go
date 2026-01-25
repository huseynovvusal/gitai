package provider

import (
	"testing"
)

func TestNewGeminiProvider(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		expectedModel string
	}{
		{
			name:          "With specific model",
			model:         "gemini-1.5-pro",
			expectedModel: "gemini-1.5-pro",
		},
		{
			name:          "With empty model (default)",
			model:         "",
			expectedModel: "gemini-2.0-flash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGeminiProvider("key", 100, 0.5, tt.model)
			if p.model != tt.expectedModel {
				t.Errorf("NewGeminiProvider() model = %v, want %v", p.model, tt.expectedModel)
			}
		})
	}
}

func TestNewOllamaProvider(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		expectedModel string
	}{
		{
			name:          "With specific model",
			model:         "mistral",
			expectedModel: "mistral",
		},
		{
			name:          "With empty model (default)",
			model:         "",
			expectedModel: "llama3.1:8b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewOllamaProvider("http://localhost:11434", tt.model)
			if p.model != tt.expectedModel {
				t.Errorf("NewOllamaProvider() model = %v, want %v", p.model, tt.expectedModel)
			}
		})
	}
}

func TestNewAnthropicProvider(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		expectedModel string
	}{
		{
			name:          "With specific model",
			model:         "claude-3-opus",
			expectedModel: "claude-3-opus",
		},
		{
			name:          "With empty model (default)",
			model:         "",
			expectedModel: "claude-3-5-sonnet-20240620",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewAnthropicProvider("key", 100, 0.5, tt.model)
			if p.model != tt.expectedModel {
				t.Errorf("NewAnthropicProvider() model = %v, want %v", p.model, tt.expectedModel)
			}
		})
	}
}
