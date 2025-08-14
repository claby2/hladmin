package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull [hostname1] [hostname2] [hostname3] ...",
	Short: "Run git pull on specified hosts",
	Long:  "Execute git pull in $HOME/nix-config on each host",
	RunE:  runPull,
}

type pullResult struct {
	hostname string
	stdout   string
	stderr   string
	err      error
}

func executePull(hostname string, isLocal bool) pullResult {
	result := pullResult{hostname: hostname}

	var pullCmd *exec.Cmd
	if isLocal {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			result.err = fmt.Errorf("HOME environment variable not set")
			return result
		}
		nixConfigPath := filepath.Join(homeDir, "nix-config")
		pullCmd = exec.Command("git", "pull")
		pullCmd.Dir = nixConfigPath

	} else {
		pullCmd = exec.Command("ssh", hostname, "cd $HOME/nix-config && git pull")
	}

	var stdout, stderr bytes.Buffer
	pullCmd.Stdout = &stdout
	pullCmd.Stderr = &stderr

	if err := pullCmd.Run(); err != nil {
		result.err = fmt.Errorf("error pulling on %s: %v", hostname, err)
	}

	result.stdout = stdout.String()
	result.stderr = stderr.String()
	return result
}

func runPull(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified
	if len(args) == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}

	// Execute pulls concurrently but maintain ordered output
	results := make([]pullResult, len(args))
	var wg sync.WaitGroup

	for i, hostname := range args {
		wg.Add(1)
		go func(index int, host string) {
			defer wg.Done()
			isLocal := host == "localhost"
			results[index] = executePull(host, isLocal)
		}(i, hostname)
	}

	wg.Wait()

	// Print results in original order with proper sandwiching
	for _, result := range results {
		fmt.Printf("Pulling changes on %s...\n", result.hostname)

		if result.stdout != "" {
			fmt.Print(result.stdout)
		}
		if result.stderr != "" {
			fmt.Print(result.stderr)
		}

		if result.err != nil {
			fmt.Printf("%v\n", result.err)
		} else {
			fmt.Printf("Successfully pulled changes on %s\n", result.hostname)
		}
	}

	return nil
}
