package git

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestFormatStatusCode(t *testing.T) {
	tests := []struct {
		code     git.StatusCode
		expected rune
	}{
		{git.Unmodified, ' '},
		{git.Modified, 'M'},
		{git.Added, 'A'},
		{git.Deleted, 'D'},
		{git.Renamed, 'R'},
		{git.Copied, 'C'},
		{git.Untracked, '?'},
		{git.StatusCode('X'), '?'}, // Default case
	}

	for _, tt := range tests {
		result := formatStatusCode(tt.code)
		if result != tt.expected {
			t.Errorf("formatStatusCode(%v) = %c; want %c", tt.code, result, tt.expected)
		}
	}
}

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/artback/gitai.git", "https://github.com/artback/gitai"},
		{"git@github.com:artback/gitai.git", "https://github.com/artback/gitai"},
		{"git@github.com:artback/gitai", "https://github.com/artback/gitai"},
		{"ssh://git@github.com/artback/gitai.git", "https://github.com/artback/gitai"},
		{"  https://github.com/artback/gitai  ", "https://github.com/artback/gitai"},
		{"github.com/artback/gitai", "https://github.com/artback/gitai"},
	}

	for _, tt := range tests {
		result := normalizeGitURL(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeGitURL(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestResolveAuth(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantsNil bool
	}{
		{"HTTPS URL", "https://github.com/artback/gitai", true},
		{"HTTP URL", "http://github.com/artback/gitai", true},
		{"SSH URL (git@)", "git@github.com:artback/gitai.git", false},
		{"SSH URL (ssh://)", "ssh://git@github.com/artback/gitai.git", false},
		{"SSH URL (custom user)", "ssh://custom@github.com/artback/gitai.git", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveAuth(tt.url)
			if tt.wantsNil {
				if result != nil {
					t.Errorf("resolveAuth(%q) expected nil, got %v", tt.url, result)
				}
				if err != nil {
					t.Errorf("resolveAuth(%q) expected no error, got %v", tt.url, err)
				}
			} else {
				if err != nil {
					t.Logf("resolveAuth(%q) failed: %v", tt.url, err)
				} else if result == nil {
					t.Logf("resolveAuth(%q) returned nil auth", tt.url)
				} else {
					t.Logf("resolveAuth(%q) result: %s", tt.url, result.String())
				}
			}
		})
	}
}

func TestGenerateDiffString(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		oldText   string
		newText   string
		isNew     bool
		isDeleted bool
		contains  []string
	}{
		{
			name:      "New file",
			path:      "main.go",
			oldText:   "",
			newText:   "package main",
			isNew:     true,
			isDeleted: false,
			contains:  []string{"diff --git a/main.go b/main.go", "new file mode 100644", "+++ b/main.go", "package main"},
		},
		{
			name:      "Modified file",
			path:      "README.md",
			oldText:   "Hello",
			newText:   "Hello World",
			isNew:     false,
			isDeleted: false,
			contains:  []string{"diff --git a/README.md b/README.md", "--- a/README.md", "+++ b/README.md", "Hello", "World"},
		},
		{
			name:      "Deleted file",
			path:      "old.txt",
			oldText:   "bye",
			newText:   "",
			isNew:     false,
			isDeleted: true,
			contains:  []string{"diff --git a/old.txt b/old.txt", "deleted file mode 100644", "--- a/old.txt", "+++ /dev/null"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateDiffString(tt.path, tt.oldText, tt.newText, tt.isNew, tt.isDeleted)
			for _, search := range tt.contains {
				if !strings.Contains(result, search) {
					t.Errorf("generateDiffString result missing %q\nResult:\n%s", search, result)
				}
			}
		})
	}
}
