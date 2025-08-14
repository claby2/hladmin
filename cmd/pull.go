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

func executePull(hostname string, isLocal bool) error {
	fmt.Printf("Pulling changes on %s...\n", hostname)

	var pullCmd *exec.Cmd
	if isLocal {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			return fmt.Errorf("HOME environment variable not set")
		}
		nixConfigPath := filepath.Join(homeDir, "nix-config")
		pullCmd = exec.Command("git", "pull")
		pullCmd.Dir = nixConfigPath

	} else {
		pullCmd = exec.Command("ssh", hostname, "cd $HOME/nix-config && git pull")
	}

	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("error pulling on %s: %v", hostname, err)
	}

	fmt.Printf("Successfully pulled changes on %s\n", hostname)
	return nil
}

func runPull(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified or --local is set
	if len(args) == 0 && !pullLocal {
		return fmt.Errorf("at least one hostname must be specified or --local flag must be set")
	}

	// Handle local execution if --local flag is set
	if pullLocal {
		if err := executePull("localhost", true); err != nil {
			fmt.Printf("%v\n", err)
		}
	}

	// Handle remote hosts
	for _, hostname := range args {
		if err := executePull(hostname, false); err != nil {
			fmt.Printf("%v\n", err)
			continue
		}
	}

	return nil
}
