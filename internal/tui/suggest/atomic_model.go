package suggest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/git"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

type AtomicState int

const (
	AtomicStateFetching AtomicState = iota
	AtomicStateGenerating
	AtomicStateReviewing
	AtomicStateEditing
	AtomicStateExecuting
	AtomicStateDone
	AtomicStatePushing
	AtomicStatePushed
	AtomicStateError
)

type (
	atomicPlanMsg []ai.AtomicCommit
	atomicExecMsg struct{ err error }
	atomicPushMsg struct {
		err    error
		output string
	}
)

type AtomicGitService interface {
	GetHunks(files []string) ([]git.DiffHunk, error)
	ApplyHunks(hunks []git.DiffHunk) error
	CommitStaged(message string) error
	Push(ctx context.Context, remoteName string) (string, error)
	HasRemotes() (bool, error)
}

type AtomicModel struct {
	generator   AtomicGenerator
	gitService  AtomicGitService
	ctx         context.Context
	files       []string
	hint        string
	hunks       []git.DiffHunk
	hunkMap     map[int]git.DiffHunk
	commits     []ai.AtomicCommit
	cursor      int
	state       AtomicState
	spinner     spinner.Model
	errMsg      string
	textArea    textarea.Model
	editorMode  string
	hunksString string
	pushOutput  string
	hasRemotes  bool
	verbose     bool
}

func NewAtomicModel(ctx context.Context, files []string, generator AtomicGenerator, gs AtomicGitService, editorMode, hint string, hasRemotes bool, verbose bool) AtomicModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = shared.CursorStyle

	ta := textarea.New()
	ta.Placeholder = "Enter commit message..."
	ta.ShowLineNumbers = false
	ta.SetWidth(80)
	ta.SetHeight(5)

	return AtomicModel{
		ctx:        ctx,
		files:      files,
		generator:  generator,
		gitService: gs,
		hint:       hint,
		state:      AtomicStateFetching,
		spinner:    s,
		textArea:   ta,
		editorMode: editorMode,
		hasRemotes: hasRemotes,
		verbose:    verbose,
	}
}

func (m *AtomicModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchHunksCmd())
}

func (m *AtomicModel) fetchHunksCmd() tea.Cmd {
	return func() tea.Msg {
		hunks, err := m.gitService.GetHunks(m.files)
		if err != nil {
			return atomicExecMsg{err: err}
		}
		return hunks
	}
}

func (m *AtomicModel) generatePlanCmd() tea.Cmd {
	return func() tea.Msg {
		commits, _, err := m.generator.GenerateAtomic(m.ctx, m.hunksString, m.hint)
		if err != nil {
			return atomicExecMsg{err: err}
		}
		return atomicPlanMsg(commits)
	}
}

func (m *AtomicModel) executePlanCmd() tea.Cmd {
	return func() tea.Msg {
		for i, c := range m.commits {
			var commitHunks []git.DiffHunk
			for _, hid := range c.HunkIDs {
				if h, ok := m.hunkMap[hid]; ok {
					commitHunks = append(commitHunks, h)
				}
			}

			if err := m.gitService.ApplyHunks(commitHunks); err != nil {
				return atomicExecMsg{err: fmt.Errorf("apply failed at commit %d: %w", i+1, err)}
			}

			if err := m.gitService.CommitStaged(c.Message); err != nil {
				return atomicExecMsg{err: fmt.Errorf("commit failed at commit %d: %w", i+1, err)}
			}
		}
		return atomicExecMsg{err: nil}
	}
}

func (m *AtomicModel) pushCmd() tea.Cmd {
	return func() tea.Msg {
		// Defaulting to "origin" and no force push for atomic flow as it builds on top of HEAD
		out, err := m.gitService.Push(m.ctx, "origin")
		return atomicPushMsg{err: err, output: out}
	}
}

