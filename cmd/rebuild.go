package cmd

import (
	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild [hostname1] [hostname2] [hostname3] ...",
	Short: "Run rebuild script on specified hosts",
	Long:  "Execute the rebuild.sh script in $HOME/nix-config on each host",
	RunE:  runRebuild,
}

func runRebuild(cmd *cobra.Command, args []string) error {
	command := "cd $HOME/nix-config && ./rebuild.sh"
	return executor.ExecuteOnHosts(args, command, executor.Interactive)
}
