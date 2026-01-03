package ai

import (
	"context"
	"github.com/spf13/viper"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/cleaner"
)

// CommitMessageGenerator defines the interface for generating commit messages.
type CommitMessageGenerator interface {
	Generate(ctx context.Context, diff string, status string, hint string) (string, error)
}

// Service implements CommitMessageGenerator using an AIProvider.
type Service struct {
	provider provider.AIProvider
}

// NewService creates a new Service.
func NewService(provider provider.AIProvider) *Service {
	return &Service{provider: provider}
}

// Generate generates a commit message.
func (s *Service) Generate(ctx context.Context, diff string, status string, hint string) (string, error) {
	userMessage := "diff: " + diff + "\n\nstatus: " + status
	if hint != "" {
		userMessage += "\n\nUser context/instruction: " + hint
	}
	userMessage = compressWhitespace(userMessage)
	systemMessage = compressWhitespace(systemMessage)
	msg, err := s.provider.GenerateContent(ctx, systemMessage, userMessage)
	if err != nil {
		return "", err
	}

	bullet := viper.GetString("suggest.bullet_point")
	return cleaner.CleanCommitMessage(msg, bullet), nil
}
