package security

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

type Finding struct {
	File string
	Line int
	Text string
}

func CheckDiffSafety(diffText string, keywords []string) error {
	fileDiffs, err := diff.ParseMultiFileDiff([]byte(diffText))
	if err != nil {
		return err
	}

	var findings []Finding
	for _, fd := range fileDiffs {
		filename := strings.TrimPrefix(fd.NewName, "b/")
		filename = strings.TrimPrefix(filename, "a/")

		for _, h := range fd.Hunks {
			lines := strings.Split(string(h.Body), "\n")
			newLine := int(h.NewStartLine)

			for _, ln := range lines {
				if ln == "" {
					continue
				}

				switch ln[0] {
				case '+':
					text := strings.TrimPrefix(ln, "+")
					if containsKeyword(text, keywords) {
						findings = append(findings, Finding{File: filename, Line: newLine, Text: strings.TrimSpace(text)})
					}
					newLine++
				case ' ':
					// context line advances new file line number
					newLine++
				case '-':
					// removed line; does not advance new file line number
				default:
					// unknown prefix - ignore
				}
			}
		}
	}

	if len(findings) == 0 {
		return nil
	}

	var b strings.Builder
	cwd, _ := os.Getwd()
	for _, f := range findings {
		abs := f.File
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
		// create file:// URI with an encoded path so terminals like VS Code treat it as a clickable link
		u := url.URL{Scheme: "file", Path: abs}
		fileURI := u.String()
		b.WriteString(fmt.Sprintf("- %s:%d:1: %s\n", fileURI, f.Line, f.Text))
	}

	return fmt.Errorf("%s", b.String())
}

func containsKeyword(s string, keywords []string) bool {
	ls := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(ls, kw) {
			return true
		}
	}
	return false
}
