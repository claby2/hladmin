package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/claby2/hladmin/internal/executor"
)

// TableSpec describes how to turn per-host execution state into table rows. The
// caller owns the column semantics and cell styling; this package owns layout and
// animation.
type TableSpec struct {
	Headers []string
	// CompletedRow returns the cells for a finished host.
	CompletedRow func(executor.Result) []string
	// RunningRow returns the cells for a host still in progress. spinnerFrame is
	// the current spinner glyph.
	RunningRow func(host, spinnerFrame string, elapsed time.Duration) []string
}

// renderTable lays out headers and rows with a compact, borderless style. When
// maxWidth > 0 and the natural table is wider, it is constrained so no line
// exceeds the terminal width.
func renderTable(spec TableSpec, rows [][]string, maxWidth int) string {
	lastCol := len(spec.Headers) - 1
	padded := lipgloss.NewStyle().PaddingRight(2)
	plain := lipgloss.NewStyle()
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if col == lastCol {
				return plain
			}
			return padded
		}).
		Headers(spec.Headers...).
		Rows(rows...)

	if maxWidth > 0 && lipgloss.Width(t.String()) > maxWidth {
		t = t.Width(maxWidth)
	}
	return t.String()
}

// tableModel renders a live, in-place table that fills in as hosts complete.
type tableModel struct {
	hosts     []string
	command   string
	spec      TableSpec
	spinner   spinner.Model
	starts    []time.Time
	done      []bool
	results   []executor.Result
	remaining int
	width     int
}

func (m tableModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	for i, host := range m.hosts {
		cmds = append(cmds, runHostCmd(i, host, m.command))
	}
	return tea.Batch(cmds...)
}

func (m tableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hostDoneMsg:
		m.done[msg.index] = true
		m.results[msg.index] = msg.result
		m.remaining--
		if m.remaining == 0 {
			return m, tea.Quit
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m tableModel) rows() [][]string {
	rows := make([][]string, len(m.hosts))
	for i, host := range m.hosts {
		if m.done[i] {
			rows[i] = m.spec.CompletedRow(m.results[i])
		} else {
			rows[i] = m.spec.RunningRow(host, m.spinner.View(), time.Since(m.starts[i]))
		}
	}
	return rows
}

func (m tableModel) View() string {
	return renderTable(m.spec, m.rows(), m.width) + "\n"
}

// RunLiveTable executes command on hosts in parallel and renders a live table via
// spec, redrawing in place as hosts complete (TTY). On a non-TTY it renders the
// final table once after all hosts finish.
func RunLiveTable(hosts []string, command string, spec TableSpec) ([]executor.Result, error) {
	if err := executor.VerifyHostsAndCommand(hosts, command); err != nil {
		return nil, err
	}

	if !IsTerminal() {
		results := runAllParallel(hosts, command)
		rows := make([][]string, len(results))
		for i, r := range results {
			rows[i] = spec.CompletedRow(r)
		}
		fmt.Println(renderTable(spec, rows, 0))
		return results, nil
	}

	now := time.Now()
	starts := make([]time.Time, len(hosts))
	for i := range starts {
		starts[i] = now
	}

	m := tableModel{
		hosts:     hosts,
		command:   command,
		spec:      spec,
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
	return final.(tableModel).results, nil
}
