package executor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

// Result represents the result of command execution on a single host
type Result struct {
	Hostname string
	Command  string
	Stdout   string
	Stderr   string
	Err      error
}

func verifyHostsAndCommand(hosts []string, command string) error {
	if len(hosts) == 0 {
		return errors.New("at least one hostname must be specified")
	}
	if strings.TrimSpace(command) == "" {
		return errors.New("command cannot be empty")
	}
	return nil
}

func ExecuteOnHostsInteractive(hosts []string, command string) error {
	if err := verifyHostsAndCommand(hosts, command); err != nil {
		return nil
	}

	for _, hostname := range hosts {
		isLocal := hostname == "localhost"
		if err := executeInteractive(hostname, command, isLocal); err != nil {
			return err
		}
	}
	return nil
}

func ExecuteOnHostsParallel(hosts []string, command string) ([]Result, error) {
	if err := verifyHostsAndCommand(hosts, command); err != nil {
		return nil, nil
	}

	results := make([]Result, len(hosts))
	var wg sync.WaitGroup

	for i, hostname := range hosts {
		wg.Add(1)
		go func(i int, host string) {
			defer wg.Done()
			isLocal := host == "localhost"
			results[i] = execute(host, command, isLocal)
		}(i, hostname)
	}
	wg.Wait()

	return results, nil
}

// ExecuteOnHostsParallelWithProgress executes commands on hosts with optional progress indicator
func ExecuteOnHostsParallelWithProgress(hosts []string, command string, progressMessage string) ([]Result, error) {
	if err := verifyHostsAndCommand(hosts, command); err != nil {
		return nil, nil
	}

	// Skip progress indicator for single host or when disabled
	if len(hosts) == 1 {
		return ExecuteOnHostsParallel(hosts, command)
	}

	if progressMessage == "" {
		progressMessage = "Executing on hosts"
	}

	results := make([]Result, len(hosts))
	var wg sync.WaitGroup
	var completedCount int64
	var mu sync.Mutex

	// Create and start spinner
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s... (0/%d hosts)", progressMessage, len(hosts))
	s.Start()

	for i, hostname := range hosts {
		wg.Add(1)
		go func(i int, host string) {
			defer wg.Done()
			isLocal := host == "localhost"
			results[i] = execute(host, command, isLocal)

			// Update progress
			mu.Lock()
			completedCount++
			s.Suffix = fmt.Sprintf(" %s... (%d/%d hosts)", progressMessage, completedCount, len(hosts))
			mu.Unlock()
		}(i, hostname)
	}
	wg.Wait()

	// Stop spinner and show completion
	s.Stop()
	fmt.Printf("✓ %s completed (%d/%d hosts)\n", progressMessage, len(hosts), len(hosts))

	return results, nil
}

func DisplayResults(results []Result) {
	for _, result := range results {
		fmt.Printf("=== Executing on %s: %s\n", result.Hostname, result.Command)

		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Print(result.Stderr)
		}

		if result.Err != nil {
			fmt.Printf("%v\n", result.Err)
		} else {
			fmt.Printf("=== ✓ Successfully executed on %s\n", result.Hostname)
		}
	}
}

func ResultsError(results []Result) error {
	for _, result := range results {
		if result.Err != nil {
			return result.Err
		}
	}
	return nil
}

func execute(hostname, command string, isLocal bool) Result {
	result := Result{Hostname: hostname, Command: command}

	cmd := exec.Command("ssh", hostname, command)
	if isLocal {
		cmd = exec.Command("bash", "-c", command)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		result.Err = fmt.Errorf("error executing on %s: %v", hostname, err)
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	return result
}

func executeInteractive(hostname, command string, isLocal bool) error {
	fmt.Printf("=== Executing on %s: %s\n", hostname, command)

	cmd := exec.Command("ssh", "-t", hostname, command)
	if isLocal {
		cmd = exec.Command("bash", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing on %s: %v", hostname, err)
	}

	fmt.Printf("=== ✓ Successfully executed on %s\n", hostname)
	return nil
}
