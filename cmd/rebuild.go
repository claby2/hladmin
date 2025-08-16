package cmd

import (
	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var rebuildCmd = &cobra.Command{
	Use:           hostUsagePattern("rebuild"),
	Short:         "Run rebuild script on specified hosts",
	Long:          hostLongDescription("Execute the rebuild.sh script in $HOME/nix-config on each host."),
	RunE:          runRebuild,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func runRebuild(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := "cd $HOME/nix-config && ./rebuild.sh"

	if err := executor.ExecuteOnHostsInteractive(hostnames, command); err != nil {
		return err
	}
	return nil
}
