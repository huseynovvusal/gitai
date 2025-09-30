package suggest

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	checkedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	fileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	selectedStyle = lipgloss.NewStyle().Bold(true)
)

type FileSelectorModel struct {
	files    []string
	selected map[int]bool
	cursor   int
	quitting bool
	done     bool
}

func NewFileSelectorModel(files []string) FileSelectorModel {
	return FileSelectorModel{
		files:    files,
		selected: make(map[int]bool),
		cursor:   0,
		quitting: false,
	}
}

func (m *FileSelectorModel) Init() tea.Cmd {
	return nil
}

func (m *FileSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			var all = true
			for i := range m.files {
				if !m.selected[i] {
					all = false
					break
				}
			}
			for i := range m.files {
				m.selected[i] = !all
			}
		case "enter":
			if m.anySelected() {
				m.done = true
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *FileSelectorModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	if m.done {
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("Selected files for commit:")
		b.WriteString("\n" + header + "\n")

		for i, file := range m.files {
			if m.selected[i] {
				line := fmt.Sprintf(" - %s", fileStyle.Render(file))
				b.WriteString(line + "\n")
			}
		}

		return b.String()
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("Select files to include in commit:")
	b.WriteString("\n" + header + "\n")

	for i, file := range m.files {
		var checked string

		if m.selected[i] {
			checked = checkedStyle.Render("[x]")
		} else {
			checked = checkedStyle.Render("[ ]")
		}

		cursor := " "

		line := fmt.Sprintf("%s %s %s", cursor, checked, fileStyle.Render(file))

		if m.cursor == i {
			cursor = cursorStyle.Render(">")
			line = selectedStyle.Render(fmt.Sprintf("%s %s %s", cursor, checked, fileStyle.Render(file)))
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n[a] Select all   [space] Toggle   [enter] OK   [q] Quit\n")

	return b.String()
}

func (m *FileSelectorModel) anySelected() bool {
	for _, selected := range m.selected {
		if selected {
			return true
		}
	}

	return false
}

func (m *FileSelectorModel) GetSelectedFiles() []string {
	var selectedFiles []string

	for i, selected := range m.selected {
		if selected && i < len(m.files) {
			selectedFiles = append(selectedFiles, m.files[i])
		}
	}

	return selectedFiles
}
