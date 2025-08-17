package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/claby2/hladmin/internal/config"
	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	Use:           hostUsagePattern("resolve"),
	Short:         "Show host configuration and resolve groups",
	Long:          hostLongDescription("Show the current host configuration and resolve group references. Without arguments, displays the full configuration including all groups and the default group. With arguments, shows how the specified hosts and groups resolve to individual hostnames."),
	RunE:          runResolve,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func runResolve(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	configPath := config.GetConfigPath()
	if configPath == "" {
		configPath = "unknown"
	}
	configExists := false
	if _, statErr := os.Stat(configPath); statErr == nil {
		configExists = true
	}

	// Always show config location first
	if configExists {
		fmt.Printf("Config: %s\n\n", configPath)
	} else {
		fmt.Printf("Config: No configuration file found (checked %s)\n\n", configPath)
	}

	// If no arguments, show full configuration
	if len(args) == 0 {
		showFullConfiguration(cfg)
		return nil
	}

	// Resolve specific arguments
	if err := showHostResolution(cfg, args); err != nil {
		return err
	}

	return nil
}

func showFullConfiguration(cfg *config.HostConfig) {
	if len(cfg.Groups) == 0 {
		fmt.Println("No groups defined.")
	} else {
		fmt.Println("Groups:")
		for groupName, hosts := range cfg.Groups {
			fmt.Printf("  %s: %s\n", groupName, strings.Join(hosts, ", "))
		}
		fmt.Println()

		if cfg.DefaultGroup != "" {
			fmt.Printf("Default Group: %s\n", cfg.DefaultGroup)
		} else {
			fmt.Println("Default Group: none")
		}
	}
}

func showHostResolution(cfg *config.HostConfig, args []string) error {
	resolvedHosts, err := cfg.ResolveHosts(args)
	if err != nil {
		return err
	}

	// Show individual resolutions
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			groupName := arg[1:]
			if hosts, exists := cfg.Groups[groupName]; exists {
				fmt.Printf("%s -> %s\n", arg, strings.Join(hosts, ", "))
			} else {
				fmt.Printf("%s -> error: unknown group\n", arg)
			}
		} else {
			fmt.Printf("%s -> %s\n", arg, arg)
		}
	}

	fmt.Println()
	fmt.Printf("Final host list: %s\n", strings.Join(resolvedHosts, ", "))
	return nil
}
