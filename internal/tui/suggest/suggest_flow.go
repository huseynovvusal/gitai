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

func RunSuggestFlow(ctx context.Context, provider ai.Provider, editorMode string) {
	files, err := git.GetChangedFiles()
	if err != nil {
		panic(err)
	}

	if len(files) == 0 {
		println("No changed files to commit.")
		return
	}

	fileSelectorModel := NewFileSelectorModel(files)
	fileSelectorProgram := tea.NewProgram(&fileSelectorModel)
	if _, err := fileSelectorProgram.Run(); err != nil {
		panic(err)
	}

	if fileSelectorModel.quitting {
		return
	}

	selectedFiles := fileSelectorModel.GetSelectedFiles()

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
