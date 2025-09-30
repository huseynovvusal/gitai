package suggest

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

type aiDoneMsg struct {
	message string
}

type commitResultMsg struct {
	err error
}

type pushResultMsg struct {
	err error
}

type State int

const (
	StateGenerating State = iota // waiting for AI generation
	StateGenerated               // AI generated, ready to commit / edit
	StateCommitting              // commit running
	StateCommitted               // commit succeeded; show commit message and push/cancel options
	StatePushing                 // push running
	StatePushed                  // push succeeded; show success and exit option
	StateError                   // show error (store message)
)

type AIMessageModel struct {
	files         []string
	commitMessage string
	state         State
	spinner       spinner.Model
	errMsg        string
	cancel        bool
	provider      ai.Provider
}

func NewAIMessageModel(files []string, provider ai.Provider) AIMessageModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = shared.CursorStyle

	return AIMessageModel{
		files:         files,
		commitMessage: "",
		state:         StateGenerating,
		spinner:       s,
		errMsg:        "",
		cancel:        false,
		provider:      provider,
	}
}

func runAIAsync(provider ai.Provider, files []string) tea.Cmd {
	return func() tea.Msg {
		diff, err := git.GetChangesForFiles(files)

		if err != nil {
			panic(err)
		}

		status, err := git.GetStatus()
		if err != nil {
			panic(err)
		}

		commitMessage, err := ai.GenerateCommitMessage(provider, diff, status)
		if err != nil {
			panic(err)
		}

		return aiDoneMsg{message: commitMessage}
	}
}

func runCommitAsync(files []string, message string) tea.Cmd {
	return func() tea.Msg {
		err := git.Commit(files, message)
		return commitResultMsg{err: err}
	}
}

func runPushAsync() tea.Cmd {
	return func() tea.Msg {
		err := git.Push()
		return pushResultMsg{err: err}
	}
}

func (m *AIMessageModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		runAIAsync(m.provider, m.files),
	)
}

func (m *AIMessageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "e":
			// TODO: Open editor to edit the commit message
		case "r":
			// TODO: Regenerate the commit message
		case "c":
			if m.state == StateGenerated && m.commitMessage != "" {
				m.state = StateCommitting
				m.errMsg = ""

				return m, tea.Batch(m.spinner.Tick, runCommitAsync(m.files, m.commitMessage))
			}
		case "p":
			// allow pushing only when we've committed
			if m.state == StateCommitted {
				m.state = StatePushing
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, runPushAsync())
			}
		case "x":
			m.cancel = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case aiDoneMsg:
		m.commitMessage = msg.message
		m.state = StateGenerated
		return m, nil

	case commitResultMsg:
		if msg.err != nil {
			m.state = StateError
			m.errMsg = msg.err.Error()
			return m, nil
		}

		// succeeded: transition to committed view and show commit message
		m.state = StateCommitted
		m.errMsg = ""
		return m, nil

	case pushResultMsg:
		if msg.err != nil {
			m.state = StateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		// push succeeded; transition to pushed state
		m.state = StatePushed
		m.errMsg = ""
		return m, tea.Quit
	}

	return m, nil
}

func (m AIMessageModel) View() string {
	if m.cancel {
		return shared.ErrorStyle.Render("Commit cancelled.") + "\n"
	}

	switch m.state {
	case StateGenerating:
		return "\n" + shared.HeaderStyle.Render("Generating commit message...") + "\n\n" + m.spinner.View() + " Generating commit message..." + "\n"

	case StateCommitting:
		return "\n" + shared.HeaderStyle.Render("Committing...") + "\n\n" + m.spinner.View() + " Committing changes..." + "\n"

	case StatePushing:
		return "\n" + shared.HeaderStyle.Render("Pushing...") + "\n\n" + m.spinner.View() + " Pushing changes..." + "\n"

	case StateError:
		var b strings.Builder
		header := shared.HeaderStyle.Render("Commit failed:")
		b.WriteString("\n" + header + "\n")
		b.WriteString(shared.ErrorStyle.Render(m.errMsg) + "\n")
		b.WriteString("\n[x] Cancel / [q] Quit\n")
		return b.String()

	case StateCommitted:
		var b strings.Builder
		header := shared.HeaderStyle.Render("Committed successfully:")
		b.WriteString("\n" + header + "\n")
		b.WriteString(m.commitMessage + "\n")
		b.WriteString("\n[p] Push   [x] Cancel\n")
		return b.String()

	case StatePushed:
		var b strings.Builder
		header := shared.HeaderStyle.Render("Pushed successfully:")
		b.WriteString("\n" + header + "\n")
		b.WriteString(m.commitMessage + "\n")
		return b.String()

	case StateGenerated:
		var b strings.Builder
		header := shared.HeaderStyle.Render("AI commit message suggestion:")
		b.WriteString("\n" + header + "\n")
		b.WriteString(m.commitMessage + "\n")
		// TODO: Implement edit and regenerate functionality
		// For now, we just show commit and cancel options
		// b.WriteString("\n[e] Edit   [r] Regenerate   [c] Commit   [x] Cancel\n")
		b.WriteString("\n[c] Commit   [x] Cancel\n")
		return b.String()

	default:
		// fallback - shouldn't happen
		return "\n" + shared.HeaderStyle.Render("Unknown state") + "\n"
	}
}
