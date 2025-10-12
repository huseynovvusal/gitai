package ai

import (
	"errors"
	"fmt"
)

var (
	ErrAPIKeyNotSet      = errors.New("API key not set")
	ErrNoResponse        = errors.New("no response from OpenAI")
	ErrOllamaPathMissing = fmt.Errorf("ollama binary not found in PATH")
)
