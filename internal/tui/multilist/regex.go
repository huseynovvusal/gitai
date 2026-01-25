package multilist

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

func regexFilter(term string, targets []string) []list.Rank {
	// Support simple wildcards: replace * with .*
	// We escape other regex meta-characters first if we wanted true globbing,
	// but for "smart" regex, just allowing * to be .* is a common convenience.
	// If the user deliberately types .* it will become ..* which is also valid but odd.
	// Better approach: if term contains *, treat it as wildcard.
	// However, if we simply replace, we might break existing regex usage.
	// Let's do a simple replacement for now as requested.
	term = strings.ReplaceAll(term, "*", ".*")

	re, err := regexp.Compile(term)
	if err != nil {
		return nil
	}

	var ranks []list.Rank

	for i, t := range targets {
		if re.MatchString(t) {
			ranks = append(ranks, list.Rank{
				Index:          i,
				MatchedIndexes: nil,
			})
		}
	}

	return ranks
}

func WithRegexFiltering() Option {
	return func(l *list.Model) {
		l.Filter = regexFilter
		l.FilterInput.Placeholder = "Filter (Regex supported)..."
	}
}
