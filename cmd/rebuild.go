package cmd

import (
	"fmt"

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
	if len(args) == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}
	command := "cd $HOME/nix-config && ./rebuild.sh"

	if err := executor.ExecuteOnHostsInteractive(args, command); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	return nil
}
