package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// command is a variable that holds the function to execute a command.
// It's implemented as a variable to allow for mocking in tests.
var command = exec.Command

// GetStatusForFiles returns the `git status --porcelain` output, but only for
// the files specified in the input list.
func GetStatusForFiles(files []string) (string, error) {
	// If the input list is empty, there's nothing to do.
	if len(files) == 0 {
		return "", nil
	}

	files = expandFiles(files)

	// Get the status for the entire repository.
	args := []string{"status", "--porcelain", "--"}
	args = append(args, files...)
	cmd := command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run git status: %w", err)
	}
	allLines := strings.Split(string(out), "\n")
	relevantLines := filterStatusLines(allLines, files)

	// Join the filtered lines back into a single string.
	return strings.Join(relevantLines, "\n"), nil
}

// filterStatusLines filters the raw output from `git status --porcelain`.
// It returns only the lines relevant to the files.
func filterStatusLines(allLines []string, files []string) []string {
	var relevantLines []string
	for _, line := range allLines {
		if len(line) < 4 {
			continue // Skip empty or malformed lines
		}
		// The porcelain format is "XY filepath", so the path starts at index 3.
		filePath := strings.TrimSpace(line[3:])

		// A special case is a renamed file, e.g., "R old-name -> new-name"
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			path1 := cleanPath(parts[0])
			path2 := cleanPath(parts[1])
			ok1 := slices.Contains(files, path1)
			ok2 := slices.Contains(files, path2)
			// Include the line if either the old or new name is in our list.
			if ok1 || ok2 {
				relevantLines = append(relevantLines, line)
			}
		} else {
			// For all other cases, check if the file path is in our set.
			if ok := slices.Contains(files, cleanPath(filePath)); ok {
				relevantLines = append(relevantLines, line)
			}
		}
	}
	return relevantLines
}

// GetChangedFiles returns a list of changed (modified, new, etc.) files.
func GetChangedFiles() ([]string, error) {
	out, err := command("git", "status", "--porcelain").Output()
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
	files = expandFiles(files)
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

	cmd := command("git", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If git diff HEAD fails, it likely means the project is new and has no HEAD.
		// In this case, we read the full content of the files.
		var sb strings.Builder
		for _, f := range clean {
			content, readErr := os.ReadFile(f)
			if readErr != nil {
				return "", fmt.Errorf("git diff failed: %w\n%s\nAlso failed to read file %s: %v", err, stderr.String(), f, readErr)
			}
			sb.WriteString(fmt.Sprintf("File: %s\n", f))
			sb.Write(content)
			sb.WriteString("\n")
		}
		return sb.String(), nil
	}

	return out.String(), nil
}

// Commit stages and commits *only* the specified files with the given message.
// This is the corrected and safe version of the commit logic.
func Commit(files []string, message string) error {
	if len(files) == 0 {
		return errors.New("no files provided to commit")
	}

	files = expandFiles(files)

	var filesToAdd []string
	var filesToCommit []string
	var missingFiles []string

	for _, f := range files {
		if fileExists(f) {
			filesToAdd = append(filesToAdd, f)
			filesToCommit = append(filesToCommit, f)
		} else {
			missingFiles = append(missingFiles, f)
		}
	}

	if len(missingFiles) > 0 {
		// Check for unstaged deletions (files present in index but missing on disk)
		cmd := command("git", append([]string{"ls-files", "--"}, missingFiles...)...)
		out, _ := cmd.Output() // Ignore error, empty output is fine
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				filesToAdd = append(filesToAdd, line)
				filesToCommit = append(filesToCommit, line)
			}
		}

		// Check for staged deletions (files missing on disk AND changed in index)
		cmd = command("git", append([]string{"diff", "--name-only", "--cached", "--"}, missingFiles...)...)
		out, _ = cmd.Output()
		lines = strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				if !slices.Contains(filesToCommit, line) {
					filesToCommit = append(filesToCommit, line)
				}
			}
		}
	}

	if len(filesToCommit) == 0 {
		return errors.New("no valid files resolved to commit")
	}

	// First, stage the specific files
	if len(filesToAdd) > 0 {
		addArgs := append([]string{"add", "--"}, filesToAdd...)
		if out, err := command("git", addArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to stage files: %w\n%s", err, string(out))
		}
	}

	// Then, commit *only* those files.
	commitArgs := append([]string{"commit", "-m", message, "--"}, filesToCommit...)
	if out, err := command("git", commitArgs...).CombinedOutput(); err != nil {
		// Check if the error is "nothing to commit" and if so, return nil.
		if strings.Contains(string(out), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %w\n%s", err, string(out))
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// expandFiles handles the "old -> new" format returned by git status for renames.
// It splits such strings into individual file paths.
// It also strips surrounding double quotes if present, which git status adds for paths with spaces/special chars.
func expandFiles(files []string) []string {
	var expanded []string
	for _, f := range files {
		if strings.Contains(f, " -> ") {
			parts := strings.Split(f, " -> ")
			for _, part := range parts {
				expanded = append(expanded, cleanPath(part))
			}
		} else {
			expanded = append(expanded, cleanPath(f))
		}
	}
	return expanded
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		return path[1 : len(path)-1]
	}
	return path
}

// Push pushes the current branch to the remote repository.
// It returns the command output and any error encountered.
func Push() (string, error) {
	cmd := command("git", "push")
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("git push failed: %s", output)
	}
	return output, nil
}
