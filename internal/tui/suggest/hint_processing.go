package suggest

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	jiraRegex   = regexp.MustCompile(`\/browse\/([A-Z][A-Z0-9]+-\d+)`)
	githubRegex = regexp.MustCompile(`\/issues\/(\d+)`)
)

// HintProcessor transforms the input hint string.
type HintProcessor func(string) string

// JiraHintProcessor extracts a Jira ticket ID from a URL and appends a formatting instruction.
func JiraHintProcessor(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if matches := jiraRegex.FindStringSubmatch(input); len(matches) > 1 {
		ticketID := matches[1]
		instruction := fmt.Sprintf("Ticket: %s. The commit message header must be in the format: <type>(<scope>): %s <description>", ticketID, ticketID)
		return fmt.Sprintf("%s\n%s", input, instruction)
	}

	return input
}

// GitHubHintProcessor extracts a GitHub issue ID from a URL and appends a formatting instruction.
func GitHubHintProcessor(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if matches := githubRegex.FindStringSubmatch(input); len(matches) > 1 {
		ticketID := "#" + matches[1]
		instruction := fmt.Sprintf("Ticket: %s. The commit message header must be in the format: <type>(<scope>): %s <description>", ticketID, ticketID)
		return fmt.Sprintf("%s\n%s", input, instruction)
	}

	return input
}

