package shared

import "github.com/charmbracelet/lipgloss"

var (
	BaseFg     = lipgloss.Color("#cbe3e7")
	BaseBg     = lipgloss.Color("#1e1e28")
	AccentFg   = lipgloss.Color("#f6c177")
	CursorFg   = lipgloss.Color("#eb6f92")
	CheckedFg  = lipgloss.Color("#9ccfd8")
	FileFg     = lipgloss.Color("#31748f")
	SelectedBg = lipgloss.Color("#26233a")
	ErrorFg    = lipgloss.Color("#eb6f92")
)

var (
	// Header: prominent, but leave background to the surrounding layout
	HeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(AccentFg)
	// Cursor: highlight the pointer with a bright foreground and bold
	CursorStyle = lipgloss.NewStyle().Foreground(CursorFg).Bold(true)
	// Checked/File/Error: plain foreground colors, no background so they blend with whatever layout is used
	CheckedStyle = lipgloss.NewStyle().Foreground(CheckedFg)
	FileStyle    = lipgloss.NewStyle().Foreground(FileFg)
	// Selected: keep a distinct background to clearly show selection
	SelectedStyle = lipgloss.NewStyle().Bold(true).Background(SelectedBg).Foreground(BaseFg)
	ErrorStyle    = lipgloss.NewStyle().Foreground(ErrorFg).Bold(true)
)
