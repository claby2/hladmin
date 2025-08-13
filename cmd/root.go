package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hladmin",
	Short: "Homelab administration tool",
	Long:  "A tool for managing homelab servers running NixOS and macOS with nix-darwin",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(pushStagedCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(rebuildCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(execCmd)
}
