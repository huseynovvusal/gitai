package suggest

import (
	"strings"
	"testing"
)

func TestHintProcessors(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		processor    HintProcessor
		wantContains []string
	}{
		{
			name:         "Jira Processor - Empty hint",
			input:        "",
			processor:    JiraHintProcessor,
			wantContains: []string{},
		},
		{
			name:         "Jira Processor - No match",
			input:        "fix login bug",
			processor:    JiraHintProcessor,
			wantContains: []string{"fix login bug"},
		},
		{
			name:      "Jira Processor - Match",
			input:     "https://jira.example.com/browse/PROJ-123",
			processor: JiraHintProcessor,
			wantContains: []string{
				"https://jira.example.com/browse/PROJ-123",
				"Ticket: PROJ-123",
				"The commit message header must be in the format: <type>(<scope>): PROJ-123 <description>",
			},
		},
		{
			name:         "GitHub Processor - Empty hint",
			input:        "",
			processor:    GitHubHintProcessor,
			wantContains: []string{},
		},
		{
			name:         "GitHub Processor - No match",
			input:        "fix login bug",
			processor:    GitHubHintProcessor,
			wantContains: []string{"fix login bug"},
		},
		{
			name:      "GitHub Processor - Match",
			input:     "https://github.com/owner/repo/issues/456",
			processor: GitHubHintProcessor,
			wantContains: []string{
				"https://github.com/owner/repo/issues/456",
				"Ticket: #456",
				"The commit message header must be in the format: <type>(<scope>): #456 <description>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.processor(tt.input)
			if tt.input == "" && got != "" {
				t.Errorf("Processor(%q) = %q, want empty string", tt.input, got)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Processor(%q) = %q, missing substring %q", tt.input, got, want)
				}
			}
		})
	}
}