func (m *AtomicModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == AtomicStateEditing && m.editorMode == "builtin" {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc":
				m.state = AtomicStateReviewing
				return m, nil
			case "ctrl+s":
				m.commits[m.cursor].Message = m.textArea.Value()
				m.state = AtomicStateReviewing
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
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.commits)-1 {
				m.cursor++
			}
		case "shift+up", "K":
			if m.cursor > 0 {
				m.commits[m.cursor], m.commits[m.cursor-1] = m.commits[m.cursor-1], m.commits[m.cursor]
				m.cursor--
			}
		case "shift+down", "J":
			if m.cursor < len(m.commits)-1 {
				m.commits[m.cursor], m.commits[m.cursor+1] = m.commits[m.cursor+1], m.commits[m.cursor]
				m.cursor++
			}
		case "e":
			if m.state == AtomicStateReviewing {
				if m.editorMode == "builtin" {
					m.state = AtomicStateEditing
					m.textArea.SetValue(m.commits[m.cursor].Message)
					m.textArea.Focus()
					return m, textarea.Blink
				}
				return m, OpenEditor(m.commits[m.cursor].Message, m.editorMode)
			}
		case "r":
			if m.state == AtomicStateReviewing || m.state == AtomicStateError {
				m.state = AtomicStateGenerating
				m.errMsg = ""
				return m, tea.Batch(m.spinner.Tick, m.generatePlanCmd())
			}
		case "c", "enter":
			if m.state == AtomicStateReviewing {
				m.state = AtomicStateExecuting
				return m, tea.Batch(m.spinner.Tick, m.executePlanCmd())
			}
		case "p":
			if m.state == AtomicStateDone && m.hasRemotes {
				m.state = AtomicStatePushing
				return m, tea.Batch(m.spinner.Tick, m.pushCmd())
			}
		}

	case []git.DiffHunk:
		m.hunks = msg
		if len(m.hunks) == 0 {
			m.state = AtomicStateError
			m.errMsg = "No changes detected relative to HEAD (did you commit already?)"
			return m, nil
		}
		m.hunkMap = make(map[int]git.DiffHunk)
		var sb strings.Builder
		for _, h := range m.hunks {
			m.hunkMap[h.ID] = h
			sb.WriteString(h.String())
			sb.WriteString("\n\n")
		}
		m.hunksString = sb.String()
		m.state = AtomicStateGenerating
		return m, m.generatePlanCmd()

	case atomicPlanMsg:
		m.commits = msg
		m.state = AtomicStateReviewing
		m.cursor = 0
		return m, nil

	case atomicExecMsg:
		if msg.err != nil {
			m.state = AtomicStateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.state = AtomicStateDone
		return m, nil

	case atomicPushMsg:
		if msg.err != nil {
			m.state = AtomicStateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.state = AtomicStatePushed
		m.pushOutput = msg.output
		return m, tea.Quit

	case EditorFinishedMsg:
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
			return m, nil
		}
		content, err := os.ReadFile(msg.Filename)
		os.Remove(msg.Filename)
		if err == nil {
			m.commits[m.cursor].Message = strings.TrimSpace(string(content))
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *AtomicModel) View() string {
	var b strings.Builder

	switch m.state {
	case AtomicStateFetching:
		b.WriteString("\n" + shared.HeaderStyle.Render("Fetching hunks...") + "\n\n" + m.spinner.View() + "\n")
	case AtomicStateGenerating:
		b.WriteString("\n" + shared.HeaderStyle.Render("Generating atomic plan...") + "\n\n" + m.spinner.View() + "\n")
	case AtomicStateExecuting:
		b.WriteString("\n" + shared.HeaderStyle.Render("Applying commits...") + "\n\n" + m.spinner.View() + "\n")
	case AtomicStatePushing:
		b.WriteString("\n" + shared.HeaderStyle.Render("Pushing...") + "\n\n" + m.spinner.View() + "\n")
	case AtomicStatePushed:
		b.WriteString("\n" + shared.HeaderStyle.Render("Pushed successfully!") + "\n")
		b.WriteString(m.pushOutput + "\n")
	case AtomicStateError:
		b.WriteString("\n" + shared.ErrorStyle.Render("Error: "+m.errMsg) + "\n")
		b.WriteString("\n[r] Retry   [q] Quit\n")
	case AtomicStateDone:
		b.WriteString("\n" + shared.HeaderStyle.Render("Success! All commits applied.") + "\n")
		if m.hasRemotes {
			b.WriteString("\n[p] Push   [q] Quit\n")
		} else {
			b.WriteString("\n[q] Quit\n")
		}
	case AtomicStateEditing:
		b.WriteString("\n" + shared.HeaderStyle.Render("Edit commit message:") + "\n\n")
		b.WriteString(m.textArea.View() + "\n\n")
		b.WriteString("(ctrl+s to save, esc to cancel)\n")
	case AtomicStateReviewing:
		b.WriteString("\n" + shared.HeaderStyle.Render("Proposed Atomic Commits:") + "\n\n")
		for i, c := range m.commits {
			cursor := " "
			style := shared.NormalTextStyle
			if i == m.cursor {
				cursor = ">"
				style = shared.SelectedTextStyle
			}

			b.WriteString(fmt.Sprintf("%s %d. %s\n", cursor, i+1, style.Render(c.Message)))
			for _, hid := range c.HunkIDs {
				if h, ok := m.hunkMap[hid]; ok {
					b.WriteString(fmt.Sprintf("      - %s (lines %d-%d)\n", h.File, h.StartLine, h.EndLine))
				}
			}
			b.WriteString("\n")
		}
		b.WriteString("\n[e] Edit   [J/K] Move Up/Down   [r] Regenerate   [c] Confirm & Apply   [q] Quit\n")
	}

	return b.String()
}
