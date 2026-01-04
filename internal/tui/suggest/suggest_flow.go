package suggest

import (
	"context"
	"fmt"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/git"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	LinksRegex = regexp.MustCompile(`remote:\s*(https?://\S+)`)
)

type Flow struct {
	ctx            context.Context
	generator      ai.CommitMessageGenerator
	editorMode     string
	hintProcessors []HintProcessor
	hint           string
	skipHint       bool
}

func NewFlow(ctx context.Context, generator ai.CommitMessageGenerator, editorMode string, hintProcessors ...HintProcessor) *Flow {
	return &Flow{
		ctx:            ctx,
		generator:      generator,
		editorMode:     editorMode,
		hintProcessors: hintProcessors,
	}
}

func (s *Flow) WithHint(hint string) *Flow {
	s.hint = hint
	return s
}

func (s *Flow) WithSkipHint(skip bool) *Flow {
	s.skipHint = skip
	return s
}

func (s *Flow) Run(filesFromArgs []string) {
	// 1. Get all changed files from Git
	changedFiles, err := git.GetChangedFiles()
	if err != nil {
		panic(err)
	}

	if len(changedFiles) == 0 {
		println("No changed files to commit.")
		return
	}

	// 2. Determine which files to use (via Args or UI)
	selectedFiles := selectFiles(changedFiles, filesFromArgs)
	if len(selectedFiles) == 0 {
		println("No valid files selected.")
		return
	}

	// 3. Get User Hint
	hint := s.hint
	if !s.skipHint && hint == "" {
		hint, err = s.runHintInput()
		if err != nil {
			return // User quit
		}
	}

	// 4. Run AI Generation Flow
	aiModel := NewAIMessageModel(s.ctx, selectedFiles, s.generator, s.editorMode, hint)
	aiModelProgram := tea.NewProgram(&aiModel, tea.WithContext(s.ctx))

	finalModel, err := aiModelProgram.Run()
	if err != nil {
		panic(err)
	}

	// 5. Post-Run Actions (PR Links)
	if m, ok := finalModel.(*AIMessageModel); ok && m.state == StatePushed {
		printPullRequestInfo(m.pushOutput)
	}
}

// selectFiles determines if we filter arguments or show the UI selector
func selectFiles(availableFiles []string, args []string) []string {
	if len(args) > 0 {
		// Logic extracted here for testability
		return FilterCompatibleFiles(availableFiles, args)
	}

	// Fallback to TUI if no args provided
	fileSelectorModel := NewFileSelectorModel(availableFiles)
	fileSelectorProgram := tea.NewProgram(&fileSelectorModel)
	if _, err := fileSelectorProgram.Run(); err != nil {
		panic(err)
	}

	if fileSelectorModel.quitting {
		return nil
	}
	return fileSelectorModel.GetSelectedFiles()
}

// FilterCompatibleFiles takes a list of changed files and a list of patterns (args),
// resolves the patterns, and returns only the files that actually exist in the changed list.
func FilterCompatibleFiles(availableFiles []string, patterns []string) []string {
	validMap := make(map[string]bool, len(availableFiles))
	for _, f := range availableFiles {
		validMap[f] = true
	}

	var selected []string

	for _, arg := range patterns {
		resolvedPaths, err := git.ResolvePath(arg)
		if err != nil {
			fmt.Printf("Warning: error resolving file '%s': %v\n", arg, err)
			continue
		}
		if len(resolvedPaths) == 0 {
			fmt.Printf("Warning: '%s' matched no tracked files\n", arg)
			continue
		}

		foundAnyChange := false
		for _, resolved := range resolvedPaths {
			if validMap[resolved] {
				selected = append(selected, resolved)
				foundAnyChange = true
			}
		}

		if !foundAnyChange {
			fmt.Printf("Warning: no changed files found matching '%s'\n", arg)
		}
	}

	return uniqueStrings(selected)
}

// runHintInput isolates the TUI logic for hints
func (s *Flow) runHintInput() (string, error) {
	hintInputModel := NewHintInputModel(s.hintProcessors...)
	hintInputProgram := tea.NewProgram(&hintInputModel)
	if _, err := hintInputProgram.Run(); err != nil {
		return "", err
	}

	if hintInputModel.quitting {
		return "", fmt.Errorf("canceled")
	}
	return hintInputModel.GetHint(), nil
}

// printPullRequestInfo handles the output parsing for PR links
func printPullRequestInfo(pushOutput string) {
	// Preferred: Git config
	prURL, err := git.GetPullRequestURL("origin")
	if err == nil && prURL != "" {
		fmt.Printf("\nCreate a Pull Request: %s\n", prURL)
		return
	}

	// Fallback: Regex scan of output
	matches := LinksRegex.FindAllStringSubmatch(pushOutput, -1)
	if len(matches) > 0 {
		fmt.Println()
	}
	for _, match := range matches {
		if len(match) > 1 {
			fmt.Printf("Create a Pull Request: %s\n", match[1])
		}
	}
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	list := make([]string, 0, len(slice))
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
