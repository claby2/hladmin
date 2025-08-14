# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`hladmin` is a homelab administration tool built in Go using the Cobra CLI framework. It manages NixOS servers and macOS machines running nix-darwin by executing commands remotely via SSH or locally when the `--local` flag is used.

## Development Commands

### Building

```bash
go build -o hladmin
```

### Nix Build

```bash
nix build
```

### Running

```bash
./hladmin <command> [flags] [hosts...]
```

### Dependencies

```bash
go mod tidy
```

### Development Shell

```bash
nix develop
```

## Architecture

### CLI Structure

- **Entry Point**: `main.go` - Simple entry point that calls `cmd.Execute()`
- **Root Command**: `cmd/root.go` - Defines the root `hladmin` command and registers all subcommands
- **Subcommands**: Each command is in its own file under `cmd/`:
  - `push_staged.go` - Pushes staged git changes from local to remote hosts
  - `status.go` - Shows system status in columnar format using tabwriter
  - `pull.go` - Executes git pull on nix-config repositories
  - `rebuild.go` - Runs rebuild.sh script with terminal support (ssh -t)
  - `exec.go` - Executes arbitrary commands on hosts

### Command Pattern

All commands follow a consistent pattern:

1. **Flag Variables**: Declared at package level (e.g., `statusLocal bool`)
2. **Command Definition**: Cobra command struct with Use, Short, Long, RunE
3. **Flag Registration**: `init()` function registers flags
4. **Execution Logic**: `run*()` function handles both local and remote execution
5. **Validation**: Commands require either hostnames OR `--local` flag

### Local vs Remote Execution

Commands support both local and remote execution:

- **Remote**: Uses SSH to execute commands on specified hostnames
- **Local**: When `--local` flag is set, executes commands directly without SSH
- **Mixed**: Can combine `--local` with remote hostnames in single command

### Key Implementation Details

#### SSH Execution

- Standard commands use `ssh hostname "command"`
- Interactive commands (rebuild) use `ssh -t hostname "command"` with stdin/stdout/stderr connected
- Remote paths always use `$HOME/nix-config` (shell variable expansion)

#### Local Execution

- Uses `bash -c "command"` or direct Go `exec.Command()`
- Local paths use `filepath.Join(os.Getenv("HOME"), "nix-config")`
- Git operations set working directory with `cmd.Dir`

#### Special Command Handling

- **exec**: Requires `--` separator between hosts and command args, manually parses `--local` flag due to `DisableFlagParsing: true` (Cobra consumes `--` during flag parsing)
- **push-staged**: Creates temporary patch files using `os.CreateTemp()`, only applies to clean git repos
- **status**: Uses modular `commandSpec` architecture with `executeStatusCommand()` for cross-platform system metrics collection

### Error Handling

Commands continue processing remaining hosts when individual hosts fail, printing errors but not stopping execution.

### Path Management

- **Local paths**: Always use `os.Getenv("HOME")` + `filepath.Join()` for cross-platform compatibility
- **Remote paths**: Use shell variable `$HOME` for SSH command execution
- **Temporary files**: Use `os.CreateTemp()` with automatic cleanup via `defer`

### Cross-Platform Compatibility

The status command implements cross-platform system monitoring:

- **Memory Usage**: Detects and uses `free` (Linux) or `vm_stat` (macOS) automatically
- **Version Detection**: Supports both `nixos-version` and `darwin-version` commands

### Modular Architecture

Recent improvements have introduced modular design patterns:

- **commandSpec**: Defines system commands with parsing functions for status collection
- **executeStatusCommand()**: Unified execution engine for both local and remote commands
- **DRY Principle**: Common execution patterns extracted into reusable functions (e.g., `executeRebuild()`, `executePull()`, `executeCommand()`)

### Function Naming Conventions

To avoid naming collisions between commands, use descriptive prefixes:

- `executeStatusCommand()` in status.go
- `executeCommand()` in exec.go (for arbitrary command execution)
- `executeRebuild()` in rebuild.go
- `executePull()` in pull.go

## Homelab Context

The tool manages a homelab consisting of:

- Multiple servers and desktop machines running NixOS
- A Mac running nix-darwin
- All systems have `$HOME/nix-config` directory with a `rebuild.sh` script
- Systems are identified by hostnames and can be accessed via SSH
- Each system has a `$HOSTCLASS` environment variable indicating its role
