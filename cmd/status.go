package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/claby2/hladmin/internal/colors"
	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// padRight pads s to width based on its visual length (excluding ANSI codes).
func padRight(s string, width int) string {
	visLen := len(ansiRegexp.ReplaceAllString(s, ""))
	if visLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visLen)
}

var statusCmd = &cobra.Command{
	Use:           hostUsagePattern("status"),
	Short:         "Show status information for specified hosts",
	Long:          hostLongDescription("Display HOSTCLASS, configuration revision, and other useful system information."),
	RunE:          runStatus,
	SilenceUsage:  true,
	SilenceErrors: true,
}

type hostInfo struct {
	hostname  string
	hostclass string
	version   string
	repo      string
	diskUsage string
	memUsage  string
}

func getLinuxMemoryCommand() string {
	return "free | grep '^Mem:' | awk '{printf \"%.0f%%\", $3/$2*100}'"
}

func getMacOSMemoryCommand() string {
	return `vm_stat | awk '
		/^Pages free/ { free = $3 }
		/^Pages inactive/ { inactive = $3 }
		/^Pages wired/ { wired = $3 }
		/^Pages active/ { active = $3 }
		END {
			total = free + inactive + wired + active
			if (total > 0) {
				used = wired + active
				printf "%.0f%%", used/total*100
			} else {
				print "0%"
			}
		}'`
}

func getMemoryCommand() string {
	return fmt.Sprintf("if command -v free >/dev/null 2>&1; then %s; else %s; fi",
		getLinuxMemoryCommand(), getMacOSMemoryCommand())
}

func createCompoundStatusCommand() string {
	memCmd := getMemoryCommand()
	return fmt.Sprintf(`
echo -n "$HOSTCLASS|||" && \
echo -n "$(nixos-version --configuration-revision 2>/dev/null || darwin-version --configuration-revision 2>/dev/null || echo 'unknown')|||" && \
echo -n "$(nix flake metadata $HOME/nix-config 2>/dev/null | grep "Revision:" | awk '{print $2}')|||" && \
echo -n "$(df -h / | tail -1 | awk '{print $5}')|||" && \
echo -n "$(%s)" && \
echo
`, memCmd)
}

func parseCompoundOutput(hostname, output string) hostInfo {
	info := hostInfo{hostname: hostname}

	// Split by delimiter
	parts := strings.Split(strings.TrimSpace(output), "|||")

	// If we don't get exactly 5 parts, return error values
	if len(parts) != 5 {
		info.hostclass = "error"
		info.version = "error"
		info.repo = "error"
		info.diskUsage = "error"
		info.memUsage = "error"
		return info
	}

	info.hostclass = strings.TrimSpace(parts[0])
	info.version = strings.TrimSpace(parts[1])
	info.repo = strings.TrimSpace(parts[2])
	info.diskUsage = strings.TrimSpace(parts[3])
	info.memUsage = strings.TrimSpace(parts[4])

	return info
}

func collectHostInfo(hosts []string) ([]hostInfo, error) {
	command := createCompoundStatusCommand()

	// Execute compound command on all hosts in parallel using executor with progress
	results, err := executor.ExecuteOnHostsParallelWithProgress(hosts, command, "Collecting host status")
	if err != nil {
		return nil, err
	}
	if err = executor.ResultsError(results); err != nil {
		return nil, err
	}

	var hostInfos []hostInfo
	for _, result := range results {
		if result.Err != nil {
			// Create error hostInfo
			hostInfos = append(hostInfos, hostInfo{
				hostname:  result.Hostname,
				hostclass: "error",
				version:   "error",
				repo:      "error",
				diskUsage: "error",
				memUsage:  "error",
			})
		} else {
			// Parse the compound output
			hostInfos = append(hostInfos, parseCompoundOutput(result.Hostname, result.Stdout))
		}
	}

	return hostInfos, nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	// Collect information for all hosts using optimized compound command
	hosts, err := collectHostInfo(hostnames)
	if err != nil {
		return err
	}

	// Compute column widths
	nameW, classW, verW, repoW, diskW := len("HOSTNAME"), len("HOSTCLASS"), len("VERSION"), len("REPO"), len("DISK")
	for _, h := range hosts {
		nameW = max(nameW, len(h.hostname))
		classW = max(classW, len(h.hostclass))
		verW = max(verW, len(h.version))
		repoW = max(repoW, len(h.repo))
		diskW = max(diskW, len(h.diskUsage))
	}

	// Print header
	fmt.Printf("%s  %s  %s  %s  %s  %s\n",
		padRight("HOSTNAME", nameW),
		padRight("HOSTCLASS", classW),
		padRight("VERSION", verW),
		padRight("REPO", repoW),
		padRight("DISK", diskW),
		"MEM")

	// Print data rows with colored VERSION and REPO
	for _, host := range hosts {
		var versionStr string
		if host.version == host.repo {
			versionStr = colors.Success.Sprint(host.version)
		} else {
			versionStr = colors.Warning.Sprint(host.version)
		}
		repoStr := colors.Success.Sprint(host.repo)

		fmt.Printf("%s  %s  %s  %s  %s  %s\n",
			padRight(host.hostname, nameW),
			padRight(host.hostclass, classW),
			padRight(versionStr, verW),
			padRight(repoStr, repoW),
			padRight(host.diskUsage, diskW),
			host.memUsage)
	}

	return nil
}
