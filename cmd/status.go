package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	statusLocal bool
)

var statusCmd = &cobra.Command{
	Use:   "status [hostname1] [hostname2] [hostname3] ...",
	Short: "Show status information for specified hosts",
	Long:  "Display HOSTCLASS, configuration revision, and other useful system information",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusLocal, "local", false, "Include localhost in status check")
}

type hostInfo struct {
	hostname  string
	hostclass string
	version   string
	uptime    string
	diskUsage string
	memUsage  string
	gitStatus string
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified or --local is set
	if len(args) == 0 && !statusLocal {
		return fmt.Errorf("at least one hostname must be specified or --local flag must be set")
	}

	var hosts []hostInfo

	// Handle local execution if --local flag is set
	if statusLocal {
		info := hostInfo{hostname: "localhost"}

		// Get HOSTCLASS
		hostclassCmd := exec.Command("bash", "-c", "echo $HOSTCLASS")
		hostclassOutput, err := hostclassCmd.Output()
		if err != nil {
			info.hostclass = "error"
		} else {
			info.hostclass = strings.TrimSpace(string(hostclassOutput))
		}

		// Try nixos-version first, then darwin-version
		nixosCmd := exec.Command("bash", "-c", "nixos-version --configuration-revision 2>/dev/null || darwin-version --configuration-revision 2>/dev/null || echo 'unknown'")
		versionOutput, err := nixosCmd.Output()
		if err != nil {
			info.version = "error"
		} else {
			info.version = strings.TrimSpace(string(versionOutput))
		}

		// Uptime (simplified)
		uptimeCmd := exec.Command("bash", "-c", "uptime | sed 's/.*up //; s/, [0-9]* user.*//' | sed 's/,.*load.*//'")
		uptimeOutput, err := uptimeCmd.Output()
		if err != nil {
			info.uptime = "error"
		} else {
			info.uptime = strings.TrimSpace(string(uptimeOutput))
		}

		// Disk usage of root (just percentage)
		diskCmd := exec.Command("bash", "-c", "df -h / | tail -1 | awk '{print $5}'")
		diskOutput, err := diskCmd.Output()
		if err != nil {
			info.diskUsage = "error"
		} else {
			info.diskUsage = strings.TrimSpace(string(diskOutput))
		}

		// Memory usage (simplified)
		memCmd := exec.Command("bash", "-c", "free | grep '^Mem:' | awk '{printf \"%.0f%%\", $3/$2*100}'")
		memOutput, err := memCmd.Output()
		if err != nil {
			info.memUsage = "error"
		} else {
			info.memUsage = strings.TrimSpace(string(memOutput))
		}

		// Git status of nix-config
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			info.gitStatus = "error"
		} else {
			nixConfigPath := filepath.Join(homeDir, "nix-config")
			gitCmd := exec.Command("git", "status", "--porcelain")
			gitCmd.Dir = nixConfigPath
			gitOutput, err := gitCmd.Output()
			if err != nil {
				info.gitStatus = "error"
			} else {
				gitStatus := strings.TrimSpace(string(gitOutput))
				if gitStatus == "" {
					info.gitStatus = "clean"
				} else {
					info.gitStatus = "dirty"
				}
			}
		}

		hosts = append(hosts, info)
	}

	// Collect information for remote hosts
	for _, hostname := range args {
		info := hostInfo{hostname: hostname}

		// Get HOSTCLASS
		hostclassCmd := exec.Command("ssh", hostname, "echo $HOSTCLASS")
		hostclassOutput, err := hostclassCmd.Output()
		if err != nil {
			info.hostclass = "error"
		} else {
			info.hostclass = strings.TrimSpace(string(hostclassOutput))
		}

		// Try nixos-version first, then darwin-version
		nixosCmd := exec.Command("ssh", hostname, "nixos-version --configuration-revision 2>/dev/null || darwin-version --configuration-revision 2>/dev/null || echo 'unknown'")
		versionOutput, err := nixosCmd.Output()
		if err != nil {
			info.version = "error"
		} else {
			info.version = strings.TrimSpace(string(versionOutput))
		}

		// Uptime (simplified)
		uptimeCmd := exec.Command("ssh", hostname, "uptime | sed 's/.*up //; s/, [0-9]* user.*//' | sed 's/,.*load.*//'")
		uptimeOutput, err := uptimeCmd.Output()
		if err != nil {
			info.uptime = "error"
		} else {
			info.uptime = strings.TrimSpace(string(uptimeOutput))
		}

		// Disk usage of root (just percentage)
		diskCmd := exec.Command("ssh", hostname, "df -h / | tail -1 | awk '{print $5}'")
		diskOutput, err := diskCmd.Output()
		if err != nil {
			info.diskUsage = "error"
		} else {
			info.diskUsage = strings.TrimSpace(string(diskOutput))
		}

		// Memory usage (simplified)
		memCmd := exec.Command("ssh", hostname, "free | grep '^Mem:' | awk '{printf \"%.0f%%\", $3/$2*100}'")
		memOutput, err := memCmd.Output()
		if err != nil {
			info.memUsage = "error"
		} else {
			info.memUsage = strings.TrimSpace(string(memOutput))
		}

		// Git status of nix-config
		gitCmd := exec.Command("ssh", hostname, "cd $HOME/nix-config && git status --porcelain")
		gitOutput, err := gitCmd.Output()
		if err != nil {
			info.gitStatus = "error"
		} else {
			gitStatus := strings.TrimSpace(string(gitOutput))
			if gitStatus == "" {
				info.gitStatus = "clean"
			} else {
				info.gitStatus = "dirty"
			}
		}

		hosts = append(hosts, info)
	}

	// Print columnar output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "HOSTNAME\tHOSTCLASS\tVERSION\tUPTIME\tDISK\tMEM\tGIT\n")

	for _, host := range hosts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			host.hostname,
			host.hostclass,
			host.version,
			host.uptime,
			host.diskUsage,
			host.memUsage,
			host.gitStatus,
		)
	}

	w.Flush()
	return nil
}
