package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/claby2/hladmin/internal/executor"
)

// streamModel runs a command on many hosts in parallel, printing each host's full
// output block to scrollback as it finishes while keeping a live spinner block for
// the hosts still running.
type streamModel struct {
	hosts     []string
	command   string
	spinner   spinner.Model
	starts    []time.Time
	done      []bool
	results   []executor.Result
	remaining int
}

func (m streamModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	for i, host := range m.hosts {
		cmds = append(cmds, runHostCmd(i, host, m.command))
	}
	return tea.Batch(cmds...)
}

func (m streamModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hostDoneMsg:
		m.done[msg.index] = true
		m.results[msg.index] = msg.result
		m.remaining--
		// Push the finished block into scrollback above the live region.
		printBlock := tea.Println(renderResultBlock(msg.result))
		if m.remaining == 0 {
			return m, tea.Sequence(printBlock, tea.Quit)
		}
		return m, printBlock
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m streamModel) View() string {
	var b strings.Builder
	for i, host := range m.hosts {
		if m.done[i] {
			continue
		}
		b.WriteString(fmt.Sprintf("%s %s  %s\n",
			m.spinner.View(),
			Hostname.Render(host),
			Secondary.Render(FormatDuration(time.Since(m.starts[i])))))
	}
	return b.String()
}

// RunStreaming executes command on hosts in parallel. On a TTY it shows a live
// spinner block for running hosts and streams each host's output block as it
// completes. On a non-TTY it prints each block as it finishes with no animation.
func RunStreaming(hosts []string, command string) ([]executor.Result, error) {
	if err := executor.VerifyHostsAndCommand(hosts, command); err != nil {
		return nil, err
	}

	if !IsTerminal() {
		return runStreamingPlain(hosts, command), nil
	}

	now := time.Now()
	starts := make([]time.Time, len(hosts))
	for i := range starts {
		starts[i] = now
	}

	m := streamModel{
		hosts:     hosts,
		command:   command,
		spinner:   newSpinner(),
		starts:    starts,
		done:      make([]bool, len(hosts)),
		results:   make([]executor.Result, len(hosts)),
		remaining: len(hosts),
	}

	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, err
	}
	return final.(streamModel).results, nil
}

func runStreamingPlain(hosts []string, command string) []executor.Result {
	results := make([]executor.Result, len(hosts))
	done := make(chan int, len(hosts))
	for i, host := range hosts {
		go func(i int, host string) {
			results[i] = executor.RunOnHost(host, command)
			done <- i
		}(i, host)
	}
	for range hosts {
		i := <-done
		fmt.Println(renderResultBlock(results[i]))
		fmt.Println()
	}
	return results
}
