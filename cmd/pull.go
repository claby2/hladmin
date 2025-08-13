package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	pullLocal bool
)

var pullCmd = &cobra.Command{
	Use:   "pull [hostname1] [hostname2] [hostname3] ...",
	Short: "Run git pull on specified hosts",
	Long:  "Execute git pull in $HOME/nix-config on each host",
	RunE:  runPull,
}

func init() {
	pullCmd.Flags().BoolVar(&pullLocal, "local", false, "Include localhost in pull operation")
}

func runPull(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified or --local is set
	if len(args) == 0 && !pullLocal {
		return fmt.Errorf("at least one hostname must be specified or --local flag must be set")
	}

	// TODO: Reduce repetition between local execution and remote execution

	// Handle local execution if --local flag is set
	if pullLocal {
		fmt.Printf("Pulling changes on localhost...\n")

		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			fmt.Printf("Error pulling on localhost: HOME environment variable not set\n")
		} else {
			nixConfigPath := filepath.Join(homeDir, "nix-config")
			pullCmd := exec.Command("git", "pull")
			pullCmd.Dir = nixConfigPath
			pullCmd.Stdout = os.Stdout
			pullCmd.Stderr = os.Stderr

			if err := pullCmd.Run(); err != nil {
				fmt.Printf("Error pulling on localhost: %v\n", err)
			} else {
				fmt.Printf("Successfully pulled changes on localhost\n")
			}
		}
	}

	// Handle remote hosts
	for _, hostname := range args {
		fmt.Printf("Pulling changes on %s...\n", hostname)

		pullCmd := exec.Command("ssh", hostname, "cd $HOME/nix-config && git pull")
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr

		if err := pullCmd.Run(); err != nil {
			fmt.Printf("Error pulling on %s: %v\n", hostname, err)
			continue
		}

		fmt.Printf("Successfully pulled changes on %s\n", hostname)
	}

	return nil
}
