# hladmin

A homelab administration tool for managing NixOS servers and macOS machines running nix-darwin. Built with Rust and the clap CLI framework, hladmin executes commands remotely via SSH and provides a unified interface for common homelab operations.

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
cargo build --release
# binary at target/release/hladmin
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

Name hosts by their real hostnames. hladmin detects which machine it is running
on and executes there through a local shell instead of over SSH, so the same
hosts file works unchanged on every machine in the fleet. `localhost` still
works as an alias for the current machine.

### Commands

#### status

Display system information for specified hosts in a tabular format.

```bash
# Check status of multiple hosts
hladmin status server1 server2 desktop1

# Check the machine you are on (detected automatically)
hladmin status desktop1
```

**Example output:**

```
HOSTNAME         HOSTCLASS  VERSION                                   REPO                                      DISK  MEM
desktop1 (self)  mac        6f88686d63493d507e6c1e4e47f1e22cab8dac13  6f88686d63493d507e6c1e4e47f1e22cab8dac13  3%    47%
altaria          server     6f88686d63493d507e6c1e4e47f1e22cab8dac13  6f88686d63493d507e6c1e4e47f1e22cab8dac13  17%   46%
onix             server     6f88686d63493d507e6c1e4e47f1e22cab8dac13  6f88686d63493d507e6c1e4e47f1e22cab8dac13  43%   25%
```

#### exec

Execute arbitrary commands on specified hosts with flexible execution modes.

```bash
# Execute command in parallel (default)
hladmin exec server1 server2 -- uptime

# Execute interactively (with stdin/stdout/stderr)
hladmin exec --interactive server1 server2 -- htop

# Mix local and remote execution (desktop1 is the machine you are on)
hladmin exec desktop1 server1 -- systemctl status nginx
```

**Flags:**

- `--interactive`: Execute with direct terminal interaction sequentially

#### rebuild

Execute the rebuild script (`$HOME/nix-config/rebuild.sh`) on specified hosts. By default this runs in two phases: all hosts build in parallel (`./rebuild.sh --build-only`, no sudo required) behind a live status table showing each host's build state and elapsed time, then each host activates sequentially with direct terminal interaction so sudo can prompt for its password. Because the build is already cached, the activation pass takes only seconds per host. A failed build skips that host's activation; a failed activation doesn't stop the remaining hosts.

After the build table, failed hosts print their full build output; successful hosts print nothing — the activation pass replays the already-cached build, so its diff output (what changed) appears there. Pass `-v` to see full build output for every host.

The parallel build phase requires a `rebuild.sh` that understands `--build-only` — run `hladmin pull` first if hosts have an older copy.

```bash
# Rebuild single host
hladmin rebuild server1

# Rebuild multiple hosts (parallel build, sequential activation)
hladmin rebuild server1 server2 desktop1

# Rebuild fully sequentially with direct terminal interaction
hladmin rebuild -i server1 server2

# Rebuild the machine you are on
hladmin rebuild desktop1
```

**Flags:**

- `--remote`: Pass `--remote` to rebuild.sh (build on the remote build host)
- `-i, --interactive`: Rebuild hosts fully sequentially with direct stdin/stdout/stderr
- `-v, --verbose`: Print full build output for every host, not just failures

#### pull

Execute `git pull` in the `$HOME/nix-config` directory on specified hosts. Runs in parallel by default for efficiency.

```bash
# Pull latest changes on multiple hosts
hladmin pull server1 server2 desktop1

# Pull on the machine you are on
hladmin pull desktop1
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
- Skips the machine you are on (your staged changes originate there)
- Creates temporary patch files for secure transfer
- Supports dry-run mode for testing

**Flags:**

- `--dry-run`: Show what would be done without making changes

#### reset

Hard-reset `$HOME/nix-config` on remote hosts to a pristine `origin/main`: fetch, check out `main`, `git reset --hard origin/main`, and remove untracked files (`git clean -fd`). Useful for discarding patches previously applied with `push-staged` so the next `push-staged` finds a clean repository again.

```bash
# Reset remote hosts to origin/main
hladmin reset server1 server2

# Iterate on staged changes without confirmation prompts
hladmin push-staged @servers
# ...make and stage more changes locally...
hladmin reset --yes @servers
hladmin push-staged @servers
```

**Features:**

- Only prompts for confirmation when uncommitted changes, untracked files, or unpushed commits would actually be destroyed (shows exactly what, per host)
- Skips the machine you are on (a reset there would discard your local work)
- Gitignored files are untouched (no `git clean -x`)

**Flags:**

- `-y`, `--yes`: Skip the confirmation prompt

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

**One file, every machine.** Name hosts by their real hostnames and this file can
be byte-identical everywhere — check it into your nix-config and deploy the same
copy to every host. There is no `localhost` entry and no per-machine variant:
hladmin compares each target against the machine it is running on and executes
locally when they match, so `hladmin status @all` produces the same table from
any host in the fleet. The machine you are on needs no SSH access to itself.

`hladmin resolve` reports which machine hladmin thinks it is and warns when that
name matches nothing in your config — worth checking first if a host is
unexpectedly going over SSH:

```
Config: /home/you/.config/hladmin/hosts
Self: desktop1 (hostname)

Groups:
  @servers: server1, server2, server3
  @all: server1, server2, server3, desktop1 (self), laptop1
```

**Identity detection.** The name comes from the system hostname, lowercased and
shortened to its first label, so a Mac reporting `laptop1.local` still matches a
`laptop1` entry. Override it with `HLADMIN_SELF=<name>`; set `HLADMIN_SELF=` to
the empty string to disable detection and send every host over SSH. A
user-qualified target such as `root@server1` is always remote, even on server1 —
running it locally would run as you instead.

**Migrating an old config.** If a group still lists both `localhost` and the
machine's real hostname, the two collapse into a single entry rather than
executing twice, and `hladmin resolve` says so. Dropping the `localhost` entry is
what makes the file shareable.

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

1. **SSH Access**: Password-less SSH key authentication configured (not needed for the machine you run hladmin from — it executes locally)
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
