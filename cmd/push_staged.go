package cmd

import (
	"fmt"
	"io/ioutil"
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
		return fmt.Errorf("failed to check staged changes: %v", err)
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
	tempDir, err := ioutil.TempDir("", "hladmin-patch-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	patchFile := filepath.Join(tempDir, "changes.patch")
	if err := ioutil.WriteFile(patchFile, diffOutput, 0644); err != nil {
		return fmt.Errorf("failed to write patch file: %v", err)
	}

	// Process each host
	for _, hostname := range args {
		fmt.Printf("Processing host: %s\n", hostname)

		// Check if remote repo is clean
		cleanCmd := exec.Command("ssh", hostname, "cd $HOME/nix-config && git status --porcelain")
		cleanOutput, err := cleanCmd.Output()
		if err != nil {
			fmt.Printf("  Error checking status: %v\n", err)
			continue
		}

		if len(strings.TrimSpace(string(cleanOutput))) != 0 {
			fmt.Printf("  Repository is dirty, skipping\n")
			if dryRun {
				fmt.Printf("  Would skip due to dirty repo\n")
			}
			continue
		}

		if dryRun {
			fmt.Printf("  Repository is clean, would apply patch\n")
			continue
		}

		// Copy and apply patch
		remotePatchFile := "/tmp/hladmin-patch.patch"

		// Copy patch to remote
		copyCmd := exec.Command("scp", patchFile, fmt.Sprintf("%s:%s", hostname, remotePatchFile))
		if err := copyCmd.Run(); err != nil {
			fmt.Printf("  Error copying patch: %v\n", err)
			continue
		}

		// Apply patch
		applyCmd := exec.Command("ssh", hostname, fmt.Sprintf("cd $HOME/nix-config && git apply %s && rm %s", remotePatchFile, remotePatchFile))
		if err := applyCmd.Run(); err != nil {
			fmt.Printf("  Error applying patch: %v\n", err)
			continue
		}

		fmt.Printf("  Patch applied successfully\n")
	}

	return nil
}
