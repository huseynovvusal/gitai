package security

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

// Finding represents a security match in a diff.
type Finding struct {
	File string
	Line int
	Text string
}

// CheckDiffSafety scans a diff for sensitive keywords in added lines.
func CheckDiffSafety(diffText string, keywords []string) error {
	fileDiffs, err := diff.ParseMultiFileDiff([]byte(diffText))
	if err != nil {
		return fmt.Errorf("failed to parse diff: %w", err)
	}

	var findings []Finding

	for _, fileDiff := range fileDiffs {
		// Normalize filename
		filename := strings.TrimPrefix(fileDiff.NewName, "b/")
		filename = strings.TrimPrefix(filename, "a/")

		for _, hunk := range fileDiff.Hunks {
			lines := strings.Split(string(hunk.Body), "\n")
			newLineNum := int(hunk.NewStartLine)

			for _, line := range lines {
				if line == "" {
					continue
				}

				switch line[0] {
				case '+':
					content := line[1:]
					if containsKeyword(content, keywords) {
						findings = append(findings, Finding{
							File: filename,
							Line: newLineNum,
							Text: strings.TrimSpace(content),
						})
					}
					newLineNum++
				case ' ':
					newLineNum++
				case '-':
					// Removed line
				}
			}
		}
	}

	if len(findings) == 0 {
		return nil
	}

	return formatFindings(findings)
}

func formatFindings(findings []Finding) error {
	var builder strings.Builder
	currentDir, _ := os.Getwd()

	for _, find := range findings {
		absPath := find.File
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(currentDir, absPath)
		}
		// Create file:// URI for clickable terminal links
		fileURL := url.URL{Scheme: "file", Path: absPath}
		builder.WriteString(fmt.Sprintf("- %s:%d:1: %s\n", fileURL.String(), find.Line, find.Text))
	}

	return fmt.Errorf("sensitive data found:\n%s", builder.String())
}

func containsKeyword(line string, keywords []string) bool {
	lowerLine := strings.ToLower(line)
	for _, kw := range keywords {
		if strings.Contains(lowerLine, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}