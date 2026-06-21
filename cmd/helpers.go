package cmd

import (
	"fmt"

	"github.com/claby2/hladmin/internal/config"
	"github.com/claby2/hladmin/internal/executor"
)

// hostUsagePattern returns a standardized usage pattern for commands that accept hosts
func hostUsagePattern(command string) string {
	return fmt.Sprintf("%s [hostname1] [hostname2] [@group] ...", command)
}

// hostLongDescription returns a standardized long description for commands that accept hosts
func hostLongDescription(baseDescription string) string {
	return fmt.Sprintf("%s Use @group to reference host groups from config.", baseDescription)
}

// resolveHosts loads the host configuration and resolves the provided arguments
// (which may include @group syntax) into a flat list of hostnames.
// Returns an error if configuration loading fails, host resolution fails,
// or no hosts are specified/resolved.
func resolveHosts(args []string) ([]string, error) {
	// Load host configuration
	hostConfig, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load host configuration: %v", err)
	}

	// Resolve host arguments (including @group syntax and defaults)
	hostnames, err := hostConfig.ResolveHosts(args)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hosts: %v", err)
	}

	// Validate that at least one host is specified
	if len(hostnames) == 0 {
		return nil, fmt.Errorf("at least one hostname must be specified")
	}

	return hostnames, nil
}

// streamCommand runs command on hosts with streaming output and returns the
// first host error, if any.
func streamCommand(hostnames []string, command string) error {
	results, err := executor.ExecuteStreaming(hostnames, command)
	if err != nil {
		return err
	}
	return executor.ResultsError(results)
}
