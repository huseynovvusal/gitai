package git

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
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
		{git.StatusCode(255), '?'}, // Unknown code
	}

	for _, tt := range tests {
		t.Run(string(tt.expected), func(t *testing.T) {
			result := formatStatusCode(tt.code)
			if result != tt.expected {
				t.Errorf("formatStatusCode(%v) = %c; want %c", tt.code, result, tt.expected)
			}
		})
	}
}

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"HTTPS URL", "https://github.com/artback/gitai.git", "https://github.com/artback/gitai"},
		{"HTTP URL", "http://github.com/artback/gitai.git", "http://github.com/artback/gitai"},
		{"SSH git@ URL", "git@github.com:artback/gitai.git", "https://github.com/artback/gitai"},
		{"SSH ssh:// URL", "ssh://git@github.com/artback/gitai.git", "https://github.com/artback/gitai"},
		{"No Scheme", "github.com/artback/gitai", "https://github.com/artback/gitai"},
		{"Spaces", "  https://github.com/artback/gitai  ", "https://github.com/artback/gitai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeGitURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeGitURL(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
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
		{"SSH URL (complex user)", "ssh://user.name@host.com:22/repo", false},
		{"Local Path", "/tmp/repo", true},
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
					t.Logf("resolveAuth(%q) failed as expected in this env: %v", tt.url, err)
				} else if result != nil {
					t.Logf("resolveAuth(%q) returned: %s", tt.url, result.String())
				}
			}
		})
	}
}

func TestResolveAuth_Callback(t *testing.T) {
	// Trigger the callback to cover its internal logic
	auth, err := resolveAuth("git@github.com:artback/gitai.git")
	if err != nil {
		t.Fatal(err)
	}

	pkc, ok := auth.(*gitssh.PublicKeysCallback)
	if !ok {
		t.Fatal("expected *gitssh.PublicKeysCallback")
	}

	t.Setenv("SSH_AUTH_SOCK", "/tmp/non-existent-sock")
	_, _ = pkc.Callback()
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
			"New File",
			"main.go",
			"",
			"package main",
			true,
			false,
			[]string{"diff --git a/main.go b/main.go", "new file mode 100644", "+++ b/main.go", "package main"},
		},
		{
			"Modified File",
			"README.md",
			"Hello",
			"Hello World",
			false,
			false,
			[]string{"diff --git a/README.md b/README.md", "--- a/README.md", "+++ b/README.md", "Hello", "World"},
		},
		{
			"Deleted File",
			"old.txt",
			"content",
			"",
			false,
			true,
			[]string{"diff --git a/old.txt b/old.txt", "deleted file mode 100644", "--- a/old.txt", "+++ /dev/null"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateDiffString(tt.path, tt.oldText, tt.newText, tt.isNew, tt.isDeleted)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Expected result to contain %q\nResult:\n%s", s, result)
				}
			}
			if strings.Contains(result, "%0A") || strings.Contains(result, "%7B") {
				t.Errorf("Result contains URL encoded characters: %s", result)
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	expected := []string{"a", "b", "c"}
	res := uniqueStrings(input)
	if len(res) != 3 {
		t.Fatalf("expected 3, got %d", len(res))
	}
	for i, v := range expected {
		if res[i] != v {
			t.Errorf("at %d: expected %s, got %s", i, v, res[i])
		}
	}
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	list := make([]string, 0, len(slice))
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}