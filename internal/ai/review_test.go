package ai

import (
	"context"
	"errors"
	"testing"

	"huseynovvusal/gitai/internal/ai/provider"
)

type mockReviewProvider struct {
	response string
	err      error
}

func (m *mockReviewProvider) GenerateContent(_ context.Context, _, _ string) (string, provider.Usage, error) {
	return m.response, provider.Usage{}, m.err
}

func (m *mockReviewProvider) StreamContent(_ context.Context, _, _ string) (<-chan provider.StreamResult, error) {
	return nil, errors.New("not supported")
}

func TestReviewService_Review(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		hint         string
		wantFindings int
		wantCritical int
		wantWarnings int
		wantInfos    int
		wantErr      bool
	}{
		{
			name:         "valid findings",
			response:     `[{"severity":"critical","file":"auth.go","line":"42","description":"SQL injection","suggestion":"Use parameterized queries"},{"severity":"warning","file":"auth.go","line":"50","description":"Unused error","suggestion":"Handle the error"}]`,
			wantFindings: 2,
			wantCritical: 1,
			wantWarnings: 1,
			wantInfos:    0,
		},
		{
			name:         "empty findings",
			response:     `[]`,
			wantFindings: 0,
		},
		{
			name:         "markdown wrapped json",
			response:     "```json\n[{\"severity\":\"info\",\"file\":\"main.go\",\"line\":\"10\",\"description\":\"Style issue\",\"suggestion\":\"Rename variable\"}]\n```",
			wantFindings: 1,
			wantInfos:    1,
		},
		{
			name:     "invalid json response",
			response: "This is not JSON",
			wantErr:  true,
		},
		{
			name:         "with hint",
			response:     `[{"severity":"critical","file":"handler.go","line":"23","description":"XSS vulnerability","suggestion":"Escape output"}]`,
			hint:         "check for XSS",
			wantFindings: 1,
			wantCritical: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockReviewProvider{response: tt.response}
			service := NewReviewService(provider)

			result, err := service.Review(context.Background(), "test diff", tt.hint)
			if (err != nil) != tt.wantErr {
				t.Errorf("Review() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(result.Findings) != tt.wantFindings {
				t.Errorf("Review() findings count = %d, want %d", len(result.Findings), tt.wantFindings)
			}

			critical, warnings, infos := result.Summary()
			if critical != tt.wantCritical {
				t.Errorf("Summary() critical = %d, want %d", critical, tt.wantCritical)
			}
			if warnings != tt.wantWarnings {
				t.Errorf("Summary() warnings = %d, want %d", warnings, tt.wantWarnings)
			}
			if infos != tt.wantInfos {
				t.Errorf("Summary() infos = %d, want %d", infos, tt.wantInfos)
			}
		})
	}
}

func TestParseFindings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name:    "plain json array",
			input:   `[{"severity":"warning","file":"a.go","line":"1","description":"test","suggestion":"fix"}]`,
			wantLen: 1,
		},
		{
			name:    "json with code block",
			input:   "```json\n[]\n```",
			wantLen: 0,
		},
		{
			name:    "bare code block",
			input:   "```\n[{\"severity\":\"info\",\"file\":\"b.go\",\"line\":\"5\",\"description\":\"d\",\"suggestion\":\"s\"}]\n```",
			wantLen: 1,
		},
		{
			name:    "invalid json",
			input:   "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := parseFindings(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFindings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(findings) != tt.wantLen {
				t.Errorf("parseFindings() len = %d, want %d", len(findings), tt.wantLen)
			}
		})
	}
}
