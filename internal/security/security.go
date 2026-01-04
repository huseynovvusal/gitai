package security

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Finding represents a security match in a diff.
type Finding struct {
	File string
	Line int
	Text string
}

// CheckDiffSafety scans a diff for sensitive keywords in added lines.
// It is designed to be resilient to different diff formats.
func CheckDiffSafety(diffText string, keywords []string) error {
	if len(keywords) == 0 {
		return nil
	}

	var findings []Finding
	var currentFile string
	var currentLine int

	lines := strings.Split(diffText, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++ "):
			// Update current file (stripping b/ prefix if present)
			currentFile = strings.TrimPrefix(strings.TrimSpace(line[4:]), "b/")
		case strings.HasPrefix(line, "@@ "):
			// Reset line counter from hunk header: @@ -1,1 +123,1 @@ -> 123
			currentLine = parseStartLine(line)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			// Check added line for keywords
			content := line[1:]
			if containsKeyword(content, keywords) {
				findings = append(findings, Finding{
					File: currentFile,
					Line: currentLine,
					Text: strings.TrimSpace(content),
				})
			}
			currentLine++
		case strings.HasPrefix(line, " "):
			// Context line advances line count
			currentLine++
		}
	}

	if len(findings) == 0 {
		return nil
	}

	return formatFindings(findings)
}

func parseStartLine(header string) int {
	re := regexp.MustCompile(`\+(\d+)`)
	matches := re.FindStringSubmatch(header)
	if len(matches) > 1 {
		val, _ := strconv.Atoi(matches[1])
		return val
	}
	return 0
}

func formatFindings(findings []Finding) error {
	var builder strings.Builder
	currentDir, _ := os.Getwd()

	for _, find := range findings {
		absPath := find.File
		if absPath == "" {
			absPath = "unknown"
		}
		if !filepath.IsAbs(absPath) && absPath != "unknown" {
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
