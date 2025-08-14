package cmd

import (
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
	command := "cd $HOME/nix-config && git pull"
	return executor.ExecuteOnHosts(args, command, executor.Parallel)
}
