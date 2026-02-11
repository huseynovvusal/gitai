package ai

import (
	"context"
	"fmt"

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
}

// NewService creates a new Service.
func NewService(provider provider.AIProvider, bulletPoint string) *Service {
	return &Service{
		provider:    provider,
		bulletPoint: bulletPoint,
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

	msg, usage, err := s.provider.GenerateContent(ctx, systemMessage, userMessage)
	if err != nil {
		return "", provider.Usage{}, fmt.Errorf("failed to generate commit message: %w", err)
	}

	return cleaner.CleanCommitMessage(msg, s.bulletPoint), usage, nil
}
