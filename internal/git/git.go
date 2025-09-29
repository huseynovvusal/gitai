package git

import (
	"os/exec"
	"strings"
)

// GetDiff returns the output of `git diff`.
func GetDiff() (string, error) {
	cmd := exec.Command("git", "diff")

	out, err := cmd.CombinedOutput()

	return string(out), err
}

// GetStatus returns the output of `git status`.
func GetStatus() (string, error) {
	cmd := exec.Command("git", "status")

	out, err := cmd.CombinedOutput()

	return string(out), err
}

// CommitChanges creates a git commit with the provided message.
func CommitChanges(message string) error {
	cmd := exec.Command("git", "commit", "-am", message)

	_, err := cmd.CombinedOutput()

	return err
}

// AddChanges stages all changes in the working directory.
func AddChanges() error {
	cmd := exec.Command("git", "add", ".")

	_, err := cmd.CombinedOutput()

	return err
}

// GetChangedFiles returns a list of changed files using `git status --porcelain`.
func GetChangedFiles() ([]string, error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	var files []string
	for _, line := range lines {
		if len(line) > 3 {
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}

// GetChangesForFiles returns the git diff for the specified files.
func GetChangesForFiles(files []string) (string, error) {
	// Trim whitespace and remove empty entries to avoid calling
	// `git diff --` with no paths (which returns the full diff).
	var clean []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		clean = append(clean, f)
	}

	if len(clean) == 0 {
		// No files specified — return empty diff instead of full repo diff.
		return "", nil
	}

	args := append([]string{"diff", "--"}, clean...)

	out, err := exec.Command("git", args...).Output()

	return string(out), err
}

// Commit stages the selected files and creates a commit with the given message.
func Commit(files []string, message string) error {
	args := append([]string{"add"}, files...)
	if err := exec.Command("git", args...).Run(); err != nil {
		return err
	}
	if err := exec.Command("git", "commit", "-m", message).Run(); err != nil {
		return err
	}
	return nil
}

// Push pushes the current branch to the remote repository.
func Push() error {
	return exec.Command("git", "push").Run()
}
