package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild [hostname1] [hostname2] [hostname3] ...",
	Short: "Run rebuild script on specified hosts",
	Long:  "Execute the rebuild.sh script in $HOME/nix-config on each host",
	RunE:  runRebuild,
}

func executeRebuild(hostname string, isLocal bool) error {
	fmt.Printf("Rebuilding %s...\n", hostname)

	var rebuildCmd *exec.Cmd
	if isLocal {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return fmt.Errorf("HOME environment variable not set")
		}
		nixConfigPath := filepath.Join(homeDir, "nix-config")
		rebuildCmd = exec.Command("./rebuild.sh")
		rebuildCmd.Dir = nixConfigPath
	} else {
		rebuildCmd = exec.Command("ssh", "-t", hostname, "cd $HOME/nix-config && ./rebuild.sh")
	}

	rebuildCmd.Stdout = os.Stdout
	rebuildCmd.Stderr = os.Stderr
	rebuildCmd.Stdin = os.Stdin

	if err := rebuildCmd.Run(); err != nil {
		return fmt.Errorf("error rebuilding %s: %v", hostname, err)
	}

	fmt.Printf("Successfully rebuilt %s\n", hostname)
	return nil
}

func runRebuild(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified
	if len(args) == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}

	// Handle all hosts sequentially
	// Note: Rebuild operations must be sequential because they may require
	// interactive input (e.g., sudo password prompts) which cannot be
	// handled properly with concurrent execution
	for _, hostname := range args {
		isLocal := hostname == "localhost"
		if err := executeRebuild(hostname, isLocal); err != nil {
			fmt.Printf("%v\n", err)
		}
	}

	return nil
}
