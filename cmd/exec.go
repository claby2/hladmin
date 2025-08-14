package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	execLocal bool
)

var execCmd = &cobra.Command{
	Use:                   "exec [--local] [hostname1] [hostname2] [hostname3] ... -- <command> [args...]",
	Short:                 "Execute command on specified hosts",
	Long:                  "Run the specified command with arguments on each host",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	RunE:                  runExec,
}

func executeCommand(hostname, command string, isLocal bool) error {
	fmt.Printf("Executing on %s: %s\n", hostname, command)

	var execCmd *exec.Cmd
	if isLocal {
		execCmd = exec.Command("bash", "-c", command)
	} else {
		execCmd = exec.Command("ssh", hostname, command)
	}

	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("error executing on %s: %v", hostname, err)
	}

	fmt.Printf("Successfully executed on %s\n", hostname)
	return nil
}

func runExec(cmd *cobra.Command, args []string) error {
	// Manually parse --local flag since we disabled flag parsing
	localFlag := false
	var filteredArgs []string

	for _, arg := range args {
		if arg == "--local" {
			localFlag = true
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
		return fmt.Errorf("command separator '--' not found. Usage: hladmin exec [--local] <hosts...> -- <command> [args...]")
	}

	if separatorIndex == len(filteredArgs)-1 {
		return fmt.Errorf("no command specified after '--'")
	}

	// Validate that at least one host is specified or --local is set
	if separatorIndex == 0 && !localFlag {
		return fmt.Errorf("at least one hostname must be specified or --local flag must be set")
	}

	var hostnames []string
	if separatorIndex > 0 {
		hostnames = filteredArgs[:separatorIndex]
	}
	command := strings.Join(filteredArgs[separatorIndex+1:], " ")

	// Handle local execution if --local flag is set
	if localFlag {
		if err := executeCommand("localhost", command, true); err != nil {
			fmt.Printf("%v\n", err)
		}
	}

	// Handle remote hosts
	for _, hostname := range hostnames {
		if err := executeCommand(hostname, command, false); err != nil {
			fmt.Printf("%v\n", err)
			continue
		}
	}

	return nil
}
