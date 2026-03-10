package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"huseynovvusal/gitai/internal/git"
)

const (
	hookFileName = "prepare-commit-msg"
	markerBegin  = "# >>> gitai hook >>>"
	markerEnd    = "# <<< gitai hook <<<"
)

// hookScript returns the shell snippet that gitai injects into prepare-commit-msg.
// It skips when a message is already provided (commit -m), on merge commits, or
// when the GITAI_SKIP_HOOK env var is set.
func hookScript() string {
	return fmt.Sprintf(`%s
# Installed by: gitai hook install
# This hook runs gitai to generate a commit message when none is provided.
# Set GITAI_SKIP_HOOK=1 to bypass this hook.

if [ -n "$GITAI_SKIP_HOOK" ]; then
  exit 0
fi

COMMIT_MSG_FILE="$1"
COMMIT_SOURCE="$2"

# Only run when there is no explicit message source (e.g. -m, merge, squash).
if [ -z "$COMMIT_SOURCE" ]; then
  if command -v gitai >/dev/null 2>&1; then
    # Run gitai in hook mode: generate message and write to the commit message file.
    GITAI_SKIP_HOOK=1 gitai suggest --no-hint 2>/dev/null
    LAST_MSG=$(git log -1 --pretty=%%B 2>/dev/null)
    if [ -n "$LAST_MSG" ]; then
      printf "%%s" "$LAST_MSG" > "$COMMIT_MSG_FILE"
    fi
  fi
fi
%s
`, markerBegin, markerEnd)
}

// Install adds the gitai hook snippet to the prepare-commit-msg hook in the
// current git repository. If a hook file already exists, the snippet is appended
// without overwriting the existing content. If a gitai hook is already installed,
// it returns an error.
func Install() error {
	hookPath, err := hookFilePath()
	if err != nil {
		return err
	}

	existing, err := readFileIfExists(hookPath)
	if err != nil {
		return fmt.Errorf("failed to read existing hook: %w", err)
	}

	if strings.Contains(existing, markerBegin) {
		return errors.New("gitai hook is already installed in this repository")
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

// Uninstall removes the gitai hook snippet from the prepare-commit-msg hook.
// If other content remains, the file is preserved; otherwise it is removed.
func Uninstall() error {
	hookPath, err := hookFilePath()
	if err != nil {
		return err
	}

	existing, err := readFileIfExists(hookPath)
	if err != nil {
		return fmt.Errorf("failed to read existing hook: %w", err)
	}

	if !strings.Contains(existing, markerBegin) {
		return errors.New("gitai hook is not installed in this repository")
	}

	cleaned := removeSection(existing, markerBegin, markerEnd)
	cleaned = strings.TrimRight(cleaned, "\n") + "\n"

	// If only the shebang remains, remove the file entirely.
	trimmed := strings.TrimSpace(cleaned)
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

// IsInstalled checks if the gitai hook is currently installed.
func IsInstalled() (bool, error) {
	hookPath, err := hookFilePath()
	if err != nil {
		return false, err
	}

	existing, err := readFileIfExists(hookPath)
	if err != nil {
		return false, err
	}

	return strings.Contains(existing, markerBegin), nil
}

func hookFilePath() (string, error) {
	root, err := git.GetGitRoot()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return filepath.Join(root, ".git", "hooks", hookFileName), nil
}

func readFileIfExists(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(data), nil
}

func removeSection(content, beginMarker, endMarker string) string {
	beginIdx := strings.Index(content, beginMarker)
	if beginIdx == -1 {
		return content
	}
	endIdx := strings.Index(content, endMarker)
	if endIdx == -1 {
		return content
	}
	endIdx += len(endMarker)

	// Also consume trailing newline after the end marker.
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// Also consume the leading newline before the begin marker.
	if beginIdx > 0 && content[beginIdx-1] == '\n' {
		beginIdx--
	}

	return content[:beginIdx] + content[endIdx:]
}
