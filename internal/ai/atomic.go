package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"huseynovvusal/gitai/internal/ai/provider"
	"strings"
)

// AtomicCommit represents a suggested split commit.
type AtomicCommit struct {
	Message string `json:"message"`
	HunkIDs []int  `json:"hunk_ids"`
}

var atomicSystemPrompt = `You are an expert developer. Your task is to split a set of diff hunks into separate, atomic, logical commits.

Input format:
Hunk 1 (file.go):
... content ...

Hunk 2 (file.go):
... content ...

Output format:
Return ONLY a valid JSON array of objects. Each object represents a commit and must have:
- "message": A concise commit message (Conventional Commits style).
- "hunk_ids": A list of integer IDs belonging to this commit.

Rules:
1. Every provided Hunk ID must be assigned to exactly one commit.
2. Group related changes (e.g. a function definition and its usage, or a test and its fix).
3. **Order Matters**: The returned list of commits MUST be in a valid dependency order. Commits must be applicable sequentially without breaking the build. For example, a definition must be committed before its usage.
4. Output raw JSON only, no markdown formatting.
5. If a hint is provided, prioritize it for context, but still ensure atomicity.`

// GenerateAtomic generates a list of atomic commits from the provided hunks string.
func (s *Service) GenerateAtomic(ctx context.Context, hunksInput string, hint string) ([]AtomicCommit, provider.Usage, error) {
	userMessage := "Hunks:\n" + hunksInput
	if hint != "" {
		userMessage += "\n\nUser hint: " + hint
	}

	// We do not compress whitespace here because hunks contain code where whitespace is significant.

	resp, usage, err := s.provider.GenerateContent(ctx, atomicSystemPrompt, userMessage)
	if err != nil {
		return nil, provider.Usage{}, fmt.Errorf("failed to generate atomic plan: %w", err)
	}

	// Clean markdown code blocks if present
	cleanResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanResp, "```json") {
		cleanResp = strings.TrimPrefix(cleanResp, "```json")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	} else if strings.HasPrefix(cleanResp, "```") {
		cleanResp = strings.TrimPrefix(cleanResp, "```")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	}
	cleanResp = strings.TrimSpace(cleanResp)

	var commits []AtomicCommit
	if err := json.Unmarshal([]byte(cleanResp), &commits); err != nil {
		return nil, usage, fmt.Errorf("failed to parse AI response: %w\nResponse: %s", err, resp)
	}

	return commits, usage, nil
}
