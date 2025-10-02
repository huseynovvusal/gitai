package ai

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed system_prompt.md
var systemMessage string

var whitespaceRegex = regexp.MustCompile(`\s+`)

// compressWhitespace replaces sequences of one or more whitespace characters
// (spaces, newlines, tabs) with a single space, and then trims leading/trailing spaces.
func compressWhitespace(input string) string {
	compressed := whitespaceRegex.ReplaceAllString(input, " ")
	return strings.TrimSpace(compressed)
}

// CompressWhitespace is an exported wrapper for compressWhitespace to allow
// other packages (e.g., CLI commands) to reuse the same normalization logic
// when preparing prompts or measuring token usage.
func CompressWhitespace(input string) string {
	return compressWhitespace(input)
}
