package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveSection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "removes section from middle",
			content:  "before\n" + markerBegin + "\nstuff\n" + markerEnd + "\nafter\n",
			expected: "beforeafter\n",
		},
		{
			name:     "removes section at end",
			content:  "before\n" + markerBegin + "\nstuff\n" + markerEnd + "\n",
			expected: "before",
		},
		{
			name:     "no section present",
			content:  "just content\n",
			expected: "just content\n",
		},
		{
			name:     "only section",
			content:  markerBegin + "\nstuff\n" + markerEnd + "\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeSection(tt.content, markerBegin, markerEnd)
			if got != tt.expected {
				t.Errorf("removeSection() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHookScript(t *testing.T) {
	script := hookScript()

	if len(script) == 0 {
		t.Fatal("hookScript() returned empty string")
	}

	// Must contain markers.
	if got := script; !contains(got, markerBegin) || !contains(got, markerEnd) {
		t.Errorf("hookScript() missing markers")
	}

	// Must check GITAI_SKIP_HOOK.
	if got := script; !contains(got, "GITAI_SKIP_HOOK") {
		t.Errorf("hookScript() missing GITAI_SKIP_HOOK check")
	}

	// Must check COMMIT_SOURCE.
	if got := script; !contains(got, "COMMIT_SOURCE") {
		t.Errorf("hookScript() missing COMMIT_SOURCE check")
	}
}

func TestInstallAndUninstall(t *testing.T) {
	// Create a temp directory simulating a git repo.
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDir, hookFileName)

	// Test install into fresh hook.
	err := installTo(hookPath)
	if err != nil {
		t.Fatalf("installTo() error: %v", err)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("failed to read hook: %v", err)
	}
	content := string(data)

	if !contains(content, "#!/bin/sh") {
		t.Error("installed hook missing shebang")
	}
	if !contains(content, markerBegin) {
		t.Error("installed hook missing begin marker")
	}
	if !contains(content, markerEnd) {
		t.Error("installed hook missing end marker")
	}

	// Check file is executable.
	info, _ := os.Stat(hookPath)
	if info.Mode()&0o111 == 0 {
		t.Error("hook file is not executable")
	}

	// Test double install fails.
	err = installTo(hookPath)
	if err == nil {
		t.Error("expected error on double install")
	}

	// Test uninstall.
	err = uninstallFrom(hookPath)
	if err != nil {
		t.Fatalf("uninstallFrom() error: %v", err)
	}

	// File should be removed since only shebang + hook content existed.
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Error("hook file should have been removed after uninstall")
	}
}

func TestInstallPreservesExistingHook(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(hooksDir, hookFileName)

	// Write an existing hook.
	existing := "#!/bin/sh\necho 'existing hook'\n"
	if err := os.WriteFile(hookPath, []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}

	// Install gitai hook.
	if err := installTo(hookPath); err != nil {
		t.Fatalf("installTo() error: %v", err)
	}

	data, _ := os.ReadFile(hookPath)
	content := string(data)

	if !contains(content, "existing hook") {
		t.Error("existing hook content was lost")
	}
	if !contains(content, markerBegin) {
		t.Error("gitai hook not appended")
	}

	// Uninstall should preserve existing content.
	if err := uninstallFrom(hookPath); err != nil {
		t.Fatalf("uninstallFrom() error: %v", err)
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal("hook file should still exist after uninstall")
	}
	content = string(data)

	if !contains(content, "existing hook") {
		t.Error("existing hook content was lost after uninstall")
	}
	if contains(content, markerBegin) {
		t.Error("gitai marker should have been removed")
	}
}

func TestUninstallWithoutInstall(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, hookFileName)

	err := uninstallFrom(hookPath)
	if err == nil {
		t.Error("expected error when uninstalling without install")
	}
}

// installTo and uninstallFrom are test helpers that operate on a specific path
// instead of discovering the git root.
func installTo(hookPath string) error {
	existing, err := readFileIfExists(hookPath)
	if err != nil {
		return err
	}

	if contains(existing, markerBegin) {
		return errAlreadyInstalled
	}

	content := existing
	if content == "" {
		content = "#!/bin/sh\n"
	}
	content += "\n" + hookScript()

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	if err := os.WriteFile(hookPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("failed to write hook file: %w", err)
	}

	return nil
}

func uninstallFrom(hookPath string) error {
	existing, err := readFileIfExists(hookPath)
	if err != nil {
		return err
	}

	if !contains(existing, markerBegin) {
		return errNotInstalled
	}

	cleaned := removeSection(existing, markerBegin, markerEnd)
	cleaned = trimRight(cleaned) + "\n"

	trimmed := trim(cleaned)
	if trimmed == "#!/bin/sh" || trimmed == "#!/bin/bash" || trimmed == "" {
		if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("failed to remove hook file: %w", err)
		}

		return nil
	}

	if err := os.WriteFile(hookPath, []byte(cleaned), 0o755); err != nil {
		return fmt.Errorf("failed to write hook file: %w", err)
	}

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func trimRight(s string) string {
	end := len(s)
	for end > 0 && (s[end-1] == '\n') {
		end--
	}
	return s[:end]
}

var (
	errAlreadyInstalled = errorString("gitai hook is already installed in this repository")
	errNotInstalled     = errorString("gitai hook is not installed in this repository")
)

type errorString string

func (e errorString) Error() string { return string(e) }
