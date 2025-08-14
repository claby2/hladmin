package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// ExecutionMode defines how commands are executed
type ExecutionMode int

const (
	// Sequential executes commands one host at a time with captured output
	Sequential ExecutionMode = iota
	// Parallel executes commands on multiple hosts concurrently with captured output
	Parallel
	// Interactive executes commands one host at a time with direct stdin/stdout/stderr
	Interactive
)

// Result represents the result of command execution on a single host
type Result struct {
	Hostname string
	Command  string
	Stdout   string
	Stderr   string
	Err      error
}

// ExecuteOnHosts executes a command on multiple hosts with specified execution mode
func ExecuteOnHosts(hosts []string, command string, mode ExecutionMode) error {
	if len(hosts) == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}

	switch mode {
	case Parallel:
		return executeParallel(hosts, command)
	case Interactive:
		return executeInteractive(hosts, command)
	default: // Sequential
		return executeSequential(hosts, command)
	}
}

func executeParallel(hosts []string, command string) error {
	results := make([]Result, len(hosts))
	var wg sync.WaitGroup

	for i, hostname := range hosts {
		wg.Add(1)
		go func(index int, host string) {
			defer wg.Done()
			isLocal := host == "localhost"
			results[index] = executeWithCapture(host, command, isLocal)
		}(i, hostname)
	}

	wg.Wait()

	// Display results in original order
	for _, result := range results {
		displayResult(result)
	}

	return nil
}

func executeSequential(hosts []string, command string) error {
	for _, hostname := range hosts {
		isLocal := hostname == "localhost"
		result := executeWithCapture(hostname, command, isLocal)
		displayResult(result)
	}

	return nil
}

func executeInteractive(hosts []string, command string) error {
	for _, hostname := range hosts {
		isLocal := hostname == "localhost"
		if err := executeWithInteraction(hostname, command, isLocal); err != nil {
			fmt.Printf("%v\n", err)
		}
	}

	return nil
}

func executeWithCapture(hostname, command string, isLocal bool) Result {
	result := Result{Hostname: hostname, Command: command}

	var cmd *exec.Cmd
	if isLocal {
		cmd = exec.Command("bash", "-c", command)
	} else {
		cmd = exec.Command("ssh", hostname, command)
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

func executeWithInteraction(hostname, command string, isLocal bool) error {
	fmt.Printf("Executing on %s: %s\n", hostname, command)

	var cmd *exec.Cmd
	if isLocal {
		// For interactive local commands, handle working directory properly
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return fmt.Errorf("HOME environment variable not set")
		}
		nixConfigPath := filepath.Join(homeDir, "nix-config")
		cmd = exec.Command("bash", "-c", fmt.Sprintf("cd %s && %s", nixConfigPath, command))
	} else {
		cmd = exec.Command("ssh", "-t", hostname, command)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing on %s: %v", hostname, err)
	}

	fmt.Printf("Successfully executed on %s\n", hostname)
	return nil
}

func displayResult(result Result) {
	fmt.Printf("Executing on %s: %s\n", result.Hostname, result.Command)

	if result.Stdout != "" {
		fmt.Print(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Print(result.Stderr)
	}

	if result.Err != nil {
		fmt.Printf("%v\n", result.Err)
	} else {
		fmt.Printf("Successfully executed on %s\n", result.Hostname)
	}
}

