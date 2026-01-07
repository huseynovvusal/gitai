package cleaner

import (
	"fmt"
	"strings"
)

var (
	bullets = []string{"* ", "• ", "- ", ". "}
)

// MessageCleaner defines a function signature for a transformation step
type MessageCleaner func(string) string

func CleanCommitMessage(s string, bulletChar string) string {
	// Define the pipeline in the order of execution
	pipeline := []MessageCleaner{
		strings.TrimSpace,
		stripMarkdownFences,
		ensureBlankLineAfterSubject,
		stripSubjectPeriod,
		normalizeBullets(bulletChar), // Returns a closure
		strings.TrimSpace,
	}

	for _, transform := range pipeline {
		s = transform(s)
	}
	return s
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") || !strings.HasSuffix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")

	if firstLine, rest, found := strings.Cut(s, "\n"); found {
		if !strings.Contains(firstLine, " ") {
			return strings.TrimSpace(rest)
		}
	}
	return strings.TrimSpace(s)
}

func stripSubjectPeriod(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 {
		lines[0] = strings.TrimSuffix(strings.TrimSpace(lines[0]), ".")
	}
	return strings.Join(lines, "\n")
}

func ensureBlankLineAfterSubject(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		lines = append(lines[:1], append([]string{""}, lines[1:]...)...)
	}
	return strings.Join(lines, "\n")
}

func normalizeBullets(bullet string) MessageCleaner {
	if bullet == "" {
		bullet = "-"
	}
	return func(text string) string {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			for _, bb := range bullets {
				if content, found := strings.CutPrefix(trimmed, bb); found {
					lines[i] = fmt.Sprintf("%s %s", bullet, content)
					break
				}
			}
		}
		return strings.Join(lines, "\n")
	}
}
