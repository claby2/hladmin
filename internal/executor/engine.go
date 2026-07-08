package executor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result represents the result of command execution on a single host
type Result struct {
	Hostname string
	Command  string
	Stdout   string
	Stderr   string
	Err      error
	Duration time.Duration
}

// VerifyHostsAndCommand validates that at least one host and a non-empty command
// were provided.
func VerifyHostsAndCommand(hosts []string, command string) error {
	if len(hosts) == 0 {
		return errors.New("at least one hostname must be specified")
	}
	if strings.TrimSpace(command) == "" {
		return errors.New("command cannot be empty")
	}
	return nil
}

// ResultsError returns the first non-nil error found in results, if any.
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

	start := time.Now()
	if err := cmd.Run(); err != nil {
		result.Err = fmt.Errorf("error executing on %s: %v", hostname, err)
	}
	result.Duration = time.Since(start)

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	return result
}

// RunOnHost executes command on a single host, capturing its output. It uses a
// local bash shell for localhost and SSH for remote hosts.
func RunOnHost(hostname, command string) Result {
	return execute(hostname, command, hostname == "localhost")
}

// RunInteractive executes command on a single host with stdin/stdout/stderr wired
// to the current process. It performs no output formatting; presentation is the
// caller's responsibility.
func RunInteractive(hostname, command string) error {
	cmd := exec.Command("ssh", "-t", hostname, command)
	if hostname == "localhost" {
		cmd = exec.Command("bash", "-c", command)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing on %s: %v", hostname, err)
	}
	return nil
}
