package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var execParallel bool

var execCmd = &cobra.Command{
	Use:                   "exec [hostname1] [hostname2] [hostname3] ... -- <command> [args...]",
	Short:                 "Execute command on specified hosts",
	Long:                  "Run the specified command with arguments on each host",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	RunE:                  runExec,
}

func init() {
	execCmd.Flags().BoolVar(&execParallel, "parallel", false, "Execute commands on hosts concurrently")
}

type execResult struct {
	hostname string
	command  string
	stdout   string
	stderr   string
	err      error
}

func displayResult(result execResult) {
	fmt.Printf("Executing on %s: %s\n", result.hostname, result.command)

	if result.stdout != "" {
		fmt.Print(result.stdout)
	}
	if result.stderr != "" {
		fmt.Print(result.stderr)
	}

	if result.err != nil {
		fmt.Printf("%v\n", result.err)
	} else {
		fmt.Printf("Successfully executed on %s\n", result.hostname)
	}
}

func executeCommandWithCapture(hostname, command string, isLocal bool) execResult {
	result := execResult{hostname: hostname, command: command}

	execCmd := exec.Command("ssh", hostname, command)
	if isLocal {
		execCmd = exec.Command("bash", "-c", command)
	}

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		result.err = fmt.Errorf("error executing on %s: %v", hostname, err)
	}

	result.stdout = stdout.String()
	result.stderr = stderr.String()
	return result
}

func executeParallel(hostnames []string, command string) {
	results := make([]execResult, len(hostnames))
	var wg sync.WaitGroup

	for i, hostname := range hostnames {
		wg.Add(1)
		go func(index int, host string) {
			defer wg.Done()
			isLocal := host == "localhost"
			results[index] = executeCommandWithCapture(host, command, isLocal)
		}(i, hostname)
	}

	wg.Wait()

	for _, result := range results {
		displayResult(result)
	}
}

func executeSequential(hostnames []string, command string) {
	for _, hostname := range hostnames {
		isLocal := hostname == "localhost"
		result := executeCommandWithCapture(hostname, command, isLocal)
		displayResult(result)
	}
}

func runExec(cmd *cobra.Command, args []string) error {
	// Manually parse --parallel flag since DisableFlagParsing is true
	parallel := false
	filteredArgs := make([]string, 0, len(args))

	for _, arg := range args {
		if arg == "--parallel" {
			parallel = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Find the -- separator
	separatorIndex := -1
	for i, arg := range filteredArgs {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}

	if separatorIndex == -1 {
		return fmt.Errorf("command separator '--' not found. Usage: hladmin exec [--parallel] <hosts...> -- <command> [args...]")
	}

	if separatorIndex == len(filteredArgs)-1 {
		return fmt.Errorf("no command specified after '--'")
	}

	// Validate that at least one host is specified
	if separatorIndex == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}

	hostnames := filteredArgs[:separatorIndex]
	command := strings.Join(filteredArgs[separatorIndex+1:], " ")

	if parallel {
		executeParallel(hostnames, command)
	} else {
		executeSequential(hostnames, command)
	}

	return nil
}
