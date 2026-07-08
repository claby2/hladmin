package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/claby2/hladmin/internal/ui"
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

// errorHostInfo returns a hostInfo with every data column marked "error".
func errorHostInfo(hostname string) hostInfo {
	return hostInfo{
		hostname:  hostname,
		hostclass: "error",
		version:   "error",
		repo:      "error",
		diskUsage: "error",
		memUsage:  "error",
	}
}

func parseCompoundOutput(hostname, output string) hostInfo {
	// Split by delimiter
	parts := strings.Split(strings.TrimSpace(output), "|||")

	// If we don't get exactly 5 parts, return error values
	if len(parts) != 5 {
		return errorHostInfo(hostname)
	}

	info := hostInfo{hostname: hostname}
	info.hostclass = strings.TrimSpace(parts[0])
	info.version = strings.TrimSpace(parts[1])
	info.repo = strings.TrimSpace(parts[2])
	info.diskUsage = strings.TrimSpace(parts[3])
	info.memUsage = strings.TrimSpace(parts[4])

	return info
}

// resultToHostInfo converts an execution result into the parsed status columns.
func resultToHostInfo(r executor.Result) hostInfo {
	if r.Err != nil {
		return errorHostInfo(r.Hostname)
	}
	return parseCompoundOutput(r.Hostname, r.Stdout)
}

// statusTableSpec defines the status table columns and per-host cell rendering.
func statusTableSpec() ui.TableSpec {
	return ui.TableSpec{
		Headers: []string{"HOSTNAME", "HOSTCLASS", "VERSION", "REPO", "DISK", "MEM"},
		CompletedRow: func(r executor.Result) []string {
			info := resultToHostInfo(r)

			versionStr := ui.Warning.Render(info.version)
			if info.version == info.repo {
				versionStr = ui.Success.Render(info.version)
			}

			return []string{
				info.hostname,
				info.hostclass,
				versionStr,
				ui.Success.Render(info.repo),
				info.diskUsage,
				info.memUsage,
			}
		},
		RunningRow: func(host, spinnerFrame string, elapsed time.Duration) []string {
			indicator := ui.Secondary.Render(fmt.Sprintf("%s collecting…  %s",
				spinnerFrame, ui.FormatDuration(elapsed)))
			return []string{host, indicator, "", "", "", ""}
		},
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := createCompoundStatusCommand()

	results, err := ui.RunLiveTable(hostnames, command, statusTableSpec())
	if err != nil {
		return err
	}

	return executor.ResultsError(results)
}
