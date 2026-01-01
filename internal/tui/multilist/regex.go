package multilist

import (
	"github.com/charmbracelet/bubbles/list"
	"regexp"
)

func regexFilter(term string, targets []string) []list.Rank {
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
