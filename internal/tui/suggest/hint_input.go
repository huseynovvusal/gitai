package suggest

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
	"strings"
)

type HintInputModel struct {
	textInput textinput.Model

	quitting bool

	done bool

	hint string

	processors []HintProcessor
}

func NewHintInputModel(processors ...HintProcessor) HintInputModel {
	ti := textinput.New()
	ti.Placeholder = "Enter a hint (optional) - e.g. 'bug fix', 'ticket-123'"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return HintInputModel{
		textInput: ti,

		quitting: false,

		done: false,

		processors: processors,
	}

}

func (m *HintInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *HintInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.hint = m.textInput.Value()
			m.done = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *HintInputModel) View() string {
	if m.quitting {
		return ""
	}
	if m.done {
		var b strings.Builder
		header := shared.HeaderStyle.Render("User context instruction provided:")
		b.WriteString("\n" + header + "\n")
		val := m.hint
		if val == "" {
			val = "(none)"
		}
		b.WriteString(val + "\n")
		return b.String()
	}

	return fmt.Sprintf(
		"\n%s\n\n%s\n\n%s",
		shared.HeaderStyle.Render("Provide a user context message for the AI (optional):"),
		m.textInput.View(),
		"(esc to quit)",
	)
}

func (m *HintInputModel) GetHint() string {
	val := m.hint
	for _, p := range m.processors {
		val = p(val)
	}
	return val
}
