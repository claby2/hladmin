package executor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/claby2/hladmin/internal/colors"
)

const tickInterval = 100 * time.Millisecond

var spinnerFrames = spinner.CharSets[14]

// SpinnerFrame returns the spinner character for the current wall-clock time, so
// every live line animates in sync without per-line frame state.
func SpinnerFrame() string {
	idx := (time.Now().UnixMilli() / int64(tickInterval/time.Millisecond)) % int64(len(spinnerFrames))
	return spinnerFrames[idx]
}

// FormatDuration renders a duration compactly, e.g. "4.2s" or "1m03s".
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%02ds", m, s)
}

// liveBlock manages a pinned multi-line region at the bottom of the terminal,
// redrawing it in place via ANSI cursor movement. On a non-TTY it is a no-op so
// callers can print plain output.
type liveBlock struct {
	enabled   bool
	prevLines int
}

func newLiveBlock() *liveBlock {
	return &liveBlock{enabled: colors.IsTerminal()}
}

// render redraws the block with the given lines, replacing whatever was drawn by
// the previous call.
func (b *liveBlock) render(lines []string) {
	if !b.enabled {
		return
	}
	if b.prevLines > 0 {
		fmt.Printf("\033[%dA", b.prevLines)
	}
	for _, line := range lines {
		fmt.Printf("\r\033[2K%s\n", line)
	}
	// Clear any leftover lines from a previously larger block.
	for i := len(lines); i < b.prevLines; i++ {
		fmt.Print("\r\033[2K\n")
	}
	if len(lines) < b.prevLines {
		fmt.Printf("\033[%dA", b.prevLines-len(lines))
	}
	b.prevLines = len(lines)
}

// clear erases the block so output can be printed above it. The block can be
// re-rendered afterwards.
func (b *liveBlock) clear() {
	if !b.enabled || b.prevLines == 0 {
		return
	}
	fmt.Printf("\033[%dA", b.prevLines)
	for i := 0; i < b.prevLines; i++ {
		fmt.Print("\r\033[2K\n")
	}
	fmt.Printf("\033[%dA", b.prevLines)
	b.prevLines = 0
}

// hostStatus tracks one host's execution timing for live display.
type hostStatus struct {
	start    time.Time
	done     bool
	duration time.Duration
}

// HostProgress is a snapshot of one host's execution state, passed to live table
// render functions.
type HostProgress struct {
	Hostname string
	Done     bool
	Elapsed  time.Duration // running: time so far; done: total duration
	Result   Result        // valid when Done
}

// launchTimed runs command on every host in parallel, recording per-host timing.
// It returns the results slice (filled in by index as hosts finish), the shared
// status slice, and a channel that emits each host's index as it completes.
func launchTimed(hosts []string, command string) ([]Result, []*hostStatus, <-chan int, *sync.Mutex) {
	results := make([]Result, len(hosts))
	statuses := make([]*hostStatus, len(hosts))
	completions := make(chan int, len(hosts))
	var mu sync.Mutex

	for i, hostname := range hosts {
		statuses[i] = &hostStatus{start: time.Now()}
		go func(i int, host string) {
			isLocal := host == "localhost"
			res := execute(host, command, isLocal)
			mu.Lock()
			results[i] = res
			statuses[i].done = true
			statuses[i].duration = res.Duration
			mu.Unlock()
			completions <- i
		}(i, hostname)
	}

	return results, statuses, completions, &mu
}

// ExecuteStreaming runs command on hosts in parallel and prints each host's full
// output as an atomic block as soon as that host finishes (completion order).
// While hosts run, a pinned spinner+timer block is shown for the hosts still
// running (TTY only).
func ExecuteStreaming(hosts []string, command string) ([]Result, error) {
	if err := verifyHostsAndCommand(hosts, command); err != nil {
		return nil, nil
	}

	results, statuses, completions, mu := launchTimed(hosts, command)
	block := newLiveBlock()

	// pendingLines builds the pinned block: every host still running.
	pendingLines := func() []string {
		mu.Lock()
		defer mu.Unlock()
		var lines []string
		for i := range hosts {
			if statuses[i].done {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s %s  %s",
				SpinnerFrame(),
				colors.Hostname.Sprint(hosts[i]),
				colors.Secondary.Sprint(FormatDuration(time.Since(statuses[i].start)))))
		}
		return lines
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	completed := 0
	block.render(pendingLines())
	for completed < len(hosts) {
		select {
		case idx := <-completions:
			completed++
			block.clear()
			printStreamResult(results[idx])
			block.render(pendingLines())
		case <-ticker.C:
			block.render(pendingLines())
		}
	}
	// All hosts done; everything has been printed and the block is empty.
	block.clear()

	return results, nil
}

// printStreamResult prints a single host's full output block.
func printStreamResult(result Result) {
	printResultHeader(result.Hostname, result.Command)

	if result.Stdout != "" {
		printOutputWithPrefix(result.Hostname, result.Stdout)
	}
	if result.Stderr != "" {
		printOutputWithPrefix(result.Hostname, result.Stderr)
	}

	dur := colors.Secondary.Sprint(FormatDuration(result.Duration))
	if result.Err != nil {
		colors.Error.Printf("%v ", result.Err)
		fmt.Printf("%s\n", dur)
	} else {
		fmt.Printf("%s %s\n", colors.Success.Sprint("✓ done"), dur)
	}
	fmt.Println()
}

// ExecuteLiveTable runs command on hosts in parallel and renders a live table via
// the provided render function, redrawing in place as hosts complete (TTY). On a
// non-TTY it renders once after all hosts finish.
func ExecuteLiveTable(hosts []string, command string, render func([]HostProgress) []string) ([]Result, error) {
	if err := verifyHostsAndCommand(hosts, command); err != nil {
		return nil, nil
	}

	results, statuses, completions, mu := launchTimed(hosts, command)
	block := newLiveBlock()

	snapshot := func() []HostProgress {
		mu.Lock()
		defer mu.Unlock()
		progress := make([]HostProgress, len(hosts))
		for i := range hosts {
			p := HostProgress{Hostname: hosts[i], Done: statuses[i].done}
			if statuses[i].done {
				p.Elapsed = statuses[i].duration
				p.Result = results[i]
			} else {
				p.Elapsed = time.Since(statuses[i].start)
			}
			progress[i] = p
		}
		return progress
	}

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	completed := 0
	block.render(render(snapshot()))
	for completed < len(hosts) {
		select {
		case <-completions:
			completed++
			block.render(render(snapshot()))
		case <-ticker.C:
			block.render(render(snapshot()))
		}
	}

	// Final frame. On a non-TTY the block was a no-op, so print it once here.
	if !block.enabled {
		fmt.Println(strings.Join(render(snapshot()), "\n"))
	} else {
		block.render(render(snapshot()))
	}

	return results, nil
}
