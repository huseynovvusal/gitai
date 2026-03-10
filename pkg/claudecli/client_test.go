package claudecli

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c.timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, c.timeout)
	}
	if c.model != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, c.model)
	}
}

func TestNewClientWithConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          Config
		expectedTimeout time.Duration
		expectedModel   string
	}{
		{
			name:            "defaults",
			config:          Config{},
			expectedTimeout: DefaultTimeout,
			expectedModel:   DefaultModel,
		},
		{
			name:            "custom model",
			config:          Config{Model: "opus"},
			expectedTimeout: DefaultTimeout,
			expectedModel:   "opus",
		},
		{
			name:            "custom timeout",
			config:          Config{Timeout: 90 * time.Second},
			expectedTimeout: 90 * time.Second,
			expectedModel:   DefaultModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClientWithConfig(tt.config)
			if c.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, c.timeout)
			}
			if c.model != tt.expectedModel {
				t.Errorf("expected model %q, got %q", tt.expectedModel, c.model)
			}
		})
	}
}

func TestExecuteEmptyPrompt(t *testing.T) {
	c := NewClient()
	_, err := c.Execute("")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestExecuteCommandNotFound(t *testing.T) {
	// Override PATH to ensure claude is not found
	t.Setenv("PATH", "")
	c := NewClient()
	_, err := c.Execute("test prompt")
	if err == nil {
		t.Fatal("expected error when claude is not in PATH")
	}
}

func TestValidateAvailable(t *testing.T) {
	// Just verify it returns an error when not in PATH
	t.Setenv("PATH", "")
	err := ValidateAvailable()
	if err == nil {
		t.Fatal("expected error when claude is not in PATH")
	}
}
