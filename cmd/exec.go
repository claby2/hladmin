package cmd

import (
	"fmt"
	"strings"

	"github.com/claby2/hladmin/internal/ui"
	"github.com/spf13/cobra"
)

var execInteractive bool

var execCmd = &cobra.Command{
	Use:                   hostUsagePattern("exec") + " -- <command> [args...]",
	Short:                 "Execute command on specified hosts",
	Long:                  hostLongDescription("Run the specified command with arguments on each host."),
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	RunE:                  runExec,
	SilenceUsage:          true,
	SilenceErrors:         true,
}

func init() {
	execCmd.Flags().BoolVarP(&execInteractive, "interactive", "i", false, "Execute commands with direct stdin/stdout/stderr")
}

func runExec(cmd *cobra.Command, args []string) error {
	// Manually parse flags since DisableFlagParsing is true
	isInteractive := false
	filteredArgs := make([]string, 0, len(args))

	for _, arg := range args {
		if arg == "--interactive" || arg == "-i" {
			isInteractive = true
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
		return fmt.Errorf("command separator '--' not found. Usage: hladmin exec [-i|--interactive] <hosts...> -- <command> [args...]")
	}

	if separatorIndex == len(filteredArgs)-1 {
		return fmt.Errorf("no command specified after '--'")
	}

	hostArgs := filteredArgs[:separatorIndex]
	command := strings.Join(filteredArgs[separatorIndex+1:], " ")

	// Resolve hosts using helper
	hostnames, err := resolveHosts(hostArgs)
	if err != nil {
		return err
	}

	// Determine execution mode
	if isInteractive {
		return ui.RunInteractive(hostnames, command)
	}
	return streamCommand(hostnames, command)
}
