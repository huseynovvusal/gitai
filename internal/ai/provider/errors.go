package provider

import (
	"errors"
)

var (
	ErrAPIKeyNotSet      = errors.New("API key not set")
	ErrNoResponse        = errors.New("no response from AI provider")
	ErrOllamaPathMissing = errors.New("ollama path not configured")
)
