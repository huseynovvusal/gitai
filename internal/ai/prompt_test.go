package ai

import (
	"regexp"
	"testing"
)

// Test that compressWhitespace collapses all whitespace sequences and trims ends.
func TestCompressWhitespace(t *testing.T) {
	in := "  some\n\n\t text\t with   spaces\n\n"

	out := compressWhitespace(in)
	if out != "some text with spaces" {
		t.Fatalf("unexpected compressedUserMessage result: %q", out)
	}

	// ensure no double spaces remain
	if regexp.MustCompile(`\s{2,}`).FindStringIndex(out) != nil {
		t.Fatalf("compressedUserMessage output still contains multiple spaces: %q", out)
	}
}
