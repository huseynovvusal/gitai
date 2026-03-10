package ai

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"huseynovvusal/gitai/internal/ai/provider"
)

//go:embed review_prompt.md
var reviewSystemMessage string

// Finding represents a single code review finding from the AI.
type Finding struct {
	Severity    string `json:"severity"`
	File        string `json:"file"`
	Line        string `json:"line"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// ReviewResult holds the parsed review output.
type ReviewResult struct {
	Findings []Finding
}

// CodeReviewer defines the interface for generating code reviews.
type CodeReviewer interface {
	Review(ctx context.Context, diff string, hint string) (*ReviewResult, error)
}

// ReviewService implements CodeReviewer using an AIProvider.
type ReviewService struct {
	provider provider.AIProvider
}

// NewReviewService creates a new ReviewService.
func NewReviewService(provider provider.AIProvider) *ReviewService {
	return &ReviewService{provider: provider}
}

// Review generates a code review for the given diff.
func (s *ReviewService) Review(ctx context.Context, diff string, hint string) (*ReviewResult, error) {
	userMessage := "diff:\n" + diff

	if hint != "" {
		userMessage += "\n\nReview focus: " + hint
	}

	userMessage = compressWhitespace(userMessage)
	sysMsg := compressWhitespace(reviewSystemMessage)

	resp, err := s.provider.GenerateContent(ctx, sysMsg, userMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate review: %w", err)
	}

	findings, err := parseFindings(resp)
	if err != nil {
		return nil, err
	}

	return &ReviewResult{Findings: findings}, nil
}

// parseFindings extracts the JSON array of findings from the AI response.
func parseFindings(resp string) ([]Finding, error) {
	cleanResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanResp, "```json") {
		cleanResp = strings.TrimPrefix(cleanResp, "```json")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	} else if strings.HasPrefix(cleanResp, "```") {
		cleanResp = strings.TrimPrefix(cleanResp, "```")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	}
	cleanResp = strings.TrimSpace(cleanResp)

	var findings []Finding
	if err := json.Unmarshal([]byte(cleanResp), &findings); err != nil {
		return nil, fmt.Errorf("failed to parse review response: %w\nResponse: %s", err, resp)
	}

	return findings, nil
}

// Summary returns a counts summary string for the review result.
func (r *ReviewResult) Summary() (critical, warnings, infos int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case "critical":
			critical++
		case "warning":
			warnings++
		case "info":
			infos++
		}
	}
	return
}
