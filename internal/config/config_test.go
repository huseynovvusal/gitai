package config

import (
	"testing"
)

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
