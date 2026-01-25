package suggest

import (
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type EditorFinishedMsg struct {
	Err      error
	Filename string
}

func OpenEditor(content string, editorCmd string) tea.Cmd {
	f, err := os.CreateTemp("", "gitai-commit-msg-*.txt")
	if err != nil {
		return func() tea.Msg { return EditorFinishedMsg{Err: err} }
	}

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())

		return func() tea.Msg { return EditorFinishedMsg{Err: err} }
	}

	f.Close()

	editor := editorCmd
	if editor == "" || editor == "system" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}

		if editor == "" {
			editor = "vim"
		}
	}

	parts := strings.Fields(editor)

	var c *exec.Cmd

	if len(parts) > 0 {
		args := make([]string, 0, len(parts))
		args = append(args, parts[1:]...)
		args = append(args, f.Name())

		c = exec.Command(parts[0], args...)
	} else {
		c = exec.Command(editor, f.Name())
	}

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return EditorFinishedMsg{Err: err, Filename: f.Name()}
	})
}
