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

func RunSuggestFlow(ctx context.Context, provider ai.Provider, editorMode string, filesFromArgs []string) {
	files, err := git.GetChangedFiles()
	if err != nil {
		panic(err)
	}

	if len(files) == 0 {
		println("No changed files to commit.")
		return
	}

	var selectedFiles []string

	if len(filesFromArgs) > 0 {
		// Filter filesFromArgs to ensure they are actually changed/available in git status
		// This prevents errors if the user typos a filename or provides a file that isn't changed.
		validFiles := make(map[string]bool)
		for _, f := range files {
			validFiles[f] = true
		}

		for _, arg := range filesFromArgs {
			resolvedPaths, err := git.ResolvePath(arg)
			if err != nil {
				fmt.Printf("Warning: error resolving file '%s': %v\n", arg, err)
				continue
			}
			if len(resolvedPaths) == 0 {
				fmt.Printf("Warning: file '%s' not found in git (ignored or deleted?)\n", arg)
				continue
			}

			for _, resolved := range resolvedPaths {
				if validFiles[resolved] {
					selectedFiles = append(selectedFiles, resolved)
				} else {
					// Only warn if the specific file (not a dir expansion) was explicitly requested but not changed?
					// Actually, if I do `gitai suggest .`, and most files are not changed, I don't want 1000 warnings.
					// So I should probably only warn if *none* of the resolved paths were valid?
					// Or just stay silent for individual files not being changed if they came from a glob/dir?
					// But `arg` is what the user typed.
					// If `arg` resulted in 1 file, and it's not changed -> warn.
					// If `arg` resulted in 10 files, and 0 are changed -> warn.
					// If `arg` resulted in 10 files, and 1 is changed -> ok.
				}
			}
		}

		// Let's refine the warning logic slightly after collecting all candidates
		// But I need to know which arg produced which files to give good warnings.
		// Re-implementing loop to improve UX:

		// Clear selectedFiles and re-populate
		selectedFiles = []string{}
		
		for _, arg := range filesFromArgs {
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
				if validFiles[resolved] {
					selectedFiles = append(selectedFiles, resolved)
					foundAnyChange = true
				}
			}

			if !foundAnyChange {
				fmt.Printf("Warning: no changed files found matching '%s'\n", arg)
			}
		}

		// Remove duplicates in case multiple args overlapped
		selectedFiles = uniqueStrings(selectedFiles)

		if len(selectedFiles) == 0 {
			println("No valid changed files provided in arguments.")
			return
		}
	} else {
		fileSelectorModel := NewFileSelectorModel(files)
		fileSelectorProgram := tea.NewProgram(&fileSelectorModel)
		if _, err := fileSelectorProgram.Run(); err != nil {
			panic(err)
		}

		if fileSelectorModel.quitting {
			return
		}

		selectedFiles = fileSelectorModel.GetSelectedFiles()
	}

	if len(selectedFiles) == 0 {
		println("No files selected.")
		return
	}

	hintInputModel := NewHintInputModel(JiraHintProcessor, GitHubHintProcessor)
	hintInputProgram := tea.NewProgram(&hintInputModel)
	if _, err := hintInputProgram.Run(); err != nil {
		panic(err)
	}

	if hintInputModel.quitting {
		return
	}

	hint := hintInputModel.GetHint()

	aiModel := NewAIMessageModel(ctx, selectedFiles, provider, editorMode, hint)
	aiModelProgram := tea.NewProgram(&aiModel, tea.WithContext(ctx))

	finalModel, err := aiModelProgram.Run()
	if err != nil {
		panic(err)
	}

	if m, ok := finalModel.(*AIMessageModel); ok && m.state == StatePushed {
		// Try to construct a PR link via git configuration (preferred)
		prURL, err := git.GetPullRequestURL()
		if err == nil && prURL != "" {
			fmt.Printf("\nCreate a Pull Request: %s\n", prURL)
			return
		}

		// Fallback: Try to find a link in push output
		matches := LinksRegex.FindAllStringSubmatch(m.pushOutput, -1)
		if len(matches) > 0 {
			fmt.Println()
		}
		for _, match := range matches {
			if len(match) > 1 {
				fmt.Printf("Create a Pull Request: %s\n", match[1])
			}
		}
	}
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
