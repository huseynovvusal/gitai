package cleaner

import (
	"testing"
)

func TestCleanCommitMessage(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		bulletChar string
		expected   string
	}{
		{
			name:       "Markdown Fences",
			input:      "```\nfeat: add login\n```",
			bulletChar: "-",
			expected:   "feat: add login",
		},
		{
			name:       "Markdown Fences with Lang",
			input:      "```markdown\nfeat: add login\n```",
			bulletChar: "-",
			expected:   "feat: add login",
		},
		{
			name:       "Trailing Period Removal",
			input:      "feat: add login.",
			bulletChar: "-",
			expected:   "feat: add login",
		},
		{
			name:       "Missing Blank Line",
			input:      "feat: add login\n- added button",
			bulletChar: "-",
			expected:   "feat: add login\n\n- added button",
		},
		{
			name:       "Bullet Normalization (* to -)",
			input:      "feat: list\n\n* item 1\n* item 2",
			bulletChar: "-",
			expected:   "feat: list\n\n- item 1\n- item 2",
		},
		{
			name:       "Bullet Normalization (• to *)",
			input:      "feat: list\n\n• item 1\n• item 2",
			bulletChar: "*",
			expected:   "feat: list\n\n* item 1\n* item 2",
		},
		{
			name:       "Complex Combined",
			input:      "```\nfeat: complex update.\n• changed api\n• updated docs\n```",
			bulletChar: "-",
			expected:   "feat: complex update\n\n- changed api\n- updated docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanCommitMessage(tt.input, tt.bulletChar)
			if got != tt.expected {
				t.Errorf("CleanCommitMessage() = %q, want %q", got, tt.expected)
			}
		})
	}
}
