package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var dryRun bool

var pushStagedCmd = &cobra.Command{
	Use:   "push-staged <hostname1> [hostname2] [hostname3] ...",
	Short: "Push staged git changes to specified hosts",
	Long:  "Check for staged changes in $HOME/nix-config and apply them to clean hosts",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runPushStaged,
}

func init() {
	pushStagedCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
}

func runPushStaged(cmd *cobra.Command, args []string) error {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return fmt.Errorf("HOME environment variable not set")
	}
	nixConfigPath := filepath.Join(homeDir, "nix-config")

	// Check for staged changes
	diffCmd := exec.Command("git", "diff", "--cached")
	diffCmd.Dir = nixConfigPath
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check staged changes in %s: %v", nixConfigPath, err)
	}

	if len(diffOutput) == 0 {
		fmt.Println("No staged changes found")
		return nil
	}

	if dryRun {
		fmt.Println("Staged changes:")
		fmt.Println(string(diffOutput))
		fmt.Println()
	}

	// Create temporary patch file
	patchFile, err := os.CreateTemp("", "hladmin-patch-*.patch")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(patchFile.Name())
	defer patchFile.Close()

	if _, err := patchFile.Write(diffOutput); err != nil {
		return fmt.Errorf("failed to write patch file: %v", err)
	}
	patchFile.Close()

	// Process each host
	for _, hostname := range args {
		fmt.Printf("Processing host: %s\n", hostname)

		// Check if remote repo is clean
		cleanCmd := exec.Command("ssh", hostname, "cd $HOME/nix-config && git status --porcelain")
		cleanOutput, err := cleanCmd.Output()
		if err != nil {
			fmt.Printf("  Error checking git status on %s: %v\n", hostname, err)
			continue
		}

		if strings.TrimSpace(string(cleanOutput)) != "" {
			fmt.Printf("  Repository has uncommitted changes, skipping\n")
			if dryRun {
				fmt.Printf("  Would skip due to uncommitted changes\n")
			}
			continue
		}

		if dryRun {
			fmt.Printf("  Repository is clean, would apply patch\n")
			continue
		}

		// Create secure temporary file on remote with unique name
		// Using hostname + PID prevents conflicts when multiple hladmin instances
		// target the same host or when running concurrent operations
		remotePatchFile := fmt.Sprintf("/tmp/hladmin-patch-%s-%d.patch", hostname, os.Getpid())

		// Copy patch to remote
		copyCmd := exec.Command("scp", patchFile.Name(), fmt.Sprintf("%s:%s", hostname, remotePatchFile))
		if err := copyCmd.Run(); err != nil {
			fmt.Printf("  Error copying patch: %v\n", err)
			continue
		}

		// Apply patch with proper cleanup on both success and failure
		applyCmd := exec.Command("ssh", hostname, fmt.Sprintf("cd $HOME/nix-config && git apply %s; rm -f %s", remotePatchFile, remotePatchFile))
		if err := applyCmd.Run(); err != nil {
			fmt.Printf("  Error applying patch: %v\n", err)
			// Ensure cleanup even on failure
			cleanupCmd := exec.Command("ssh", hostname, fmt.Sprintf("rm -f %s", remotePatchFile))
			cleanupCmd.Run()
			continue
		}

		fmt.Printf("  Patch applied successfully\n")
	}

	return nil
}
