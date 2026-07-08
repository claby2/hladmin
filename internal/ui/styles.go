package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Semantic styles used across the CLI. These mirror the previous fatih/color
// palette but are Lip Gloss styles rendered via .Render().
var (
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	Error   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Info    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	Bold      = lipgloss.NewStyle().Bold(true)
	Header    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	Hostname  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	Secondary = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func init() {
	// Disable color when explicitly requested (NO_COLOR is the standard,
	// HLADMIN_NO_COLOR is a tool-specific override). Lip Gloss already downgrades
	// to no color when stdout is not a TTY.
	if os.Getenv("NO_COLOR") != "" || os.Getenv("HLADMIN_NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// IsTerminal reports whether stdout is connected to a terminal.
func IsTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
