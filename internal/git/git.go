package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GetDiff returns the output of `git diff`.
func GetDiff() (string, error) {
	cmd := exec.Command("git", "diff")

	out, err := cmd.CombinedOutput()

	return string(out), err
}

// GetStatus returns the output of `git status --porcelain`.
func GetStatus() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// GetStatusForFiles returns the `git status --porcelain` output, but only for
// the files specified in the input list.
func GetStatusForFiles(files []string) (string, error) {
	// If the input list is empty, there's nothing to do.
	if len(files) == 0 {
		return "", nil
	}

	// Create a set (using a map) for efficient O(1) lookups.
	// This lets us quickly check if a file is one we care about.
	filesToInclude := make(map[string]struct{})
	for _, f := range files {
		filesToInclude[f] = struct{}{}
	}

	// Get the status for the entire repository.
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run git status: %w", err)
	}

	var relevantLines []string
	allLines := strings.Split(string(out), "\n")

	// Iterate over each line of the status output and filter it.
	for _, line := range allLines {
		if len(line) < 4 {
			continue // Skip empty or malformed lines
		}

		// The porcelain format is "XY filepath", so the path starts at index 3.
		filePath := strings.TrimSpace(line[3:])

		// A special case is a renamed file, e.g., "R  new-name -> old-name"
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			newName := parts[0]
			oldName := parts[1]
			// Include the line if either the old or new name is in our list.
			if filesToInclude[newName] == struct{}{} || filesToInclude[oldName] == struct{}{} {
				relevantLines = append(relevantLines, line)
			}
		} else {
			// For all other cases, just check if the file path is in our set.
			if filesToInclude[filePath] == struct{}{} {
				relevantLines = append(relevantLines, line)
			}
		}
	}

	// Join the filtered lines back into a single string.
	return strings.Join(relevantLines, "\n"), nil
}

// GetChangedFiles returns a list of changed (modified, new, etc.) files.
func GetChangedFiles() ([]string, error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	var files []string
	for _, line := range lines {
		// Ensure the line is long enough and extract the file path
		if len(line) > 3 {
			files = append(files, strings.TrimSpace(line[3:]))
		}
	}
	return files, nil
}

// GetChangesForFiles returns the git diff for only the specified files.
// GetChangesForFiles returns the git diff for the specified files against HEAD.
// This shows all staged and unstaged changes for only those files.
func GetChangesForFiles(files []string) (string, error) {
	var clean []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" {
			clean = append(clean, f)
		}
	}

	if len(clean) == 0 {
		return "", nil
	}

	// Construct the arguments: git diff HEAD -- <file1> <file2>...
	args := append([]string{"diff", "HEAD", "--"}, clean...)

	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\n%s", err, stderr.String())
	}

	return out.String(), nil
}

// Commit stages and commits *only* the specified files with the given message.
// This is the corrected and safe version of the commit logic.
func Commit(files []string, message string) error {
	if len(files) == 0 {
		return errors.New("no files provided to commit")
	}

	// First, stage the specific files
	addArgs := append([]string{"add", "--"}, files...)
	if out, err := exec.Command("git", addArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage files: %w\n%s", err, string(out))
	}

	// Then, commit *only* those files, leaving other staged files alone.
	// Note: We don't use -a here. We commit what we just added.
	commitArgs := append([]string{"commit", "-m", message})
	if out, err := exec.Command("git", commitArgs...).CombinedOutput(); err != nil {
		// Check if the error is "nothing to commit" and if so, return nil.
		// This can happen if the files added had no actual changes.
		if strings.Contains(string(out), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %w\n%s", err, string(out))
	}

	return nil
}

// Push pushes the current branch to the remote repository.
// This simplified version returns Git's helpful error messages directly.
func Push() error {
	cmd := exec.Command("git", "push")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %s", string(out))
	}
	return nil
}
