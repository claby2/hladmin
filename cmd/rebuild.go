package cmd

import (
	"github.com/claby2/hladmin/internal/ui"
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

func init() {
	rebuildCmd.Flags().Bool("remote", false, "Pass --remote flag to rebuild.sh")
}

func runRebuild(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := "cd $HOME/nix-config && ./rebuild.sh"
	if remote, _ := cmd.Flags().GetBool("remote"); remote {
		command += " --remote"
	}

	return ui.RunInteractive(hostnames, command)
}
