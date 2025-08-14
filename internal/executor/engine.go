package executor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Result represents the result of command execution on a single host
type Result struct {
	Hostname string
	Command  string
	Stdout   string
	Stderr   string
	Err      error
}

func ExecuteOnHostsInteractive(hosts []string, command string) error {
	if len(hosts) == 0 {
		return errors.New("at least one hostname must be specified")
	}
	if strings.TrimSpace(command) == "" {
		return errors.New("command cannot be empty")
	}

	for _, hostname := range hosts {
		isLocal := hostname == "localhost"
		executeInteractive(hostname, command, isLocal)
	}
	return nil
}

func ExecuteOnHostsParallel(hosts []string, command string) ([]Result, error) {
	if len(hosts) == 0 {
		return nil, errors.New("at least one hostname must be specified")
	}
	if strings.TrimSpace(command) == "" {
		return nil, errors.New("command cannot be empty")
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

func DisplayResults(results []Result) {
	for _, result := range results {
		displayResult(result)
	}
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
	fmt.Printf("Executing on %s: %s\n", hostname, command)

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
