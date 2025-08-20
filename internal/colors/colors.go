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
	// Respect NO_COLOR environment variable (standard)
	if os.Getenv("NO_COLOR") != "" {
		disableColors()
		return
	}

	// Respect HLADMIN_NO_COLOR for tool-specific override
	if os.Getenv("HLADMIN_NO_COLOR") != "" {
		disableColors()
		return
	}

	// Auto-detect if output is not a TTY
	if !isTerminal() {
		disableColors()
	}
}

func disableColors() {
	color.NoColor = true
}

func isTerminal() bool {
	// Check if stdout is a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
