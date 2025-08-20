package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/claby2/hladmin/internal/colors"
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
		fmt.Printf("%s %s\n\n", colors.Info.Sprint("Config:"), configPath)
	} else {
		fmt.Printf("%s %s (checked %s)\n\n", colors.Info.Sprint("Config:"), colors.Warning.Sprint("No configuration file found"), configPath)
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
		fmt.Println(colors.Warning.Sprint("No groups defined."))
	} else {
		fmt.Println(colors.Header.Sprint("Groups:"))
		for groupName, hosts := range cfg.Groups {
			fmt.Printf("  %s: %s\n", colors.Bold.Sprintf("@%s", groupName), strings.Join(hosts, ", "))
		}
		fmt.Println()

		if cfg.DefaultGroup != "" {
			fmt.Printf("%s %s\n", colors.Info.Sprint("Default Group:"), colors.Bold.Sprint(cfg.DefaultGroup))
		} else {
			fmt.Printf("%s %s\n", colors.Info.Sprint("Default Group:"), colors.Secondary.Sprint("none"))
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
				fmt.Printf("%s -> %s\n", colors.Bold.Sprint(arg), strings.Join(hosts, ", "))
			} else {
				fmt.Printf("%s -> %s\n", colors.Bold.Sprint(arg), colors.Error.Sprint("error: unknown group"))
			}
		} else {
			fmt.Printf("%s -> %s\n", colors.Hostname.Sprint(arg), arg)
		}
	}

	fmt.Println()
	fmt.Printf("%s %s\n", colors.Info.Sprint("Final host list:"), strings.Join(resolvedHosts, ", "))
	return nil
}
