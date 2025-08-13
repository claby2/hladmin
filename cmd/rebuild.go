package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	rebuildLocal bool
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild [hostname1] [hostname2] [hostname3] ...",
	Short: "Run rebuild script on specified hosts",
	Long:  "Execute the rebuild.sh script in $HOME/nix-config on each host",
	RunE:  runRebuild,
}

func init() {
	rebuildCmd.Flags().BoolVar(&rebuildLocal, "local", false, "Include localhost in rebuild operation")
}

func runRebuild(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified or --local is set
	if len(args) == 0 && !rebuildLocal {
		return fmt.Errorf("at least one hostname must be specified or --local flag must be set")
	}

	// TODO: Reduce repetition between local execution and remote execution

	// Handle local execution if --local flag is set
	if rebuildLocal {
		fmt.Printf("Rebuilding localhost...\n")

		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			fmt.Printf("Error rebuilding localhost: HOME environment variable not set\n")
		} else {
			nixConfigPath := filepath.Join(homeDir, "nix-config")
			rebuildCmd := exec.Command("./rebuild.sh")
			rebuildCmd.Dir = nixConfigPath
			rebuildCmd.Stdout = os.Stdout
			rebuildCmd.Stderr = os.Stderr
			rebuildCmd.Stdin = os.Stdin

			if err := rebuildCmd.Run(); err != nil {
				fmt.Printf("Error rebuilding localhost: %v\n", err)
			} else {
				fmt.Printf("Successfully rebuilt localhost\n")
			}
		}
	}

	// Handle remote hosts
	for _, hostname := range args {
		fmt.Printf("Rebuilding %s...\n", hostname)

		rebuildCmd := exec.Command("ssh", "-t", hostname, "cd $HOME/nix-config && ./rebuild.sh")
		rebuildCmd.Stdout = os.Stdout
		rebuildCmd.Stderr = os.Stderr
		rebuildCmd.Stdin = os.Stdin

		if err := rebuildCmd.Run(); err != nil {
			fmt.Printf("Error rebuilding %s: %v\n", hostname, err)
			continue
		}

		fmt.Printf("Successfully rebuilt %s\n", hostname)
	}

	return nil
}
