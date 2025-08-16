package cmd

import (
	"fmt"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   hostUsagePattern("pull"),
	Short: "Run git pull on specified hosts",
	Long:  hostLongDescription("Execute git pull in $HOME/nix-config on each host."),
	RunE:  runPull,
}

func runPull(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := "cd $HOME/nix-config && git pull"

	var results []executor.Result
	results, err = executor.ExecuteOnHostsParallel(hostnames, command)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	executor.DisplayResults(results)
	return nil
}
