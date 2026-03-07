package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/cleaner"
)

// CommitMessageGenerator defines the interface for generating commit messages.
type CommitMessageGenerator interface {
	Generate(ctx context.Context, diff string, status string, hint string, version string) (string, provider.Usage, error)
	Stream(ctx context.Context, diff string, status string, hint string, version string) (<-chan provider.StreamResult, error)
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

func (s *Service) buildUserMessage(diff string, status string, hint string, version string) string {
	userMessage := "diff: " + diff + "\n\nstatus: " + status
	if version != "" {
		userMessage += "\n\nVersion update detected: " + version + "\nInstruction: Follow the 'chore(release)' format mentioned in the system prompt."
	}

	if hint != "" {
		userMessage += "\n\nUser context/instruction: " + hint
	}
	return compressWhitespace(userMessage)
}

func (s *Service) BulletPoint() string {
	return s.bulletPoint
}

// Generate generates a commit message.
func (s *Service) Generate(ctx context.Context, diff string, status string, hint string, version string) (string, provider.Usage, error) {
	userMessage := s.buildUserMessage(diff, status, hint, version)
	sysMsg := compressWhitespace(systemMessage)

	if s.debugFile != "" {
		debugContent := fmt.Sprintf("SYSTEM PROMPT:\n%s\n\nUSER PROMPT:\n%s\n", sysMsg, userMessage)
		_ = os.WriteFile(s.debugFile, []byte(debugContent), 0644)
	}

	msg, usage, err := s.provider.GenerateContent(ctx, sysMsg, userMessage)
	if err != nil {
		return "", provider.Usage{}, fmt.Errorf("failed to generate commit message: %w", err)
	}

	return cleaner.CleanCommitMessage(msg, s.bulletPoint), usage, nil
}

// Stream generates a commit message stream.
func (s *Service) Stream(ctx context.Context, diff string, status string, hint string, version string) (<-chan provider.StreamResult, error) {
	userMessage := s.buildUserMessage(diff, status, hint, version)
	sysMsg := compressWhitespace(systemMessage)

	if s.debugFile != "" {
		debugContent := fmt.Sprintf("SYSTEM PROMPT:\n%s\n\nUSER PROMPT:\n%s\n", sysMsg, userMessage)
		_ = os.WriteFile(s.debugFile, []byte(debugContent), 0644)
	}

	stream, err := s.provider.StreamContent(ctx, sysMsg, userMessage)
	if err != nil {
		return nil, err
	}

	out := make(chan provider.StreamResult)
	go func() {
		defer close(out)
		var fullMessage strings.Builder
		for res := range stream {
			if res.Err != nil {
				out <- res
				return
			}
			fullMessage.WriteString(res.Token)
			// For streaming, we might not want to clean every single token if it depends on context,
			// but commit messages are simple. Let's just pass it through and maybe clean at the end or
			// just pass tokens.
			out <- res
		}
	}()

	return out, nil
}
