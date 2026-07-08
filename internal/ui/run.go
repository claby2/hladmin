package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/claby2/hladmin/internal/executor"
)

// tickInterval matches the previous hand-rolled spinner cadence.
const tickInterval = 100 * time.Millisecond

// brailleSpinner reproduces the braille dot spinner used previously.
var brailleSpinner = spinner.Spinner{
	Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
	FPS:    tickInterval,
}

func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = brailleSpinner
	return s
}

// hostDoneMsg reports that host index finished with the given result.
type hostDoneMsg struct {
	index  int
	result executor.Result
}

// runHostCmd returns a tea.Cmd that executes command on host and reports
// completion as a hostDoneMsg.
func runHostCmd(index int, host, command string) tea.Cmd {
	return func() tea.Msg {
		return hostDoneMsg{index: index, result: executor.RunOnHost(host, command)}
	}
}

// runAllParallel executes command on every host concurrently and returns the
// results in host order once all have finished.
func runAllParallel(hosts []string, command string) []executor.Result {
	results := make([]executor.Result, len(hosts))
	done := make(chan int, len(hosts))
	for i, host := range hosts {
		go func(i int, host string) {
			results[i] = executor.RunOnHost(host, command)
			done <- i
		}(i, host)
	}
	for range hosts {
		<-done
	}
	return results
}
