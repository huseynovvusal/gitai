package ai

import (
	_ "embed"
	"strings"
)

//go:embed system_prompt.md
var systemMessage string

// compressWhitespace replaces sequences of one or more whitespace characters
// (spaces, newlines, tabs) with a single space, and then trims leading/trailing spaces.
func compressWhitespace(input string) string {
	// 1. Replace all sequences of whitespace with a single space.
	compressed := whitespaceRegex.ReplaceAllString(input, " ")

	// 2. Remove any leading or trailing spaces.
	return strings.TrimSpace(compressed)
}
