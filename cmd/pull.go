package cmd

import (
	"fmt"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull [hostname1] [hostname2] [hostname3] ...",
	Short: "Run git pull on specified hosts",
	Long:  "Execute git pull in $HOME/nix-config on each host",
	RunE:  runPull,
}

func runPull(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}
	command := "cd $HOME/nix-config && git pull"

	var results []executor.Result
	results, err := executor.ExecuteOnHostsParallel(args, command)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	executor.DisplayResults(results)
	return nil
}
