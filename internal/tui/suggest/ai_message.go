package suggest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/ai/provider"
	"huseynovvusal/gitai/internal/cleaner"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/security"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

type aiDoneMsg struct {
	message string
	version string
	usage   provider.Usage
}

type aiTokenMsg struct {
	token   string
	version string
}

type aiStreamDoneMsg struct {
	version string
}

type aiUsageMsg struct {
	usage provider.Usage
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
	err     error
	diff    string
	status  string
	version string
}

type startStreamMsg struct {
	params runAIParams
}

type streamMsg struct {
	stream  <-chan provider.StreamResult
	version string
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

type messageGitService interface {
	GitDiffStatus
	GitCommitter
	GetAmendChangesForFiles(files []string) (string, error)
	CommitAmend(files []string, message string) error
	PushForce(ctx context.Context, remoteName string) (string, error)
}

type AIMessageModel struct {
	files         []string
	commitMessage string
	state         State
	spinner       spinner.Model
	errMsg        string
	cancel        bool
	generator     ai.CommitMessageGenerator
	gitService    messageGitService
	config        MessageConfig
	savedDiff     string
	savedStatus   string
	savedVersion  string
	ctx           context.Context
	textArea      textarea.Model
	hint          string
	pushOutput    string
	usage         provider.Usage
	bulletPoint   string
}

type MessageConfig struct {
	EditorMode       string
	SecurityKeywords []string
	Amend            bool
	ForcePush        bool
	Verbose          bool
	BulletPoint      string
}

func NewAIMessageModel(ctx context.Context, files []string, generator ai.CommitMessageGenerator, gs messageGitService, cfg MessageConfig, hint string) AIMessageModel {
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
		generator:     generator,
		gitService:    gs,
		config:        cfg,
		textArea:      ta,
		hint:          hint,
		bulletPoint:   cfg.BulletPoint,
	}
}

type runAIParams struct {
	ctx              context.Context
	generator        ai.CommitMessageGenerator
	gitService       messageGitService
	files            []string
	securityKeywords []string
	hint             string
	amend            bool
}

func runAIStreamAsync(p runAIParams) tea.Cmd {
	return func() tea.Msg {
		var diff string
		var err error
		if p.amend {
			diff, err = p.gitService.GetAmendChangesForFiles(p.files)
		} else {
			diff, err = p.gitService.GetChangesForFiles(p.files)
		}
		if err != nil {
			return aiErrorMsg{err: err}
		}

		status, err := p.gitService.GetStatusForFiles(p.files)
		if err != nil {
			return aiErrorMsg{err: err}
		}

		version := git.ExtractVersionFromDiff(diff)

		err = security.CheckDiffSafety(diff, p.securityKeywords)
		if err != nil {
			return commitSecurityWarningMsg{err: err, diff: diff, status: status, version: version}
		}

		stream, err := p.generator.Stream(p.ctx, diff, status, p.hint, version)
		if err != nil {
			// Fallback to non-stream if streaming fails
			msg, usage, err := p.generator.Generate(p.ctx, diff, status, p.hint, version)
			if err != nil {
				return aiErrorMsg{err: err}
			}
			return aiDoneMsg{message: msg, version: version, usage: usage}
		}

		return streamMsg{stream: stream, version: version}
	}
}

func readStream(stream <-chan provider.StreamResult, version string) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-stream
		if !ok {
			return aiStreamDoneMsg{version: version}
		}
		if res.Err != nil {
			return aiErrorMsg{err: res.Err}
		}
		if res.Token != "" {
			return aiTokenMsg{token: res.Token, version: version}
		}
		if res.Usage.TotalTokens > 0 {
			return aiUsageMsg{usage: res.Usage}
		}
		// If we get an empty result but channel is not closed, keep reading.
		return readStream(stream, version)()
	}
}

func runAIAsync(p runAIParams) tea.Cmd {
	return func() tea.Msg {
		return startStreamMsg{params: p}
	}
}

func runGenerateAfterWarningAsync(ctx context.Context, generator ai.CommitMessageGenerator, diff, status, hint, version string) tea.Cmd {
	return func() tea.Msg {
		commitMessage, usage, err := generator.Generate(ctx, diff, status, hint, version)
		if err != nil {
			return aiErrorMsg{err: err}
		}

		return aiDoneMsg{message: commitMessage, version: version, usage: usage}
	}
}

func runCommitAsync(gs messageGitService, files []string, message string, amend bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if amend {
			err = gs.CommitAmend(files, message)
		} else {
			err = gs.Commit(files, message)
		}

		return commitResultMsg{err: err}
	}
}

