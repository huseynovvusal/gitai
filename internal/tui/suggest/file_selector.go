package suggest

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	"huseynovvusal/gitai/internal/tui/multilist"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type FileSelectorModel struct {
	MultiList multilist.Model
	quitting  bool
	done      bool
}

func NewFileSelectorModel(files []string) FileSelectorModel {
	return FileSelectorModel{
		MultiList: multilist.New(
			files,
			"Select files to include in commit",
			multilist.WithHeight(15),
			multilist.WithPaginatorType(paginator.Arabic),
		),
		quitting: false,
		done:     false,
	}
}

func (m *FileSelectorModel) Init() tea.Cmd {
	return nil
}

func (m *FileSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.MultiList.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		// Handle global keys for the specific screen here
		if m.MultiList.List.FilterState() != list.Filtering {
			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				// Validation: Ensure at least one file is selected
				if len(m.MultiList.GetSelected()) > 0 {
					m.done = true
					return m, tea.Quit
				}
			}
		}
	}

	// Forward everything else to the wrapper
	var cmd tea.Cmd
	m.MultiList, cmd = m.MultiList.Update(msg)
	return m, cmd
}

func (m *FileSelectorModel) View() string {
	if m.quitting {
		return ""
	}

	if m.done {
		var b strings.Builder
		b.WriteString(shared.HeaderStyle.Render("Selected files:") + "\n")
		for _, f := range m.GetSelectedFiles() {
			b.WriteString(fmt.Sprintf(" - %s\n", f))
		}
		return b.String()
	}

	return docStyle.Render(m.MultiList.View())
}

func (m *FileSelectorModel) GetSelectedFiles() []string {
	return m.MultiList.GetSelected()
}
