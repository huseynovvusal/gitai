package git

import (
	"errors"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

func GetGitRoot() (string, error) {
	path, err := filepath.Abs(".")
	if err != nil {
		return "", err
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