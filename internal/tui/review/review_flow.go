package review

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"huseynovvusal/gitai/internal/ai"
	"huseynovvusal/gitai/internal/tui/suggest/shared"
)

// GitDiffService is the subset of git operations needed for review.
type GitDiffService interface {
	GetChangedFiles() ([]string, error)
	ResolvePath(path string) ([]string, error)
	GetChangesForFiles(files []string) (string, error)
}

// reviewDoneMsg carries the completed review result.
type reviewDoneMsg struct {
	result *ai.ReviewResult
}

// reviewErrorMsg carries an error from the review process.
type reviewErrorMsg struct {
	err error
}

type reviewState int

const (
	stateReviewing reviewState = iota
	stateResults
	stateError
)

var (
	criticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#eb6f92")).Bold(true)
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f6c177")).Bold(true)
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ccfd8"))
	fileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#31748f")).Bold(true)
	suggStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#c4a7e7")).Italic(true)
)

// Model is the Bubbletea model for the review results viewer.
type Model struct {
	state     reviewState
	spinner   spinner.Model
	result    *ai.ReviewResult
	errMsg    string
	paginator paginator.Model
	ctx       context.Context
	reviewer  ai.CodeReviewer
	git       GitDiffService
	files     []string
	hint      string
}

// NewModel creates a new review Model.
func NewModel(ctx context.Context, files []string, reviewer ai.CodeReviewer, git GitDiffService, hint string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = shared.CursorStyle

	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = 10

	return Model{
		state:     stateReviewing,
		spinner:   s,
		paginator: p,
		ctx:       ctx,
		reviewer:  reviewer,
		git:       git,
		files:     files,
		hint:      hint,
	}
}

