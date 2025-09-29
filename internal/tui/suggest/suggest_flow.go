package suggest

import (
	"huseynovvusal/gitai/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

func RunSuggestFlow() {
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

	aiModel := NewAIMessageModel(selectedFiles)
	aiModelProgram := tea.NewProgram(&aiModel)

	_, err = aiModelProgram.Run()
	if err != nil {
		panic(err)
	}

}
