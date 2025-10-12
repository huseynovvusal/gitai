package suggest

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/security"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

type aiDoneMsg struct {
	message string
}

type aiErrorMsg struct {
	err error
}

type commitResultMsg struct {
	err error
}

type pushResultMsg struct {
	err error
}

type commitSecurityWarningMsg struct {
	err    error
	diff   string
	status string
}

type State int

const (
	StateGenerating      State = iota // waiting for AI generation
	StateGenerated                    // AI generated, ready to commit / edit
	StateCommitting                   // commit running
	StateCommitted                    // commit succeeded; show commit message and push/cancel options
	StatePushing                      // push running
	StatePushed                       // push succeeded; show success and exit option
	StateError                        // show error (store message)
	StateSecurityWarning              // warn and prompt the user for confirmation regarding safety reasons of the code being committed
)

type AIMessageModel struct {
	files         []string
	commitMessage string
	state         State
	spinner       spinner.Model
	errMsg        string
	cancel        bool
	provider      ai.Provider
	savedDiff     string
	savedStatus   string
	ctx           context.Context
}

func NewAIMessageModel(ctx context.Context, files []string, provider ai.Provider) AIMessageModel {
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
		ctx:           ctx,
		provider:      provider,
	}
}

func runAIAsync(ctx context.Context, provider ai.Provider, files []string) tea.Cmd {
	return func() tea.Msg {
		diff, err := git.GetChangesForFiles(files)
		if err != nil {
			return aiErrorMsg{err: err}
		}

		status, err := git.GetStatusForFiles(files)
		if err != nil {
			return aiErrorMsg{err: err}
		}

		err = security.CheckDiffSafety(diff)
		if err != nil {
			return commitSecurityWarningMsg{err: err, diff: diff, status: status}
		}

		commitMessage, err := ai.GenerateCommitMessage(ctx, provider, diff, status)
		if err != nil {
			return aiErrorMsg{err: err}
		}

		return aiDoneMsg{message: commitMessage}
	}
}

// runGenerateAfterWarningAsync resumes commit message generation using the
// previously saved diff/status after the user confirmed the warning.
func runGenerateAfterWarningAsync(ctx context.Context, provider ai.Provider, diff, status string) tea.Cmd {
	return func() tea.Msg {
		commitMessage, err := ai.GenerateCommitMessage(ctx, provider, diff, status)
		if err != nil {
			return aiErrorMsg{err: err}
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
		runAIAsync(m.ctx, m.provider, m.files),
	)
}

func (m *AIMessageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "x":
			m.cancel = true
			return m, tea.Quit
		case "y", "enter":
			if m.state == StateSecurityWarning {
				m.state = StateGenerating
				m.errMsg = ""
				return m, runGenerateAfterWarningAsync(m.ctx, m.provider, m.savedDiff, m.savedStatus)
			}
		case "n":
			if m.state == StateSecurityWarning {
				m.state = StateError
				m.errMsg = "Commit cancelled by user due to security findings"
				return m, nil
			}
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
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case aiDoneMsg:
		m.commitMessage = msg.message
		m.state = StateGenerated
		return m, nil

	case aiErrorMsg:
		m.state = StateError
		m.errMsg = msg.err.Error()
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
	case commitSecurityWarningMsg:
		if msg.err != nil {
			// save context so we can resume generation if the user confirms
			m.savedDiff = msg.diff
			m.savedStatus = msg.status
			m.state = StateSecurityWarning
			m.errMsg = msg.err.Error()
			return m, nil
		}
	}

	return m, nil
}

func (m *AIMessageModel) View() string {
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
	case StateSecurityWarning:
		var b strings.Builder
		header := shared.HeaderStyle.Render("Warning, potential sensitive data detected in added lines:")
		b.WriteString("\n" + header + "\n")
		b.WriteString(m.errMsg + "\n")
		b.WriteString("\nDo you wish to continue?\n")
		b.WriteString("\n[Y] yes   [n] no\n")
		return b.String()
	default:
		// fallback - shouldn't happen
		return "\n" + shared.HeaderStyle.Render("Unknown state") + "\n"
	}
}
