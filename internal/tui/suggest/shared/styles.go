package shared

import "github.com/charmbracelet/lipgloss"

var (
	HeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	CursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	CheckedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	FileStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	SelectedStyle = lipgloss.NewStyle().Bold(true)
	ErrorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
)
