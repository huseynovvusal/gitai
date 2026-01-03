package cleaner

import (
	"fmt"
	"strings"
)

// CleanCommitMessage cleans and formats a raw commit message string.
// It handles:
// 1. Stripping Markdown code fences.
// 2. Enforcing "Tim Pope" style rules (no trailing period in subject, blank line separation).
// 3. Normalizing bullet points to the specified character.
func CleanCommitMessage(s string, bulletChar string) string {
	s = strings.TrimSpace(s)

	// 1. Handle Markdown Fences
	if strings.HasPrefix(s, "```") && strings.HasSuffix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")

		// Handle Language Identifier (e.g., ```go)
		if firstLine, rest, found := strings.Cut(s, "\n"); found {
			if !strings.Contains(firstLine, " ") {
				s = rest
			}
		}
	}

	s = strings.TrimSpace(s)

	// 2. Enforce "Tim Pope" Style Rules
	lines := strings.Split(s, "\n")
	if len(lines) > 0 {
		// Rule: Do not end the subject line with a period
		lines[0] = strings.TrimSuffix(strings.TrimSpace(lines[0]), ".")
	}

	// Rule: Separate subject from body with a blank line
	if len(lines) > 1 {
		// If the second line (index 1) is NOT empty, we assume the AI forgot the blank line.
		if strings.TrimSpace(lines[1]) != "" {
			// Insert an empty line at index 1
			lines = append(lines[:1], append([]string{""}, lines[1:]...)...)
		}
	}

	s = strings.Join(lines, "\n")

	// 3. Normalize Bullet Points
	s = normalizeBulletPoints(s, bulletChar)

	return strings.TrimSpace(s)
}

func normalizeBulletPoints(text string, bullet string) string {
	if bullet == "" {
		bullet = "-" // Default
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for common bullet point indicators
		if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") || strings.HasPrefix(trimmed, "- ") {
			// Find where the content starts (skip the bullet marker)
			content := ""
			if strings.HasPrefix(trimmed, "* ") {
				content = strings.TrimPrefix(trimmed, "* ")
			} else if strings.HasPrefix(trimmed, "• ") {
				content = strings.TrimPrefix(trimmed, "• ")
			} else if strings.HasPrefix(trimmed, "- ") {
				content = strings.TrimPrefix(trimmed, "- ")
			}
			
			// Reconstruct line
			lines[i] = fmt.Sprintf("%s %s", bullet, content)
		}
	}
	return strings.Join(lines, "\n")
}