func runReviewAsync(ctx context.Context, reviewer ai.CodeReviewer, git GitDiffService, files []string, hint string) tea.Cmd {
	return func() tea.Msg {
		diff, err := git.GetChangesForFiles(files)
		if err != nil {
			return reviewErrorMsg{err: err}
		}

		result, err := reviewer.Review(ctx, diff, hint)
		if err != nil {
			return reviewErrorMsg{err: err}
		}

		return reviewDoneMsg{result: result}
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		runReviewAsync(m.ctx, m.reviewer, m.git, m.files, m.hint),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		if m.state == stateResults {
			var cmd tea.Cmd
			m.paginator, cmd = m.paginator.Update(msg)
			return m, cmd
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case reviewDoneMsg:
		m.result = msg.result
		m.state = stateResults
		m.paginator.SetTotalPages(len(m.result.Findings))
		return m, nil

	case reviewErrorMsg:
		m.state = stateError
		m.errMsg = msg.err.Error()
		return m, nil
	}

	return m, nil
}

func (m *Model) View() string {
	switch m.state {
	case stateReviewing:
		return "\n" + shared.HeaderStyle.Render("Reviewing changes...") + "\n\n" +
			m.spinner.View() + " Analyzing diff with AI..." + "\n"

	case stateError:
		var b strings.Builder
		b.WriteString("\n" + shared.HeaderStyle.Render("Review failed:") + "\n")
		b.WriteString(shared.ErrorStyle.Render(m.errMsg) + "\n")
		b.WriteString("\n[q] Quit\n")
		return b.String()

	case stateResults:
		return m.renderResults()

	default:
		return ""
	}
}

func (m *Model) renderResults() string {
	var b strings.Builder

	if len(m.result.Findings) == 0 {
		b.WriteString("\n" + shared.HeaderStyle.Render("Review complete:") + "\n")
		b.WriteString("\nNo issues found. Your changes look good!\n")
		b.WriteString("\n[q] Quit\n")
		return b.String()
	}

	critical, warnings, infos := m.result.Summary()
	b.WriteString("\n" + shared.HeaderStyle.Render("Review results:") + "\n\n")

	grouped := groupByFile(m.result.Findings)
	for _, group := range grouped {
		b.WriteString(fileStyle.Render(group.file) + "\n")
		for _, f := range group.findings {
			icon := severityIcon(f.Severity)
			style := severityStyle(f.Severity)
			b.WriteString(fmt.Sprintf("  %s %s (L%s): %s\n",
				icon,
				style.Render(f.Severity),
				f.Line,
				f.Description,
			))
			if f.Suggestion != "" {
				b.WriteString(fmt.Sprintf("    %s\n", suggStyle.Render(f.Suggestion)))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("Summary: %s, %s, %s across %d file(s)\n",
		criticalStyle.Render(fmt.Sprintf("%d critical", critical)),
		warningStyle.Render(fmt.Sprintf("%d warning(s)", warnings)),
		infoStyle.Render(fmt.Sprintf("%d info", infos)),
		len(grouped),
	))
	b.WriteString("\n[q] Quit\n")

	return b.String()
}

// FormatPlain returns a non-TUI plain text representation of the review results.
func FormatPlain(result *ai.ReviewResult) string {
	if len(result.Findings) == 0 {
		return "No issues found. Your changes look good!\n"
	}

	var b strings.Builder
	grouped := groupByFile(result.Findings)
	for _, group := range grouped {
		b.WriteString(group.file + "\n")
		for _, f := range group.findings {
			icon := severityIcon(f.Severity)
			b.WriteString(fmt.Sprintf("  %s %s (L%s): %s\n", icon, f.Severity, f.Line, f.Description))
			if f.Suggestion != "" {
				b.WriteString(fmt.Sprintf("    Suggestion: %s\n", f.Suggestion))
			}
		}
		b.WriteString("\n")
	}

	critical, warnings, infos := result.Summary()
	b.WriteString(fmt.Sprintf("Summary: %d critical, %d warning(s), %d info across %d file(s)\n",
		critical, warnings, infos, len(grouped)))

	return b.String()
}

// FormatJSON returns the findings as a JSON string.
func FormatJSON(result *ai.ReviewResult) (string, error) {
	data, err := json.MarshalIndent(result.Findings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal findings: %w", err)
	}
	return string(data), nil
}

type fileGroup struct {
	file     string
	findings []ai.Finding
}

func groupByFile(findings []ai.Finding) []fileGroup {
	order := make([]string, 0)
	m := make(map[string][]ai.Finding)

	for _, f := range findings {
		if _, exists := m[f.File]; !exists {
			order = append(order, f.File)
		}
		m[f.File] = append(m[f.File], f)
	}

	groups := make([]fileGroup, 0, len(order))
	for _, file := range order {
		groups = append(groups, fileGroup{file: file, findings: m[file]})
	}
	return groups
}

func severityIcon(s string) string {
	switch s {
	case "critical":
		return "X"
	case "warning":
		return "!"
	case "info":
		return "*"
	default:
		return "?"
	}
}

func severityStyle(s string) lipgloss.Style {
	switch s {
	case "critical":
		return criticalStyle
	case "warning":
		return warningStyle
	case "info":
		return infoStyle
	default:
		return lipgloss.NewStyle()
	}
}

// Flow orchestrates the review process including file selection.
type Flow struct {
	reviewer ai.CodeReviewer
	git      GitDiffService
	hint     string
	format   string
}

// FlowConfig holds configuration for the review flow.
type FlowConfig struct {
	Hint   string
	NoHint bool
	Format string // "text", "json"
}

// NewFlow creates a new review Flow.
func NewFlow(reviewer ai.CodeReviewer, git GitDiffService, cfg FlowConfig) *Flow {
	return &Flow{
		reviewer: reviewer,
		git:      git,
		hint:     cfg.Hint,
		format:   cfg.Format,
	}
}

// Run executes the review flow.
func (f *Flow) Run(ctx context.Context, filesFromArgs []string) {
	changedFiles, err := f.git.GetChangedFiles()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if len(changedFiles) == 0 {
		fmt.Println("No changed files to review.")
		return
	}

	// Determine files
	var selectedFiles []string
	if len(filesFromArgs) > 0 {
		selectedFiles = filterFiles(changedFiles, filesFromArgs, f.git)
	} else {
		selectedFiles = changedFiles
	}

	if len(selectedFiles) == 0 {
		fmt.Println("No valid files selected.")
		return
	}

	// Non-interactive JSON mode
	if f.format == "json" {
		f.runNonInteractive(ctx, selectedFiles)
		return
	}

	// Interactive TUI mode
	model := NewModel(ctx, selectedFiles, f.reviewer, f.git, f.hint)
	p := tea.NewProgram(&model, tea.WithContext(ctx))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running review: %v\n", err)
	}
}

func (f *Flow) runNonInteractive(ctx context.Context, files []string) {
	diff, err := f.git.GetChangesForFiles(files)
	if err != nil {
		fmt.Println("Error getting diff:", err)
		return
	}

	result, err := f.reviewer.Review(ctx, diff, f.hint)
	if err != nil {
		fmt.Println("Error during review:", err)
		return
	}

	out, err := FormatJSON(result)
	if err != nil {
		fmt.Println("Error formatting JSON:", err)
		return
	}
	fmt.Println(out)
}

func filterFiles(available []string, patterns []string, git GitDiffService) []string {
	validMap := make(map[string]bool, len(available))
	for _, f := range available {
		validMap[f] = true
	}

	var selected []string
	for _, arg := range patterns {
		resolved, err := git.ResolvePath(arg)
		if err != nil {
			fmt.Printf("Warning: error resolving file '%s': %v\n", arg, err)
			continue
		}
		for _, r := range resolved {
			if validMap[r] {
				selected = append(selected, r)
			}
		}
	}

	return uniqueStrings(selected)
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	list := make([]string, 0, len(slice))
	for _, entry := range slice {
		if !keys[entry] {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
