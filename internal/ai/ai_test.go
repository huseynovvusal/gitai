package ai

import (
	"context"
	_ "embed"
	"errors"
	"testing"

	"github.com/pkoukk/tiktoken-go"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/ai/test_prompts"
)

// MockProvider is a mock implementation of AIProvider.
type MockProvider struct {
	GenerateContentFunc func(ctx context.Context, systemMessage, userMessage string) (string, provider.Usage, error)
}

func (m *MockProvider) GenerateContent(ctx context.Context, systemMessage, userMessage string) (string, provider.Usage, error) {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, systemMessage, userMessage)
	}

	return "", provider.Usage{}, nil
}

func (m *MockProvider) StreamContent(_ context.Context, _, _ string) (<-chan provider.StreamResult, error) {
	return nil, errors.New("not supported")
}

// Test that errors from provider propagate (e.g., ErrNoResponse).
func TestService_Generate_PropagatesError(t *testing.T) {
	mockProvider := &MockProvider{
		GenerateContentFunc: func(ctx context.Context, systemMessage, userMessage string) (string, provider.Usage, error) {
			return "", provider.Usage{}, provider.ErrNoResponse
		},
	}

	service := NewService(mockProvider, "-", "")

	_, _, err := service.Generate(context.Background(), "diff", "status", "", "1.0.0")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, provider.ErrNoResponse) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// This is more of a blank test that lists the different prompts and allows to iterate over different versions,
// It logs the cost of the tokens used by each candidate prompt, and accumulates the total.
// The purpose is not to validate the correctness of outputs, but to compare prompt formulations,
// track their relative token lengths, and help optimize prompt design for cost and efficiency.
func TestPromptIterations_TokenCounts(t *testing.T) {
	enc, err := tiktoken.GetEncoding("o200k_base")
	if err != nil {
		t.Skipf("skipping: tokenizer unavailable (requires network): %v", err)
	}

	tests := []struct {
		name       string
		candidates []string
	}{
		{
			name: "Test system prompt",
			candidates: []string{
				systemMessage,
				"Expert Git generator. Summarize diff/status to a single, scoped, conventional commit. Use: <type>(scope): <desc>. Body: dot list of non-trivial changes. Add BREAKING CHANGE footer if applicable. Output ONLY message.",
				"Role: You expert Git message generator. Summarize the intent and scope of the following diff/status into a conventional, professional commit message with a single scope\nExpected Output: A single commit message in the Conventional Commits format:\n<type>[**single, highest-level component**]: <description>\n[Body, as dot list of non-trivial changes]\n[optional footer(s), include BREAKING CHANGES if necessary ]",
				"You are a highly skilled software engineer with deep expertise in crafting precise, professional, and conventional git commit messages. Given a git diff and status, generate a single, clear, and accurate commit message that succinctly summarizes the intent and scope of the changes. Only output the commit message itself, with no explanations, prefixes, formatting, or any other text. The output must be ready to use as a commit message and strictly adhere to best practices.",
			},
		},
		{
			name: "Shortest usermessage",
			candidates: []string{
				compressWhitespace(test_prompts.UserMessageCompressedDiff),
				test_prompts.UserMessageCompressedDiff,
				compressWhitespace(test_prompts.UserMessage),
				test_prompts.UserMessage,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var totalTokens int

			for i, s := range tt.candidates {
				tokens := len(enc.Encode(s, nil, nil))
				totalTokens += tokens

				t.Logf("  Candidate %d: %d tokens", i, tokens)
			}
		})
	}
}
