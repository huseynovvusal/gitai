package provider

import (
	"errors"
)

var (
	ErrAPIKeyNotSet      = errors.New("API key not set")
	ErrNoResponse        = errors.New("no response from AI provider")
	ErrOllamaPathMissing = errors.New("ollama path not configured")
)

type InvalidProviderError struct {
	Provider string
}

func (e *InvalidProviderError) Error() string {
	return "invalid AI provider: " + e.Provider
}
