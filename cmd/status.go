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
	diskUsage string
	memUsage  string
	gitStatus string
}

type commandSpec struct {
	name        string
	command     string
	parseOutput func(string) string
}

var statusCommands = []commandSpec{
	{
		name:    "hostclass",
		command: "echo $HOSTCLASS",
		parseOutput: func(output string) string {
			return strings.TrimSpace(output)
		},
	},
	{
		name:    "version",
		command: "nixos-version --configuration-revision 2>/dev/null || darwin-version --configuration-revision 2>/dev/null || echo 'unknown'",
		parseOutput: func(output string) string {
			return strings.TrimSpace(output)
		},
	},
	{
		name:    "diskUsage",
		command: "df -h / | tail -1 | awk '{print $5}'",
		parseOutput: func(output string) string {
			return strings.TrimSpace(output)
		},
	},
	{
		name:    "memUsage",
		command: "if command -v free >/dev/null 2>&1; then free | grep '^Mem:' | awk '{printf \"%.0f%%\", $3/$2*100}'; else vm_stat | awk '/^Pages/ {free+=$3; inactive+=$3; wired+=$3; active+=$3} /^Pages free/ {free=$3} /^Pages inactive/ {inactive=$3} /^Pages wired/ {wired=$3} /^Pages active/ {active=$3} END {total=free+inactive+wired+active; used=wired+active; printf \"%.0f%%\", used/total*100}'; fi",
		parseOutput: func(output string) string {
			return strings.TrimSpace(output)
		},
	},
}

func executeStatusCommand(cmdSpec commandSpec, isLocal bool, hostname string) string {
	var cmd *exec.Cmd
	if isLocal {
		cmd = exec.Command("bash", "-c", cmdSpec.command)
	} else {
		cmd = exec.Command("ssh", hostname, cmdSpec.command)
	}

	output, err := cmd.Output()
	if err != nil {
		return "error"
	}
	return cmdSpec.parseOutput(string(output))
}

func getGitStatus(isLocal bool, hostname string, nixConfigPath string) string {
	if isLocal && nixConfigPath != "" {
		gitCmd := exec.Command("git", "status", "--porcelain")
		gitCmd.Dir = nixConfigPath
		gitOutput, err := gitCmd.Output()
		if err != nil {
			return "error"
		}
		gitStatus := strings.TrimSpace(string(gitOutput))
		if gitStatus == "" {
			return "clean"
		}
		return "dirty"
	} else if !isLocal {
		gitCmd := exec.Command("ssh", hostname, "cd $HOME/nix-config && git status --porcelain")
		gitOutput, err := gitCmd.Output()
		if err != nil {
			return "error"
		}
		gitStatus := strings.TrimSpace(string(gitOutput))
		if gitStatus == "" {
			return "clean"
		}
		return "dirty"
	}
	return "error"
}

func collectHostInfo(hostname string, isLocal bool) hostInfo {
	info := hostInfo{hostname: hostname}

	var nixConfigPath string
	if isLocal {
		homeDir := os.Getenv("HOME")
		if homeDir != "" {
			nixConfigPath = filepath.Join(homeDir, "nix-config")
		}
	}

	// Execute all status commands using the modular approach
	for _, cmdSpec := range statusCommands {
		result := executeStatusCommand(cmdSpec, isLocal, hostname)
		switch cmdSpec.name {
		case "hostclass":
			info.hostclass = result
		case "version":
			info.version = result
		case "diskUsage":
			info.diskUsage = result
		case "memUsage":
			info.memUsage = result
		}
	}

	// Handle git status separately as it has special logic
	info.gitStatus = getGitStatus(isLocal, hostname, nixConfigPath)

	return info
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Validate that at least one host is specified or --local is set
	if len(args) == 0 && !statusLocal {
		return fmt.Errorf("at least one hostname must be specified or --local flag must be set")
	}

	var hosts []hostInfo

	// Handle local execution if --local flag is set
	if statusLocal {
		hosts = append(hosts, collectHostInfo("localhost", true))
	}

	// Collect information for remote hosts
	for _, hostname := range args {
		hosts = append(hosts, collectHostInfo(hostname, false))
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
