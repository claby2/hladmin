package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/claby2/hladmin/internal/executor"
)

// FormatDuration renders a duration compactly, e.g. "4.2s" or "1m03s".
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%02ds", m, s)
}

// resultHeader renders the "===» host" / "cmd: ..." banner for a host.
func resultHeader(hostname, command string) string {
	return fmt.Sprintf("%s %s\n%s %s",
		Header.Render("===»"), Hostname.Render(hostname),
		Secondary.Render("cmd:"), command)
}

// prefixedOutput renders output with a per-line "host |" prefix. The result ends
// without a trailing newline.
func prefixedOutput(hostname, output string) string {
	prefix := fmt.Sprintf("%s %s ", Hostname.Render(hostname), Secondary.Render("|"))
	lines := strings.Split(output, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(prefix + line)
	}
	return b.String()
}

// renderResultBlock renders a host's full output block: header, prefixed
// stdout/stderr, and a colored done/error footer with the duration.
func renderResultBlock(result executor.Result) string {
	var b strings.Builder
	b.WriteString(resultHeader(result.Hostname, result.Command))

	if result.Stdout != "" {
		b.WriteByte('\n')
		b.WriteString(prefixedOutput(result.Hostname, result.Stdout))
	}
	if result.Stderr != "" {
		b.WriteByte('\n')
		b.WriteString(prefixedOutput(result.Hostname, result.Stderr))
	}

	dur := Secondary.Render(FormatDuration(result.Duration))
	b.WriteByte('\n')
	if result.Err != nil {
		b.WriteString(fmt.Sprintf("%s %s", Error.Render(result.Err.Error()), dur))
	} else {
		b.WriteString(fmt.Sprintf("%s %s", Success.Render("✓ done"), dur))
	}
	return b.String()
}
