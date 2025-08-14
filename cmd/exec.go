package cmd

import (
	"fmt"
	"strings"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var execParallel bool
var execInteractive bool

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
	execCmd.Flags().BoolVar(&execInteractive, "interactive", false, "Execute commands with direct stdin/stdout/stderr")
}

func runExec(cmd *cobra.Command, args []string) error {
	// Manually parse flags since DisableFlagParsing is true
	parallel := false
	interactive := false
	filteredArgs := make([]string, 0, len(args))

	for _, arg := range args {
		if arg == "--parallel" {
			parallel = true
		} else if arg == "--interactive" {
			interactive = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Validate mutually exclusive flags
	if parallel && interactive {
		return fmt.Errorf("--parallel and --interactive flags cannot be used together")
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
		return fmt.Errorf("command separator '--' not found. Usage: hladmin exec [--parallel|--interactive] <hosts...> -- <command> [args...]")
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

	// Determine execution mode
	var mode executor.ExecutionMode
	if interactive {
		mode = executor.Interactive
	} else if parallel {
		mode = executor.Parallel
	} else {
		mode = executor.Sequential
	}

	return executor.ExecuteOnHosts(hostnames, command, mode)
}
