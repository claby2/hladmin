package cmd

import (
	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:           hostUsagePattern("pull"),
	Short:         "Run git pull on specified hosts",
	Long:          hostLongDescription("Execute git pull in $HOME/nix-config on each host."),
	RunE:          runPull,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func runPull(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := "cd $HOME/nix-config && git pull"

	var results []executor.Result
	results, err = executor.ExecuteOnHostsParallelWithProgress(hostnames, command, "Running git pull")
	if err != nil {
		return err
	}
	executor.DisplayResults(results)
	if err = executor.ResultsError(results); err != nil {
		return err
	}
	return nil
}