func runPushAsync(ctx context.Context, gs messageGitService, remote string, force bool) tea.Cmd {
	return func() tea.Msg {
		var out string
		var err error
		if force {
			out, err = gs.PushForce(ctx, remote)
		} else {
			out, err = gs.Push(ctx, remote)
		}

		return pushResultMsg{err: err, output: out}
	}
}

func (m *AIMessageModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		runAIAsync(runAIParams{
			ctx:              m.ctx,
			generator:        m.generator,
			gitService:       m.gitService,
			files:            m.files,
			securityKeywords: m.config.SecurityKeywords,
			hint:             m.hint,
			amend:            m.config.Amend,
		}),
	)
}

func (m *AIMessageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == StateEditing {
		if msg, ok := msg.(tea.KeyMsg); ok {
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
				return m, runGenerateAfterWarningAsync(m.ctx, m.generator, m.savedDiff, m.savedStatus, m.hint, m.savedVersion)
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
				return m, tea.Batch(m.spinner.Tick, runCommitAsync(m.gitService, m.files, m.commitMessage, m.config.Amend))
			}
		case "p":
			if m.state == StateCommitted {
				m.state = StatePushing
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, runPushAsync(m.ctx, m.gitService, "origin", m.config.ForcePush))
			}
		case "e":
			if m.state == StateGenerated {
				if m.config.EditorMode == "builtin" {
					m.state = StateEditing
					m.textArea.SetValue(m.commitMessage)
					m.textArea.Focus()
					return m, textarea.Blink
				} else {
					return m, OpenEditor(m.commitMessage, m.config.EditorMode)
				}
			}
		case "r":
			if m.state == StateGenerated {
				m.commitMessage = ""
				m.state = StateGenerating
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, runAIAsync(runAIParams{
					ctx:              m.ctx,
					generator:        m.generator,
					gitService:       m.gitService,
					files:            m.files,
					securityKeywords: m.config.SecurityKeywords,
					hint:             m.hint,
				}))
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case startStreamMsg:
		return m, runAIStreamAsync(msg.params)

	case streamMsg:
		return m, readStream(msg.stream, msg.version)

	case aiTokenMsg:
		m.commitMessage += msg.token
		return m, nil

	case aiStreamDoneMsg:
		m.commitMessage = cleaner.CleanCommitMessage(m.commitMessage, m.bulletPoint)
		m.state = StateGenerated
		return m, nil

	case aiUsageMsg:
		m.usage = msg.usage
		return m, nil

	case aiDoneMsg:
		m.commitMessage = msg.message
		m.savedVersion = msg.version
		m.usage = msg.usage
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
		m.state = StateCommitted
		m.errMsg = ""
		return m, nil

	case pushResultMsg:
		if msg.err != nil {
			m.state = StateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.state = StatePushed
		m.pushOutput = msg.output
		m.errMsg = ""
		return m, tea.Quit

	case commitSecurityWarningMsg:
		if msg.err != nil {
			m.savedDiff = msg.diff
			m.savedStatus = msg.status
			m.savedVersion = msg.version
			m.state = StateSecurityWarning
			m.errMsg = msg.err.Error()
			return m, nil
		}

	case EditorFinishedMsg:
		if msg.Err != nil {
			m.state = StateError
			m.errMsg = msg.Err.Error()
			return m, nil
		}
		content, err := os.ReadFile(filepath.Clean(msg.Filename))
		os.Remove(msg.Filename)
		if err != nil {
			m.state = StateError
			m.errMsg = fmt.Errorf("failed to read commit message: %w", err).Error()
			return m, nil
		}
		m.commitMessage = strings.TrimSpace(string(content))
		return m, nil
	}

	return m, nil
}

func (m *AIMessageModel) View() string {
	if m.cancel {
		if m.state == StateCommitted {
			var b strings.Builder
			header := shared.HeaderStyle.Render("Committed successfully:")
			b.WriteString("\n" + header + "\n")
			b.WriteString(m.commitMessage + "\n")
			return b.String()
		}
		return shared.ErrorStyle.Render("Commit cancelled.") + "\n"
	}

	switch m.state {
	case StateGenerating:
		var b strings.Builder
		b.WriteString("\n" + shared.HeaderStyle.Render("Generating commit message...") + "\n\n")
		if m.commitMessage != "" {
			b.WriteString(m.commitMessage + "\n")
		} else {
			b.WriteString(m.spinner.View() + " Generating commit message..." + "\n")
		}
		return b.String()

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
		if m.config.Verbose && m.usage.TotalTokens > 0 {
			fmt.Fprintf(&b, "\nTokens: %d (prompt) + %d (completion) = %d (total)\n",
				m.usage.PromptTokens, m.usage.CompletionTokens, m.usage.TotalTokens)
		}
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
		return "\n" + shared.HeaderStyle.Render("Unknown state") + "\n"
	}
}
