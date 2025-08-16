package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

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
	diskUsage string
	memUsage  string
	gitStatus string
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
echo -n "$(df -h / | tail -1 | awk '{print $5}')|||" && \
echo -n "$(%s)|||" && \
echo -n "$(cd $HOME/nix-config 2>/dev/null && if [ "$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')" = "0" ]; then echo 'clean'; else echo 'dirty'; fi || echo 'error')" && \
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
		info.diskUsage = "error"
		info.memUsage = "error"
		info.gitStatus = "error"
		return info
	}

	info.hostclass = strings.TrimSpace(parts[0])
	info.version = strings.TrimSpace(parts[1])
	info.diskUsage = strings.TrimSpace(parts[2])
	info.memUsage = strings.TrimSpace(parts[3])
	info.gitStatus = strings.TrimSpace(parts[4])

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
				diskUsage: "error",
				memUsage:  "error",
				gitStatus: "error",
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

	// Print columnar output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "HOSTNAME\tHOSTCLASS\tVERSION\tDISK\tMEM\tGIT\n")

	for _, host := range hosts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			host.hostname,
			host.hostclass,
			host.version,
			host.diskUsage,
			host.memUsage,
			host.gitStatus,
		)
	}

	w.Flush()
	return nil
}
