package colors

import (
	"os"

	"github.com/fatih/color"
)

var (
	// Core colors
	Success = color.New(color.FgGreen)
	Error   = color.New(color.FgRed)
	Warning = color.New(color.FgYellow)
	Info    = color.New(color.FgCyan)

	// Text styling
	Bold      = color.New(color.Bold)
	Header    = color.New(color.FgCyan, color.Bold)
	Hostname  = color.New(color.FgYellow, color.Bold)
	Secondary = color.New(color.FgHiBlack)
)

func init() {
	// Disable color when explicitly requested (NO_COLOR is the standard,
	// HLADMIN_NO_COLOR is a tool-specific override) or when stdout is not a TTY.
	if os.Getenv("NO_COLOR") != "" || os.Getenv("HLADMIN_NO_COLOR") != "" || !IsTerminal() {
		color.NoColor = true
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
