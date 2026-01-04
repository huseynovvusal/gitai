package git

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// GetGitRoot returns the absolute path to the repository root.
func GetGitRoot() (string, error) {
	return findGitRoot(".")
}

// findGitRoot traverses up from the startPath to find the git repository root.
func findGitRoot(startPath string) (string, error) {
	path, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	for {
		_, err := git.PlainOpen(path)
		if err == nil {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			return "", errors.New("not a git repository")
		}

		path = parent
	}
}
