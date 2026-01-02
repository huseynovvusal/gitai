package suggest

import (
	"context"
	"fmt"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/git"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
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

	selectedFiles := []string{}
	for i := range fileSelectorModel.files {
		if fileSelectorModel.selected[i] {
			selectedFiles = append(selectedFiles, fileSelectorModel.files[i])
		}
	}

	if len(selectedFiles) == 0 {
		println("No files selected.")
		return
	}

	aiModel := NewAIMessageModel(ctx, selectedFiles, provider, editorMode)
	aiModelProgram := tea.NewProgram(&aiModel, tea.WithContext(ctx))

	finalModel, err := aiModelProgram.Run()
	if err != nil {
		panic(err)
	}

	if m, ok := finalModel.(*AIMessageModel); ok && m.state == StatePushed {
		re := regexp.MustCompile(`remote:\s*(https?://\S+)`)
		matches := re.FindAllStringSubmatch(m.pushOutput, -1)
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
