package git

import (
	"os/exec"
	"strings"
)

func GetGitRoot() (string, error) {
	// Executes: git rev-parse --show-toplevel
	// This command outputs the absolute path to the repository root.
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")

	// Capture the output
	output, err := cmd.Output()
	if err != nil {
		// Return nil error if 'git' command isn't found or if not in a git repo
		// This allows the app to proceed using other config paths.
		return "", err
	}

	// Trim whitespace and newlines from the command output
	return strings.TrimSpace(string(output)), nil
}
