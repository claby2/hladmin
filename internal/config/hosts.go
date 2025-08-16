package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HostConfig represents the parsed host configuration
type HostConfig struct {
	Groups       map[string][]string
	DefaultGroup string
}

// getConfigDir returns the XDG-compliant config directory
func getConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "hladmin")
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "hladmin")
}

// getConfigPath returns the full path to the hosts config file
func getConfigPath() string {
	configDir := getConfigDir()
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "hosts")
}

// LoadConfig loads the host configuration from the config file
func LoadConfig() (*HostConfig, error) {
	configPath := getConfigPath()
	if configPath == "" {
		return &HostConfig{Groups: make(map[string][]string)}, nil
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &HostConfig{Groups: make(map[string][]string)}, nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %s: %v", configPath, err)
	}
	defer file.Close()

	config := &HostConfig{
		Groups: make(map[string][]string),
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid syntax on line %d: %s", lineNum, line)
		}

		switch fields[0] {
		case "group":
			if len(fields) < 3 {
				return nil, fmt.Errorf("group directive requires at least one host on line %d: %s", lineNum, line)
			}
			groupName := fields[1]
			hosts := fields[2:]
			config.Groups[groupName] = hosts

		case "default":
			if len(fields) != 2 {
				return nil, fmt.Errorf("default directive requires exactly one group name on line %d: %s", lineNum, line)
			}
			config.DefaultGroup = fields[1]

		default:
			return nil, fmt.Errorf("unknown directive '%s' on line %d: %s", fields[0], lineNum, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Validate that default group exists if specified
	if config.DefaultGroup != "" {
		if _, exists := config.Groups[config.DefaultGroup]; !exists {
			return nil, fmt.Errorf("default group '%s' is not defined", config.DefaultGroup)
		}
	}

	return config, nil
}

// ResolveHosts resolves a list of host arguments (which may include @group syntax)
// into a flat list of hostnames. If no arguments are provided and a default group
// is configured, it returns the hosts from the default group.
func (c *HostConfig) ResolveHosts(args []string) ([]string, error) {
	// If no arguments and we have a default group, use it
	if len(args) == 0 && c.DefaultGroup != "" {
		if hosts, exists := c.Groups[c.DefaultGroup]; exists {
			return hosts, nil
		}
	}

	// If no arguments and no default group, return empty (caller should handle)
	if len(args) == 0 {
		return nil, nil
	}

	var resolvedHosts []string
	seenHosts := make(map[string]bool) // Track duplicates

	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			// Group reference
			groupName := arg[1:]
			if groupName == "" {
				return nil, fmt.Errorf("empty group name: %s", arg)
			}

			hosts, exists := c.Groups[groupName]
			if !exists {
				return nil, fmt.Errorf("unknown group: %s", groupName)
			}

			// Add hosts from group, avoiding duplicates
			for _, host := range hosts {
				if !seenHosts[host] {
					resolvedHosts = append(resolvedHosts, host)
					seenHosts[host] = true
				}
			}
		} else {
			// Individual host
			if !seenHosts[arg] {
				resolvedHosts = append(resolvedHosts, arg)
				seenHosts[arg] = true
			}
		}
	}

	return resolvedHosts, nil
}
