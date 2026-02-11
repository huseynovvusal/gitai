package ai

import (
	"context"
	"fmt"
	"os"

	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/cleaner"
)

// CommitMessageGenerator defines the interface for generating commit messages.
type CommitMessageGenerator interface {
	Generate(ctx context.Context, diff string, status string, hint string, version string) (string, provider.Usage, error)
}

// Service implements CommitMessageGenerator using an AIProvider.
type Service struct {
	provider    provider.AIProvider
	bulletPoint string
	debugFile   string
}

// NewService creates a new Service.
func NewService(provider provider.AIProvider, bulletPoint string, debugFile string) *Service {
	return &Service{
		provider:    provider,
		bulletPoint: bulletPoint,
		debugFile:   debugFile,
	}
}

// Generate generates a commit message.
func (s *Service) Generate(ctx context.Context, diff string, status string, hint string, version string) (string, provider.Usage, error) {
	userMessage := "diff: " + diff + "\n\nstatus: " + status
	if version != "" {
		userMessage += "\n\nVersion update detected: " + version + "\nInstruction: Follow the 'chore(release)' format mentioned in the system prompt."
	}

	if hint != "" {
		userMessage += "\n\nUser context/instruction: " + hint
	}

	userMessage = compressWhitespace(userMessage)
	systemMessage = compressWhitespace(systemMessage)

	if s.debugFile != "" {
		debugContent := fmt.Sprintf("SYSTEM PROMPT:\n%s\n\nUSER PROMPT:\n%s\n", systemMessage, userMessage)
		_ = os.WriteFile(s.debugFile, []byte(debugContent), 0644)
	}

	msg, usage, err := s.provider.GenerateContent(ctx, systemMessage, userMessage)
	if err != nil {
		return "", provider.Usage{}, fmt.Errorf("failed to generate commit message: %w", err)
	}

	return cleaner.CleanCommitMessage(msg, s.bulletPoint), usage, nil
}
