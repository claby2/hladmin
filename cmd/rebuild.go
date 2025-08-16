package cmd

import (
	"fmt"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var rebuildCmd = &cobra.Command{
	Use:   hostUsagePattern("rebuild"),
	Short: "Run rebuild script on specified hosts",
	Long:  hostLongDescription("Execute the rebuild.sh script in $HOME/nix-config on each host."),
	RunE:  runRebuild,
}

func runRebuild(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := "cd $HOME/nix-config && ./rebuild.sh"

	if err := executor.ExecuteOnHostsInteractive(hostnames, command); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	return nil
}
