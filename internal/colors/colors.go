package colors

import (
	"os"
	"strconv"
	"strings"

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

func ColorizeUsage(usage string) string {
	if usage == "error" {
		return Error.Sprint(usage)
	}

	// Try to parse percentage for disk/memory usage coloring
	if strings.HasSuffix(usage, "%") {
		if percent, err := strconv.Atoi(strings.TrimSuffix(usage, "%")); err == nil {
			if percent >= 90 {
				return Error.Sprint(usage) // Red for high usage
			} else if percent >= 70 {
				return Warning.Sprint(usage) // Yellow for medium usage
			} else {
				return Success.Sprint(usage) // Green for low usage
			}
		}
	}

	return usage
}

