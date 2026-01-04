package git

import (
	"errors"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// GetGitRoot returns the absolute path to the repository root using go-git.
func GetGitRoot() (string, error) {
	// Start looking from the current directory
	path, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}

	// Traverse up to find the .git directory
	for {
		// Use PlainOpen to check if the current directory is a valid git repository
		_, err := git.PlainOpen(path)
		if err == nil {
			return path, nil
		}

		parent := filepath.Dir(path)
		if parent == path {
			// Reached root without finding a git repo
			return "", errors.New("not a git repository")
		}
		path = parent
	}
}
