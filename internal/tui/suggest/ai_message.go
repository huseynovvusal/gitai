package suggest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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
	err    error
	output string
}

type commitSecurityWarningMsg struct {
	err    error
	diff   string
	status string
}

type editorFinishedMsg struct {
	err      error
	filename string
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
	StateEditing                      // Internal editor active
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
	editorMode    string
	textArea      textarea.Model
	hint          string
	pushOutput    string
}

func NewAIMessageModel(ctx context.Context, files []string, provider ai.Provider, editorMode string, hint string) AIMessageModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = shared.CursorStyle

	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.ShowLineNumbers = false
	ta.SetWidth(80)
	ta.SetHeight(10)

	return AIMessageModel{
		files:         files,
		commitMessage: "",
		state:         StateGenerating,
		spinner:       s,
		errMsg:        "",
		cancel:        false,
		ctx:           ctx,
		provider:      provider,
		editorMode:    editorMode,
		textArea:      ta,
		hint:          hint,
	}
}

func runAIAsync(ctx context.Context, provider ai.Provider, files []string, hint string) tea.Cmd {
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

				commitMessage, err := ai.GenerateCommitMessage(ctx, provider, diff, status, hint)
				if err != nil {
					return aiErrorMsg{err: err}
				}
		return aiDoneMsg{message: commitMessage}
	}
}

// runGenerateAfterWarningAsync resumes commit message generation using the
// previously saved diff/status after the user confirmed the warning.
func runGenerateAfterWarningAsync(ctx context.Context, provider ai.Provider, diff, status, hint string) tea.Cmd {
	return func() tea.Msg {
		commitMessage, err := ai.GenerateCommitMessage(ctx, provider, diff, status, hint)
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
		out, err := git.Push()
		return pushResultMsg{err: err, output: out}
	}
}

func openEditor(content string, editorCmd string) tea.Cmd {
	f, err := os.CreateTemp("", "gitai-commit-msg-*.txt")
	if err != nil {
		return func() tea.Msg { return aiErrorMsg{err: err} }
	}

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return func() tea.Msg { return aiErrorMsg{err: err} }
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
		args := append(parts[1:], f.Name())
		c = exec.Command(parts[0], args...)
	} else {
		c = exec.Command(editor, f.Name())
	}

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, filename: f.Name()}
	})
}

func (m *AIMessageModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		runAIAsync(m.ctx, m.provider, m.files, m.hint),
	)
}

func (m *AIMessageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle internal editor state
	if m.state == StateEditing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.state = StateGenerated
				return m, nil
			case "ctrl+s":
				m.commitMessage = m.textArea.Value()
				m.state = StateGenerated
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.textArea, cmd = m.textArea.Update(msg)
		return m, cmd
	}

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
				return m, runGenerateAfterWarningAsync(m.ctx, m.provider, m.savedDiff, m.savedStatus, m.hint)
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
		case "e":
			if m.state == StateGenerated {
				if m.editorMode == "builtin" {
					m.state = StateEditing
					m.textArea.SetValue(m.commitMessage)
					m.textArea.Focus()
					return m, textarea.Blink
				} else {
					return m, openEditor(m.commitMessage, m.editorMode)
				}
			}
		case "r":
			if m.state == StateGenerated {
				m.state = StateGenerating
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, runAIAsync(m.ctx, m.provider, m.files, m.hint))
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
		m.pushOutput = msg.output
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
	case editorFinishedMsg:
		if msg.err != nil {
			m.state = StateError
			m.errMsg = msg.err.Error()
			return m, nil
		}

		content, err := os.ReadFile(msg.filename)
		os.Remove(msg.filename) // Clean up

		if err != nil {
			m.state = StateError
			m.errMsg = err.Error()
			return m, nil
		}

		m.commitMessage = strings.TrimSpace(string(content))
		return m, nil
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
		b.WriteString("\n[e] Edit   [r] Regenerate   [c] Commit   [x] Cancel\n")
		return b.String()
	case StateSecurityWarning:
		var b strings.Builder
		header := shared.HeaderStyle.Render("Warning, potential sensitive data detected in added lines:")
		b.WriteString("\n" + header + "\n")
		b.WriteString(m.errMsg + "\n")
		b.WriteString("\nDo you wish to continue?\n")
		b.WriteString("\n[Y] yes   [n] no\n")
		return b.String()
	case StateEditing:
		return fmt.Sprintf(
			"\n%s\n\n%s\n\n%s",
			shared.HeaderStyle.Render("Edit commit message:"),
			m.textArea.View(),
			"(ctrl+s to save, esc to cancel)",
		)
	default:
		// fallback - shouldn't happen
		return "\n" + shared.HeaderStyle.Render("Unknown state") + "\n"
	}
}
