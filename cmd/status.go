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

// renderStatusTable builds the status table lines from the current per-host
// progress. Completed hosts show their parsed data; hosts still running show a
// spinner and elapsed timer in place of their data columns.
func renderStatusTable(progress []executor.HostProgress) []string {
	infos := make([]hostInfo, len(progress))
	for i, p := range progress {
		switch {
		case !p.Done:
			infos[i] = hostInfo{hostname: p.Hostname}
		case p.Result.Err != nil:
			infos[i] = hostInfo{
				hostname:  p.Hostname,
				hostclass: "error",
				version:   "error",
				repo:      "error",
				diskUsage: "error",
				memUsage:  "error",
			}
		default:
			infos[i] = parseCompoundOutput(p.Hostname, p.Result.Stdout)
		}
	}

	// Compute column widths from the header and completed rows.
	nameW, classW, verW, repoW, diskW := len("HOSTNAME"), len("HOSTCLASS"), len("VERSION"), len("REPO"), len("DISK")
	for i, info := range infos {
		nameW = max(nameW, len(info.hostname))
		if progress[i].Done {
			classW = max(classW, len(info.hostclass))
			verW = max(verW, len(info.version))
			repoW = max(repoW, len(info.repo))
			diskW = max(diskW, len(info.diskUsage))
		}
	}

	lines := []string{
		fmt.Sprintf("%s  %s  %s  %s  %s  %s",
			padRight("HOSTNAME", nameW),
			padRight("HOSTCLASS", classW),
			padRight("VERSION", verW),
			padRight("REPO", repoW),
			padRight("DISK", diskW),
			"MEM"),
	}

	for i, info := range infos {
		if !progress[i].Done {
			indicator := colors.Secondary.Sprintf("%s collecting…  %s",
				executor.SpinnerFrame(), executor.FormatDuration(progress[i].Elapsed))
			lines = append(lines, fmt.Sprintf("%s  %s", padRight(info.hostname, nameW), indicator))
			continue
		}

		var versionStr string
		if info.version == info.repo {
			versionStr = colors.Success.Sprint(info.version)
		} else {
			versionStr = colors.Warning.Sprint(info.version)
		}
		repoStr := colors.Success.Sprint(info.repo)

		lines = append(lines, fmt.Sprintf("%s  %s  %s  %s  %s  %s",
			padRight(info.hostname, nameW),
			padRight(info.hostclass, classW),
			padRight(versionStr, verW),
			padRight(repoStr, repoW),
			padRight(info.diskUsage, diskW),
			info.memUsage))
	}

	return lines
}

func runStatus(cmd *cobra.Command, args []string) error {
	hostnames, err := resolveHosts(args)
	if err != nil {
		return err
	}

	command := createCompoundStatusCommand()

	results, err := executor.ExecuteLiveTable(hostnames, command, renderStatusTable)
	if err != nil {
		return err
	}

	return executor.ResultsError(results)
}
