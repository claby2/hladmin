package ui

import (
	"fmt"

	"github.com/claby2/hladmin/internal/executor"
)

// RunInteractive executes command on each host sequentially with stdio connected,
// printing a styled header and completion marker around each run. It is not a
// Bubble Tea program because the child process (ssh -t) owns the terminal.
func RunInteractive(hosts []string, command string) error {
	if err := executor.VerifyHostsAndCommand(hosts, command); err != nil {
		return err
	}

	for i, hostname := range hosts {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(resultHeader(hostname, command))
		if err := executor.RunInteractive(hostname, command); err != nil {
			return fmt.Errorf("%s", Error.Render(err.Error()))
		}
		fmt.Println(Success.Render("✓ done"))
	}
	return nil
}
