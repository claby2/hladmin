# hladmin

A homelab administration tool for managing NixOS servers and macOS machines running nix-darwin. Built with Go and the Cobra CLI framework, hladmin executes commands remotely via SSH and provides a unified interface for common homelab operations.

> ⚠️ WARNING: This tool makes several assumptions about the underlying hosts to accommodate my setup. Most of these assumptions should be documented here, but some details could be missing.

## Features

- **Multi-host Management**: Execute commands on multiple hosts simultaneously
- **Cross-platform Support**: Works with NixOS and macOS (nix-darwin) systems
- **Flexible Execution Modes**: Sequential, parallel, and interactive command execution
- **System Status Monitoring**: View system information across all hosts in a tabular format
- **Git Operations**: Synchronize configuration changes across your infrastructure
- **Configuration Management**: Deploy staged changes and rebuild systems

## Installation

### Using Nix

```bash
nix build github:claby2/hladmin
# Or for development
nix develop
```

### From Source

```bash
git clone https://github.com/claby2/hladmin.git
cd hladmin
go build -o hladmin
```

## Prerequisites

- SSH access to all managed hosts
- Each host must have a `$HOME/nix-config` directory with a `rebuild.sh` script (see [claby2/nix-config](https://github.com/claby2/nix-config) for an example)
- `$HOSTCLASS` environment variable defined on each host

## Usage

### Basic Syntax

```bash
hladmin <command> [flags] [hostname1] [hostname2] ...
```

For local execution, use `localhost` as the hostname.

### Commands

#### status

Display system information for specified hosts in a tabular format.

```bash
# Check status of multiple hosts
hladmin status server1 server2 desktop1

# Check local system status
hladmin status localhost
```

**Example output:**

```
HOSTNAME   HOSTCLASS  VERSION                                   REPO                                      DISK  MEM
localhost  mac        6f88686d63493d507e6c1e4e47f1e22cab8dac13  6f88686d63493d507e6c1e4e47f1e22cab8dac13  3%    47%
altaria    server     6f88686d63493d507e6c1e4e47f1e22cab8dac13  6f88686d63493d507e6c1e4e47f1e22cab8dac13  17%   46%
onix       server     6f88686d63493d507e6c1e4e47f1e22cab8dac13  6f88686d63493d507e6c1e4e47f1e22cab8dac13  43%   25%
```

#### exec

Execute arbitrary commands on specified hosts with flexible execution modes.

```bash
# Execute command in parallel (default)
hladmin exec server1 server2 -- uptime

# Execute interactively (with stdin/stdout/stderr)
hladmin exec --interactive server1 server2 -- htop

# Mix local and remote execution
hladmin exec localhost server1 -- systemctl status nginx
```

**Flags:**

- `--interactive`: Execute with direct terminal interaction sequentially

#### rebuild

Execute the rebuild script (`$HOME/nix-config/rebuild.sh`) on specified hosts. This command provides real-time feedback and runs interactively during system rebuilds.

```bash
# Rebuild single host
hladmin rebuild server1

# Rebuild multiple hosts (sequential)
hladmin rebuild server1 server2 desktop1

# Rebuild local system
hladmin rebuild localhost
```

#### pull

Execute `git pull` in the `$HOME/nix-config` directory on specified hosts. Runs in parallel by default for efficiency.

```bash
# Pull latest changes on multiple hosts
hladmin pull server1 server2 desktop1

# Pull on local system
hladmin pull localhost
```

#### push-staged

Push staged git changes from your local `$HOME/nix-config` to clean remote repositories. Only applies changes to hosts with clean git status.

```bash
# Push staged changes to remote hosts
hladmin push-staged server1 server2

# Dry run to see what would be pushed
hladmin push-staged --dry-run server1 server2
```

**Features:**

- Only pushes changes to hosts with clean git repositories
- Creates temporary patch files for secure transfer
- Supports dry-run mode for testing

**Flags:**

- `--dry-run`: Show what would be done without making changes

#### resolve

Show host configuration and resolve group references. Displays the current host configuration including group definitions and default group settings.

```bash
# Show full configuration
hladmin resolve

# Resolve specific hosts and groups
hladmin resolve @servers desktop1 @all

# Check what hosts a group contains
hladmin resolve @servers
```

## Examples

### Common Workflows

**Deploy configuration changes across infrastructure:**

```bash
# 1. Stage your changes locally
git add .

# 2. Push to clean hosts
hladmin push-staged --dry-run @servers  # verify changes
hladmin push-staged @servers            # apply changes

# 3. Rebuild affected systems
hladmin rebuild @servers
```

**Check system health across homelab:**

```bash
# Get comprehensive status overview
hladmin status @all

# Check specific metrics on all hosts
hladmin exec @all -- "uptime && free -h"
```

**Update all systems:**

```bash
# Pull latest configuration
hladmin pull @all

# Rebuild all systems
hladmin rebuild @all
```

**Interactive troubleshooting:**

```bash
# Check logs interactively
hladmin exec --interactive server1 -- journalctl -f

# Run system maintenance
hladmin exec --interactive server1 -- nix-collect-garbage -d
```

**Parallel monitoring:**

```bash
# Check disk space across all hosts (parallel by default)
hladmin exec @all -- "df -h / | tail -1"

# Monitor network connectivity
hladmin exec @servers -- ping -c 3 8.8.8.8
```

## Configuration

### Host Groups

hladmin supports organizing hosts into groups for easier management. Create a configuration file to define host groups and set defaults.

**Configuration File Location:**

- `$XDG_CONFIG_HOME/hladmin/hosts` or
- `~/.config/hladmin/hosts`

**Configuration Syntax:**

```bash
# Define host groups
group servers server1 server2 server3
group desktops desktop1 laptop1
group all server1 server2 server3 desktop1 laptop1

# Set default group (used when no hosts specified)
default servers
```

**Using Host Groups:**

```bash
# Use @group syntax to reference groups
hladmin status @servers          # Check status of all servers
hladmin exec @desktops -- uptime # Execute command on desktop hosts
hladmin status                   # Uses default group (servers in example above)
hladmin rebuild @all             # Rebuild all hosts

# Groups can be mixed with individual hosts
hladmin status @servers desktop1 laptop1
```

### Host Requirements

Each managed host must have:

1. **SSH Access**: Password-less SSH key authentication configured
2. **Nix Configuration**: `$HOME/nix-config` directory with:
   - Git repository with your NixOS/nix-darwin configuration
   - Executable `rebuild.sh` script
3. **Environment Variables**: `$HOSTCLASS` variable indicating the host's role

### Example Host Setup

```bash
# On each host, ensure these exist:
ls $HOME/nix-config/rebuild.sh  # executable rebuild script
echo $HOSTCLASS                 # should output host role (e.g., "server", "desktop")
```
