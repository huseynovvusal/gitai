package git

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// This regex is strict:
// 1. It looks for a version number ([0-9]+\.[0-9]+...)
// 2. It ensures the version is NOT preceded by a digit or dot (preventing 6.0 matching inside 0.6.0)
// 3. It ensures the version is NOT followed by a digit or dot.
// 4. It optionally captures prefixes like "version": or "v".
var versionRegex = regexp.MustCompile(`(?i)(?:version["']?\s*[:=]\s*["']|v)?([0-9]+\.[0-9]+(?:\.[0-9a-z.-]*)?)`)

// ExtractVersionFromDiff scans a unified diff for lines that look like version updates.
func ExtractVersionFromDiff(diffText string) string {
	lines := strings.Split(diffText, "\n")

	var oldVersion, newVersion string
	var currentFile string
	var isVerFile bool

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				rawPath := parts[len(parts)-1]
				currentFile = filepath.Base(rawPath)
				isVerFile = isVersionFile(currentFile)
				oldVersion, newVersion = "", ""
			}
			continue
		}

		if !isVerFile {
			continue
		}

		lowerLine := strings.ToLower(line)
		isExplicitVersionFile := strings.EqualFold(currentFile, "VERSION")
		containsVersionKeyword := strings.Contains(lowerLine, "version") && !strings.Contains(lowerLine, "versioning")

		if isExplicitVersionFile || containsVersionKeyword {
			if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				if v := findStrictVersion(versionRegex, line[1:]); v != "" {
					oldVersion = v
				}
			}
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				if v := findStrictVersion(versionRegex, line[1:]); v != "" {
					newVersion = v
				}
			}
		}

		if oldVersion != "" && newVersion != "" && oldVersion != newVersion {
			return fmt.Sprintf("%s -> %s", oldVersion, newVersion)
		}
	}

	return newVersion
}

func findStrictVersion(re *regexp.Regexp, text string) string {
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}

	var best string
	for _, m := range matches {
		if len(m) > 1 {
			cand := strings.Trim(m[1], " \t\n\r\"',")

			// Find where this match starts in the line
			idx := strings.Index(text, cand)
			if idx > 0 {
				prevChar := text[idx-1]
				// If the character before the match is a digit or a dot,
				// then '6.0' is just a part of '0.6.0'. Skip it.
				if (prevChar >= '0' && prevChar <= '9') || prevChar == '.' {
					continue
				}
			}

			// If the character AFTER the match is a digit or a dot, skip it.
			endIdx := idx + len(cand)
			if endIdx < len(text) {
				nextChar := text[endIdx]
				if (nextChar >= '0' && nextChar <= '9') || nextChar == '.' {
					continue
				}
			}

			if len(cand) > len(best) {
				best = cand
			}
		}
	}
	return best
}

func isVersionFile(filename string) bool {
	f := strings.ToLower(filename)
	if strings.Contains(f, "test") || strings.Contains(f, "_spec") {
		return false
	}
	targets := []string{"version", "package.json", "go.mod", "cargo.toml", "pyproject.toml", "composer.json", "gemfile", "mix.exs", "version.rb", "version.py", "setup.py", "cmakelists.txt"}
	for _, t := range targets {
		if strings.EqualFold(f, t) {
			return true
		}
	}
	return false
}
