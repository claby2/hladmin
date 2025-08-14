package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/claby2/hladmin/internal/executor"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [hostname1] [hostname2] [hostname3] ...",
	Short: "Show status information for specified hosts",
	Long:  "Display HOSTCLASS, configuration revision, and other useful system information",
	RunE:  runStatus,
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
	return "vm_stat | awk '/^Pages/ {free+=$3; inactive+=$3; wired+=$3; active+=$3} /^Pages free/ {free=$3} /^Pages inactive/ {inactive=$3} /^Pages wired/ {wired=$3} /^Pages active/ {active=$3} END {total=free+inactive+wired+active; used=wired+active; printf \"%.0f%%\", used/total*100}'"
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

	var hostInfos []hostInfo

	// Execute compound command on all hosts in parallel
	// We can't use ExecuteOnHosts directly because it prints results,
	// but we need to capture and parse them. So we'll use the internal
	// executeWithCapture approach.

	resultsChan := make(chan executor.Result, len(hosts))
	for _, hostname := range hosts {
		go func(host string) {
			isLocal := host == "localhost"
			result := executeWithCapture(host, command, isLocal)
			resultsChan <- result
		}(hostname)
	}

	// Collect results
	for i := 0; i < len(hosts); i++ {
		result := <-resultsChan
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

// executeWithCapture is copied from executor package for direct access
func executeWithCapture(hostname, command string, isLocal bool) executor.Result {
	result := executor.Result{Hostname: hostname, Command: command}

	var cmd *exec.Cmd
	if isLocal {
		cmd = exec.Command("bash", "-c", command)
	} else {
		cmd = exec.Command("ssh", hostname, command)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		result.Err = fmt.Errorf("error executing on %s: %v", hostname, err)
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	return result
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified
	if len(args) == 0 {
		return fmt.Errorf("at least one hostname must be specified")
	}

	// Collect information for all hosts using optimized compound command
	hosts, err := collectHostInfo(args)
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
